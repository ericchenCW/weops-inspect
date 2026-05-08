## 1. 探针框架(`collector-probe-framework`)

- [ ] 1.1 在 `collector/probe.go` 新增 `ErrorClass` 枚举(`network` / `auth` / `protocol` / `timeout` / `unknown` / 空)
- [ ] 1.2 定义 `ProbeResult` 结构(`Target` / `Latency` / `Err` / `ErrClass`),并补 `Logger` 接口与 `nopLogger` fallback
- [ ] 1.3 定义 `Probe` 接口(`Name() string` / `Run(ctx) ProbeResult`)
- [ ] 1.4 实现 `classify(err error) ErrorClass` 工具函数:`*net.OpError` / DNS 解析失败 → `network`;`context.DeadlineExceeded` / `i/o timeout` → `timeout`;按驱动专属错误把 MySQL 1045、Redis `NOAUTH`/`WRONGPASS`、Mongo auth fail → `auth`;非 2xx HTTP / 解析失败 → `protocol`;其余 → `unknown`
- [ ] 1.5 实现 `redactDSN(s string) string`(MySQL DSN 中 `user:pass@` 与 Mongo URI `mongodb://user:pass@`)以及 `wrapErr(err) error` 在错误传播路径上脱敏
- [ ] 1.6 实现 `RunProbe(ctx, p Probe, defaultTimeout=5s)`:套 `context.WithTimeout`、计时、调用 `Logger.Probe(...)`、返回 `ProbeResult`
- [ ] 1.7 在 `collector/common.go`(或新建 `collector/logger.go`)暴露默认 `Logger` 注入点;`main.go` 暂不接,保留 nopLogger 即可

## 2. 引入驱动依赖

- [ ] 2.1 `go get github.com/go-sql-driver/mysql@latest`
- [ ] 2.2 `go get github.com/redis/go-redis/v9@latest`
- [ ] 2.3 `go get go.mongodb.org/mongo-driver/mongo@latest`(v1)
- [ ] 2.4 确认 `go mod tidy` 后 `go.sum` 干净,无 cgo 依赖被牵入(`go build -tags 'osusergo netgo'` 验证)
- [ ] 2.5 三平台交叉编译验证:`GOOS=linux GOARCH=amd64`、`GOOS=linux GOARCH=arm64`、`GOOS=linux GOARCH=arm64`(aarch64 别名)产物均能产出

## 3. MySQL collector 重写(`collector/mysql.go`)

- [ ] 3.1 删除 `exec.LookPath("mysql")` 与 `mysqlQuery` / `mysqlQueryInt` 文本工具
- [ ] 3.2 新增 `openMySQL(ip, port string, creds) (*sql.DB, error)`:DSN 形如 `user:pass@tcp(ip:port)/?timeout=3s&readTimeout=5s&writeTimeout=5s&interpolateParams=true`,设置 `db.SetMaxOpenConns(1)` / `SetMaxIdleConns(0)`
- [ ] 3.3 用 `db.QueryRowContext(ctx, "SELECT @@VERSION")` 等替换原文本查询;批量字段一次 `SELECT` 多个 `@@xxx` 减少握手次数
- [ ] 3.4 改 `SHOW SLAVE STATUS` 为 `db.QueryContext` + `rows.Columns()` + `rows.Scan` 到 `[]sql.RawBytes`,按列名取值;同时尝试 `SHOW REPLICA STATUS` 作 8.4 fallback
- [ ] 3.5 `SHOW MASTER LOGS` 改为先尝试 `SHOW BINARY LOGS`,失败回退;`BinlogCount = len(rows)`
- [ ] 3.6 `collectMySQLNode` 实现 `Probe` 接口,把所有错误经 `classify` 打标,并写入 `MySQLNode.ErrorClass`
- [ ] 3.7 `model.MySQLNode` 新增 `ErrorClass string` 字段(omitempty 渲染兼容)
- [ ] 3.8 端到端冒烟:对 `bk.env` 测试集群跑一次,字段值与改造前完全一致(`Version` / `MaxConnections` / `Role` / `BinlogCount` 等)

## 4. Redis collector 重写(`collector/redis.go` + `collector/replication.go` Redis 部分)

