## Context

`weops-inspect` 当前在 `collector/` 下统一以"调用本机 CLI 二进制 + 解析文本"的方式探测 MySQL / Redis / MongoDB / RabbitMQ / ES,详见 `proposal.md` 的动机。本次设计聚焦两件事:

1. 用纯 Go 驱动替代 `mysql` / `redis-cli` / `mongosh` 三个 CLI;
2. 抽出统一的探针框架(`Probe` 接口 + `ProbeResult`),为本次替换以及后续新增组件采集提供共享基线。

RabbitMQ Mgmt API / ES 仍走 `curl + JSON` 路径(用户明确要求保留 curl),但要纳入新的 `Probe` 框架以共享超时与错误分类。

约束:

- 现有报告结构 (`model.MySQLNode` / `model.RedisNode` / `model.MongoStatus` / `model.RabbitMQStatus` / `model.HostMetrics` 等) 已被 `render/` 消费,本次保持字段向后兼容,只新增不删改。
- 跨平台编译目标保持 `linux/amd64`、`linux/arm64`、`linux/aarch64`,**禁止引入 cgo**。
- 凭据来源不变,仍使用 `config.Credentials`。

## Goals / Non-Goals

**Goals:**

- 完全移除 collector 对 `mysql` / `redis-cli` / `mongosh` 三个外部二进制的依赖。
- 引入 `Probe` 接口与 `ProbeResult` 结构,让所有 collector 共享:每次探测受 `context` 超时约束、错误按类(network / auth / protocol / timeout / unknown)分类、统一记录耗时与目标地址、可挂结构化日志钩子。
- 把现在的 CLI 文本解析(尤其是 MySQL `SHOW SLAVE STATUS\G`、Redis `INFO`)替换为按列名/字段名的结构化扫描,顺便兼容 MySQL 8 的 `Replica_*` 字段重命名。
- 把凭据从 `argv` 中拿掉,只在驱动内部握手时使用。

**Non-Goals:**

- 不替换 RabbitMQ / ES 当前使用的 `curl`(用户要求保留);只把它们包进 `Probe` 框架。
- 不变更巡检报告字段语义、不变更配置文件结构、不引入新的 RPC / 服务架构。
- 不引入 cgo 依赖、不改变交叉编译产物的命名。
- 不重写 `host.go` / `service.go` 这两个走 SSH 的 collector(本次只针对组件探测层)。

## Decisions

### D1: 选用 `database/sql` + `github.com/go-sql-driver/mysql`

**为什么:** 纯 Go、社区事实标准、对 MySQL 5.7 / 8.0 / 8.4 兼容良好,支持 `caching_sha2_password`(MySQL 8 默认鉴权插件)。

**替代方案:**
- `github.com/ziutek/mymysql` — 维护停滞。
- 通过 `mysqlx` / x-protocol — 与现有 5.7 集群不兼容。

**注意点:**
- DSN 里强制 `timeout=3s&readTimeout=5s&writeTimeout=5s&parseTime=false&interpolateParams=true`。
- `SHOW SLAVE STATUS\G` 改为 `SHOW SLAVE STATUS` + `rows.Columns()` + `rows.Scan` 到 `[]sql.RawBytes`,按列名取值;同时 fallback `SHOW REPLICA STATUS`(MySQL 8.0.22+)。
- `SHOW MASTER LOGS` 改为优先 `SHOW BINARY LOGS`,失败再回退老命令。

### D2: 选用 `github.com/redis/go-redis/v9`

**为什么:** 纯 Go、官方推荐、对 sentinel 提供 `SentinelClient` 低层 API。

**Sentinel 处理方式:** 使用 `redis.NewSentinelClient`(逐个 sentinel 调用 `Ping` 与 `GetMasterAddrByName`),**不使用** `NewFailoverClient`。理由:巡检语义是"显式列出每个 sentinel 的可达性 + 主发现 + 主可达性",`FailoverClient` 会把这些细节封装掉,抹去报告所需的可见性。

**替代方案:**
- `github.com/gomodule/redigo` — 维护节奏慢,sentinel 支持需要自己拼。
- `github.com/joomcode/redispipe` — 过于偏 pipeline 性能场景。

### D3: 选用 `go.mongodb.org/mongo-driver`(官方 v1)

**为什么:** 官方驱动,稳定;v2 API 仍在演进,暂不切。
**关键参数:** `MaxPoolSize=2`、`ServerSelectionTimeout=3s`、`ConnectTimeout=3s`、`SocketTimeout=5s`,以避免一次性巡检触发不必要的连接池。
**复制集状态:** 仍用 `RunCommand({replSetGetStatus: 1})` 取 `members[*].name / stateStr`,与现有 `model.MongoStatus` 字段对齐。

### D4: 引入 `collector/probe.go` 框架

