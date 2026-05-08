## 1. 探针框架(`collector-probe-framework`)

- [x] 1.1 在 `collector/probe.go` 新增 `ErrorClass` 枚举(`network` / `auth` / `protocol` / `timeout` / `unknown` / 空)
- [x] 1.2 定义 `ProbeResult` 结构(`Target` / `Latency` / `Err` / `ErrClass`),并补 `Logger` 接口与 `nopLogger` fallback
- [x] 1.3 定义 `Probe` 接口(`Name() string` / `Run(ctx) ProbeResult`)
- [x] 1.4 实现 `classify(err error) ErrorClass` 工具函数:`*net.OpError` / DNS 解析失败 → `network`;`context.DeadlineExceeded` / `i/o timeout` → `timeout`;按驱动专属错误把 MySQL 1045、Redis `NOAUTH`/`WRONGPASS`、Mongo auth fail → `auth`;非 2xx HTTP / 解析失败 → `protocol`;其余 → `unknown`
- [x] 1.5 实现 `redactDSN(s string) string`(MySQL DSN 中 `user:pass@` 与 Mongo URI `mongodb://user:pass@`)以及 `wrapErr(err) error` 在错误传播路径上脱敏
- [x] 1.6 实现 `RunProbe(ctx, p Probe, defaultTimeout=5s)`:套 `context.WithTimeout`、计时、调用 `Logger.Probe(...)`、返回 `ProbeResult`
- [x] 1.7 在 `collector/common.go`(或新建 `collector/logger.go`)暴露默认 `Logger` 注入点;`main.go` 暂不接,保留 nopLogger 即可

## 2. 引入驱动依赖

- [x] 2.1 `go get github.com/go-sql-driver/mysql@latest`
- [x] 2.2 `go get github.com/redis/go-redis/v9@latest`
- [x] 2.3 `go get go.mongodb.org/mongo-driver/mongo@latest`(v1)(注:v1 已 deprecated,后续可单独提 change 切 v2)
- [x] 2.4 确认 `go mod tidy` 后 `go.sum` 干净,无 cgo 依赖被牵入(`go build -tags 'osusergo netgo'` 验证)
- [x] 2.5 三平台交叉编译验证:`GOOS=linux GOARCH=amd64`、`GOOS=linux GOARCH=arm64` 均产出 (aarch64 与 arm64 共享产物)

## 3. MySQL collector 重写(`collector/mysql.go`)

- [x] 3.1 删除 `exec.LookPath("mysql")` 与 `mysqlQuery` / `mysqlQueryInt` 文本工具(注:`replication.go` 段一过渡期保留私有 `mysqlQuery` shim,段二一并删除)
- [x] 3.2 新增 `openMySQL(ip, port string, creds) (*sql.DB, error)`:DSN 形如 `user:pass@tcp(ip:port)/?timeout=3s&readTimeout=5s&writeTimeout=5s&interpolateParams=true`,设置 `db.SetMaxOpenConns(1)` / `SetMaxIdleConns(0)`
- [x] 3.3 用 `db.QueryRowContext(ctx, "SELECT @@VERSION")` 等替换原文本查询;批量字段一次 `SELECT` 多个 `@@xxx` 减少握手次数
- [x] 3.4 改 `SHOW SLAVE STATUS` 为 `db.QueryContext` + `rows.Columns()` + `rows.Scan` 到 `[]sql.RawBytes`,按列名取值;同时尝试 `SHOW REPLICA STATUS` 作 8.4 fallback
- [x] 3.5 `SHOW MASTER LOGS` 改为先尝试 `SHOW BINARY LOGS`,失败回退;`BinlogCount = len(rows)`
- [x] 3.6 `collectMySQLNode` 实现 `Probe` 接口,把所有错误经 `classify` 打标,并写入 `MySQLNode.ErrorClass`
- [x] 3.7 `model.MySQLNode` 新增 `ErrorClass string` 字段(omitempty 渲染兼容)
- [ ] 3.8 端到端冒烟:对 `bk.env` 测试集群跑一次(用户后续手工验证)

## 4. Redis collector 重写(`collector/redis.go` + `collector/replication.go` Redis 部分)

