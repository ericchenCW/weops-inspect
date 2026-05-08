## Why

当前 `collector/` 下对 MySQL / Redis / MongoDB 的采集全部通过 `exec.Command(...)` 调用目标二进制 (`mysql` / `redis-cli` / `mongosh`),带来三类问题:

1. **巡检机环境耦合**:任一二进制缺失即直接给出 `critical / not available`,与目标组件的真实健康状态无关;且 MySQL 8 / 8.4 等版本对客户端协议、命令名 (`SHOW MASTER LOGS` → `SHOW BINARY LOGS`) 有差异,客户端版本飘移会污染巡检结果。
2. **凭据通过命令行参数传递**:`mysql -p<密码>`、`redis-cli -a <密码>` 在目标巡检机的 `ps` / shell history 中可见,存在泄漏面。
3. **缺乏统一的探针语义**:每个 collector 自己写超时/错误处理/重试,实际上几乎都没有超时,目标僵死时巡检主流程会一起挂;错误信息也无分类(network / auth / protocol / timeout 难以区分)。

借这次去 CLI 化的机会,顺势把 collector 抽象成统一的 `Probe` 接口,让超时、错误分类、结构化日志、耗时统计在一处实现,后续新增组件 (PostgreSQL / Kafka / etcd) 无需重复造轮子。

## What Changes

- 引入新的内部能力 `collector-probe-framework`:定义 `Probe` 接口 + `ProbeResult` 结构(含 `ErrorClass`、耗时、目标地址)+ 默认 ctx 超时策略 + 结构化日志钩子。
- 用纯 Go 驱动重写以下 collector,**不再依赖任何外部二进制**(curl 暂时保留用于 RabbitMQ Mgmt API / ES,见 design.md):
  - `collector/mysql.go` → `database/sql` + `github.com/go-sql-driver/mysql`
  - `collector/redis.go` → `github.com/redis/go-redis/v9`(标准客户端 + `SentinelClient` 低层调用,不使用 `FailoverClient`)
  - `collector/mongo.go` → `go.mongodb.org/mongo-driver`
  - `collector/replication.go` 中 redis 主从相关查询同步切换
- 删除对应的 `exec.LookPath` 探针检查与 CLI 输出解析逻辑;`SHOW SLAVE STATUS\G` 字符串切分改为按列名 `Query` 扫描,顺带兼容 MySQL 8 的 `Replica_*` 字段重命名。
- **BREAKING(运行时层面)**:不再要求巡检机预装 `mysql` / `redis-cli` / `mongosh`;`go.mod` 新增三个驱动依赖,二进制体积约 +5–8MB。
- collector 错误信息升级:`Error` 字段除了原有自由文本外,新增 `ErrorClass` 枚举(`network` / `auth` / `protocol` / `timeout` / `unknown`),用于报告渲染时统一判定告警级别。

## Capabilities

### New Capabilities

- `collector-probe-framework`: 定义所有 collector 共用的探针接口、错误分类与超时/日志规约。

### Modified Capabilities

- `infra-component-collection`: 新增"无外部 CLI 依赖"、"每次探测必须受 ctx 超时约束"、"错误结果必须携带 ErrorClass"三类要求;MySQL 字段抓取改为按列名扫描以兼容 5.7/8.0/8.4。
- `replication-collection`: Redis 主从一致性检查改为通过 go-redis 直连,不再 `exec("redis-cli")`。

## Impact

- **代码**:`collector/mysql.go`、`collector/redis.go`、`collector/mongo.go`、`collector/replication.go`、`collector/common.go` 全部重写;新增 `collector/probe.go`(框架代码)。
- **依赖**:`go.mod` 新增 `github.com/go-sql-driver/mysql`、`github.com/redis/go-redis/v9`、`go.mongodb.org/mongo-driver`。
- **运维**:巡检机不再需要安装 `mysql` / `redis-cli` / `mongosh`;ARM64 / AMD64 跨平台编译保持纯 Go,无 cgo 引入。
- **报告渲染**:`render/` 下消费 `Error` 的地方需要兼容新增的 `ErrorClass` 字段(向后兼容,旧字段保留)。
- **配置/凭据**:无变化,沿用现有 `Config.Creds.*`。
- **不在范围内**:RabbitMQ / ES 仍走 `curl + JSON` 路径,本次不动(用户明确要求保留 curl);如未来要继续去 CLI 化可独立提一个 change。