```go
// pseudo, 实际签名以 tasks.md 为准
type ErrorClass string
const (
    ErrNone     ErrorClass = ""
    ErrNetwork  ErrorClass = "network"
    ErrAuth     ErrorClass = "auth"
    ErrProtocol ErrorClass = "protocol"
    ErrTimeout  ErrorClass = "timeout"
    ErrUnknown  ErrorClass = "unknown"
)

type ProbeResult struct {
    Target   string        // ip:port
    Latency  time.Duration
    Err      error
    ErrClass ErrorClass
}

type Probe interface {
    Name() string                              // mysql / redis / mongo / rabbitmq / es
    Run(ctx context.Context) ProbeResult
}
```

- 默认每个 probe 通过 `context.WithTimeout(parent, 5s)` 包裹(可在 `Config` 里覆盖,但本次不暴露给用户)。
- `ErrClass` 由各 collector 在 catch 错误时显式打标签(例如 driver 返回 `*net.OpError` → `network`,`mysql.MySQLError{Number: 1045}` → `auth`,`context.DeadlineExceeded` → `timeout`)。
- 框架提供一个 `RunAll(ctx, probes []Probe) []ProbeResult` 帮助函数,按现有的 collector 串行/并行习惯调用(本次保持串行,避免改动行为)。
- 结构化日志:框架接受一个 `Logger` 接口(本次实现可以是简单 `log.Printf` 包装,核心是预留扩展点),每次 `Run` 结束打 `target / latency_ms / err_class / err`。

### D5: 报告字段与渲染兼容策略

- `model.*Node` 现有的 `Error string` 字段保留,语义不变(自由文本)。
- 在每个组件的 model 里新增 `ErrorClass string`(可选),由 collector 写入。
- `render/` 不强依赖新字段:存在时优先据此判级,缺失时按旧的"是否有 Error 字符串"判级;让本次改动可独立 ship,不必同步改全部 renderer。

### D6: curl 保留范围

- `rabbitmq.go` / `es.go` 继续用 `exec("curl", ...)`,但把执行包进 `Probe.Run` 里:超时由 ctx 控制(传 `--max-time`),错误分类按 `curl exit code` 映射(7 → network、22 → protocol、28 → timeout)。
- 不在本次范围切到 `net/http`(虽然技术上更合适),保持与用户指令一致;留作单独 change。

## Risks / Trade-offs

- **二进制体积 +5–8MB** → 接受。运维工具体积非关键。
- **驱动行为差异**:`go-sql-driver/mysql` 在 server 关闭连接时返回的 error 文案不同,影响日志可读性 → 通过 `ErrClass` 抽象掉,renderer 不再依赖文案。
- **MySQL 5.6 之前的兼容性**:`go-sql-driver/mysql` 已不再优先支持 5.5 以下;考虑到当前 `bk.env` 内最低也是 5.7,不做下兼容。**Risk** → 若未来出现 5.5 集群,需要回退或加白名单提示。
- **MongoDB 驱动会主动建立后台监控连接**:对短生命周期 CLI 工具不友好,需要在 `Run` 结束时 `Disconnect(ctx)` 显式释放,避免巡检退出时端口残留。**Mitigation** → tasks.md 中显式列入 disconnect 步骤。
- **凭据从 `argv` 移到驱动握手**:进程列表里干净了,但驱动可能在错误日志里打印 DSN(包含密码)→ 在 collector 错误处理里做 redact,严禁直接 `fmt.Errorf("dial: %v", err)` 时把 DSN 带进去。
- **Sentinel 自定义路径与 go-redis 的 `FailoverClient` 不同**:维护成本略高 → 接受,理由见 D2。
- **测试现状空白**:collector 目录没有任何 `_test.go`,本次重写若不补回归网,出问题不易察觉 → tasks.md 列入"为每个 collector 增加 mock-driver 单测"。

## Migration Plan

1. 新增 `collector/probe.go` 与错误分类工具,先不接入 collector,确保编译。
2. 切换 `mysql.go`,保留旧 `mysql_cli.go` 副本作为参考(merge 前删除)。在测试集群跑一次报告对比字段。
3. 切换 `redis.go` 与 `replication.go` 中 redis 部分。
4. 切换 `mongo.go`。
5. 把 `rabbitmq.go` / `es.go` 接入 `Probe` 框架(curl 保留)。
6. 删除 `exec.LookPath` 三个二进制的检查、删除文本解析辅助函数(`parseRedisInfo` 等若 go-redis 已替代则删,若还需则保留)。
7. 跑一次完整 `weops-inspect`,与重写前的报告对照(基于线下 `bk.env` 拓扑)。

**Rollback:** 直接 revert 整个 change(改动集中在 `collector/`,无 schema / 配置变更,可单 commit 回滚)。

## Open Questions

- 是否要把"探测耗时(latency_ms)"也直接写入巡检报告?当前不写,只走日志;若要写需要在 model 层多加字段 → 留待 review 决定。
- `Probe` 的并行度:本次保持串行以最小化行为变更,但若未来用户感受到巡检整体耗时偏长,是否提供 `INSPECT_PARALLEL=true` 开关?**待定。**
- MySQL 驱动是否启用 `tls=preferred`?当前内网环境不需要,但若蓝鲸有强 TLS 需求可补充。**待定。**