- [x] 4.1 删除所有 `exec.Command("redis-cli", ...)` 与 `parseRedisInfo` 中无人引用的部分(保留 parseRedisInfo,go-redis 的 Info 返回同一段文本)
- [x] 4.2 用 `redis.NewClient(&redis.Options{Addr, Password, DialTimeout=3s, ReadTimeout=5s, WriteTimeout=5s, MaxRetries=-1})` 替换 `collectRedisNode`
- [x] 4.3 用 `client.LLen(ctx, "celery").Val()` / `LLen(ctx, "monitor").Val()` 替换 celery / monitor 长度查询(并 db=11)
- [x] 4.4 Sentinel:用 `redis.NewSentinelClient(&redis.Options{Addr})`、调 `sc.Ping(ctx)` 与 `sc.GetMasterAddrByName(ctx, masterName)`
- [x] 4.5 master 可达性 `redisPing` 改为新建临时 `redis.Client` + `Ping(ctx)`;凭据从 `Config.Creds.RedisPassword` 取
- [x] 4.6 `collector/replication.go` 中 Redis 主从 `INFO replication` 走同一 `redis.Client.Info(ctx, "replication")`,字段解析复用 `parseRedisInfo`
- [x] 4.7 全部 redis 调用接入 `Probe` + `classify`(`NOAUTH` / `WRONGPASS` → auth)
- [x] 4.8 `model.RedisNode` / `model.SentinelNodeStatus` / 复制状态结构追加 `ErrorClass` 字段
- [ ] 4.9 端到端冒烟:覆盖单点 / Sentinel / master + slave 三种拓扑(用户后续手工验证)

## 5. MongoDB collector 重写(`collector/mongo.go`)

- [x] 5.1 删除 `exec.LookPath("mongosh")` / 选 `mongo` 二进制的逻辑与 `--eval` 文本拼接
- [x] 5.2 用 `mongo.Connect(ctx, options.Client().ApplyURI(uri).SetMaxPoolSize(2).SetServerSelectionTimeout(3s).SetConnectTimeout(3s).SetSocketTimeout(5s))` 建立连接
- [x] 5.3 用 `client.Database("admin").RunCommand(ctx, bson.D{{"replSetGetStatus", 1}})` + `Decode(&result)` 取副本集成员;字段映射到 `model.MongoCluster`
- [x] 5.4 在 `Run` 结束(无论成败)`defer client.Disconnect(ctx)`,避免后台监控连接残留
- [x] 5.5 错误经 `classify` 打标;鉴权失败专门识别(`mongo.CommandError` Code 18 / 13)
- [x] 5.6 `model.MongoCluster` 新增 `ErrorClass`
- [x] 5.7 URI 中的密码在错误日志/返回值中走 `redactDSN` 脱敏

## 6. RabbitMQ / ES 接入 Probe 框架(curl 保留)

- [x] 6.1 把 `rmqAPI` / es 请求函数包成 `Probe.Run`,统一 ctx 超时:`exec.CommandContext(ctx, "curl", "-s", "--max-time", "5", ...)`
- [x] 6.2 错误分类:`curl` exit 6/7 → `network`、22 → `protocol`、28 → `timeout`、67 → `auth`、其他非零 → `unknown`
- [x] 6.3 `model.RabbitMQStatus` / `model.ESCluster` 报告结构新增 `ErrorClass`
- [ ] 6.4 验证 RabbitMQ Mgmt API 不可达时的报告输出与改造前一致(用户后续手工验证)

## 7. 主流程接入

- [x] 7.1 `main.go` / 调用方传入 `context.Background()` 给所有 collector
- [x] 7.2 删除 collector 中所有 `exec.LookPath` / `os/exec` 的 import(rabbitmq.go、es.go 仍保留 `os/exec`)
- [x] 7.3 `go vet ./...` + `go build ./...` + 三平台交叉编译全部通过

## 8. 单元测试(回归网)

- [x] 8.1 为 `classify` 增加表驱动测试,覆盖 `*net.OpError` / DNSError / 文本兜底 / `mysql.MySQLError{1045/1044/1698}` 等关键 case
- [x] 8.2 为 `redactDSN` 写测试,覆盖 MySQL DSN 与 Mongo URI
- [x] 8.3 `pickFirst` 列重命名兼容测试 + ctx 截止穿透测试(替代沉重的 sqlmock,验证同一不变量)
- [ ] 8.4 为 Redis collector 用 `miniredis` 写 `INFO` / `LLEN` 测试(留下次单独提)
- [ ] 8.5 为 RabbitMQ probe 写 fake curl 退出码映射测试(留下次单独提)

## 9. 报告对照与切换

- [ ] 9.1 在测试集群上分别用旧版本 binary 与新版本 binary 跑一次完整巡检,逐字段 diff 报告(用户后续手工验证)
- [x] 9.2 删除任何遗留的"二进制不可用"分支与 `not available` 文案(curl 入口仍保留,作为 RabbitMQ/ES 唯一二进制依赖)
- [ ] 9.3 PR 描述中说明 `go.mod` 新增依赖与二进制体积变化(交付时由用户在 PR 描述补充)

## 10. 文档与收尾

- [x] 10.1 更新 `openspec/project.md`:外部依赖列表与采集能力表已改为"无需 CLI,使用原生驱动"
- [x] 10.2 README 不含 mysql/redis/mongo client 前置条件描述,无需修改
- [x] 10.3 运行 `openspec validate native-collector-clients --strict` 确认结构合规