- [ ] 4.1 删除所有 `exec.Command("redis-cli", ...)` 与 `parseRedisInfo` 中无人引用的部分(`INFO` 解析仍可保留,因为 go-redis 的 `Info(ctx).Result()` 返回的就是同一段文本)
- [ ] 4.2 用 `redis.NewClient(&redis.Options{Addr, Password, DialTimeout=3s, ReadTimeout=5s, WriteTimeout=5s, MaxRetries=0})` 替换 `collectRedisNode`
- [ ] 4.3 用 `client.LLen(ctx, "celery").Val()` / `LLen(ctx, "monitor").Val()` 替换 celery / monitor 长度查询(并 `Select(11)`)
- [ ] 4.4 Sentinel:用 `redis.NewSentinelClient(&redis.Options{Addr})`、调 `sc.Ping(ctx)` 与 `sc.GetMasterAddrByName(ctx, masterName)`;**不要**用 `NewFailoverClient`
- [ ] 4.5 master 可达性 `redisPing` 改为新建临时 `redis.Client` + `Ping(ctx)`;凭据从 `Config.Creds.RedisPassword` 取
- [ ] 4.6 `collector/replication.go` 中 Redis 主从 `INFO replication` 走同一 `redis.Client.Info(ctx, "replication")`,字段解析复用 `parseRedisInfo`
- [ ] 4.7 全部 redis 调用接入 `Probe` + `classify`(`NOAUTH` / `WRONGPASS` → auth)
- [ ] 4.8 `model.RedisNode` / `model.SentinelNodeStatus` / 复制状态结构追加 `ErrorClass` 字段
- [ ] 4.9 端到端冒烟:覆盖单点 / Sentinel / master + slave 三种拓扑

## 5. MongoDB collector 重写(`collector/mongo.go`)

- [ ] 5.1 删除 `exec.LookPath("mongosh")` / 选 `mongo` 二进制的逻辑与 `--eval` 文本拼接
- [ ] 5.2 用 `mongo.Connect(ctx, options.Client().ApplyURI(uri).SetMaxPoolSize(2).SetServerSelectionTimeout(3s).SetConnectTimeout(3s).SetSocketTimeout(5s))` 建立连接
- [ ] 5.3 用 `client.Database("admin").RunCommand(ctx, bson.D{{"replSetGetStatus", 1}})` + `Decode(&result)` 取副本集成员;字段映射到 `model.MongoStatus`
- [ ] 5.4 在 `Run` 结束(无论成败)`defer client.Disconnect(ctx)`,避免后台监控连接残留
- [ ] 5.5 错误经 `classify` 打标;鉴权失败专门识别(`mongo.CommandError` Code 18 / 13)
- [ ] 5.6 `model.MongoStatus` 新增 `ErrorClass`
- [ ] 5.7 URI 中的密码在错误日志/返回值中走 `redactDSN` 脱敏

## 6. RabbitMQ / ES 接入 Probe 框架(curl 保留)

- [ ] 6.1 把 `rmqAPI` / es 请求函数包成 `Probe.Run`,统一 ctx 超时:`exec.CommandContext(ctx, "curl", "-s", "--max-time", "5", ...)`
- [ ] 6.2 错误分类:`curl` exit 7 → `network`、22 → `protocol`、28 → `timeout`、其他非零 → `unknown`
- [ ] 6.3 `model.RabbitMQStatus` / ES 报告结构新增 `ErrorClass`
- [ ] 6.4 验证 RabbitMQ Mgmt API 不可达时的报告输出与改造前一致(同样的 Error 文本 + 新的 ErrorClass)

## 7. 主流程接入

- [ ] 7.1 `main.go` / 调用方传入 `context.Background()` 给所有 collector(本次保持串行,不引入并发)
- [ ] 7.2 删除 collector 中所有 `exec.LookPath` / `os/exec` 的 import(rabbitmq.go、es.go 仍保留 `os/exec`)
- [ ] 7.3 `go vet ./...` + `go build ./...` + 三平台交叉编译全部通过

## 8. 单元测试(回归网)

- [ ] 8.1 为 `classify` 增加表驱动测试,覆盖 `*net.OpError` / `context.DeadlineExceeded` / `mysql.MySQLError{1045}` / `redis.Error("NOAUTH")` 等关键 case
- [ ] 8.2 为 `redactDSN` 写测试,覆盖 MySQL DSN 与 Mongo URI
- [ ] 8.3 为 MySQL collector 用 `DATA-DOG/go-sqlmock`(或同等的 `database/sql` mock)写一个 `SHOW SLAVE STATUS` 列扫描的回归测试
- [ ] 8.4 为 Redis collector 用 `miniredis`(纯 Go in-memory Redis)写 `INFO` / `LLEN` / `Sentinel get-master-addr-by-name` 测试
- [ ] 8.5 为 RabbitMQ probe 写一个用 fake `curl` 二进制脚本(放进 `t.TempDir`)的退出码 → ErrorClass 映射测试

## 9. 报告对照与切换

- [ ] 9.1 在测试集群上分别用旧版本 binary 与新版本 binary 跑一次完整巡检,逐字段 diff 报告(只允许新增 `ErrorClass`,其他字段值不应变化)
- [ ] 9.2 删除任何遗留的"二进制不可用"分支与 `not available` 文案
- [ ] 9.3 PR 描述中说明 `go.mod` 新增依赖与二进制体积变化

## 10. 文档与收尾

- [ ] 10.1 更新 `openspec/project.md` 中提及"依赖巡检机本地 mysql / redis-cli / mongosh"的部分(若有)
- [ ] 10.2 在 README / 部署说明里删除"巡检机需安装 mysql client 等"的前置条件
- [ ] 10.3 运行 `openspec validate native-collector-clients --strict` 确认结构合规
