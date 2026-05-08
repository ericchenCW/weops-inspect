## 1. Config 重构

- [x] 1.1 在 `config/config.go` 中将 `MySQLIP` / `RedisIP` / `MongoDBIP` 字段重命名为 `MySQLIPs` / `RedisIPs` / `MongoDBIPs`,类型改为 `[]string`
- [x] 1.2 新增字段 `RedisSentinelIPs []string`、`MongoRSName string`
- [x] 1.3 将 `Load()` 中对 `BK_MYSQL_IP` / `BK_REDIS_IP` / `BK_MONGODB_IP` 的读取替换为 `BK_MYSQL_IP_COMMA` / `BK_REDIS_IP_COMMA` / `BK_MONGODB_IP_COMMA` 并通过 `parseIPList` 解析
- [x] 1.4 在 `Load()` 中读取 `BK_REDIS_SENTINEL_IP_COMMA` 到 `RedisSentinelIPs`
- [x] 1.5 在 `Load()` 中读取 `INSPECT_MONGO_RS_NAME`(默认 `rs0`)到 `MongoRSName`
- [x] 1.6 端口默认值改为通过 `INSPECT_MYSQL_PORT`(默认 `3306`)、`INSPECT_REDIS_PORT`(默认 `6379`)读取
- [x] 1.7 `buildAllHosts()` 合并 `ES7IPs`、`RabbitMQIPs`、`MySQLIPs`、`MongoDBIPs`、`RedisIPs`、`RedisSentinelIPs`,保持去重
- [x] 1.8 同步更新 `bkModules` 表(如有用到)与字段消费方

## 2. 阈值 env 覆盖

- [x] 2.1 新增 `parseFloatEnv(key string, def float64) (float64, error)` 与 `parseIntEnv(key string, def int) (int, error)` 工具函数
- [x] 2.2 在 `Load()` 中分别从 `INSPECT_CPU_THRESHOLD` / `INSPECT_DISK_THRESHOLD` / `INSPECT_INODE_THRESHOLD` / `INSPECT_MEM_THRESHOLD` / `INSPECT_MAX_OPEN_FILES` / `INSPECT_RUN_DAYS` 读取阈值
- [x] 2.3 任一阈值解析失败时让 `Load()` 返回错误,错误信息包含变量名
- [x] 2.4 移除原硬编码的 `Thresholds` 直接赋值,改走解析路径

## 3. SSH 多策略

- [x] 3.1 在 `Config` 中新增 `SSHPort int`、`SSHKeyPath string`、`SSHUseSudo bool` 字段
- [x] 3.2 `Load()` 读取 `INSPECT_SSH_PORT`(默认 22)、`INSPECT_SSH_KEY_PATH`(可空)、`INSPECT_SSH_USE_SUDO`(布尔,默认 false)
- [x] 3.3 修改 `ssh.New(...)` 签名以接收 port / keyPath / useSudo,并在内部使用
- [x] 3.4 SSH 客户端拨号时使用 `SSHPort`,认证时若 `SSHKeyPath` 非空则加载该文件作为私钥
- [x] 3.5 `Run(host, cmd)` 中,当 `SSHUseSudo == true` 时将命令改写为 `sudo <cmd>`(只在前部加,不破坏现有 here-doc)
- [x] 3.6 `main.go` 中调用 `ssh.New` 时传入新参数

## 4. MySQL collector

- [x] 4.1 修改 `collector/mysql.go`,以 `Config.MySQLIPs` 为输入,逐个节点连接
- [x] 4.2 节点结果聚合为 `[]MySQLNodeStatus`(包含 IP、连接结果、版本、运行天数等)
- [x] 4.3 更新 `model.InspectReport.MySQL` 类型为切片
- [x] 4.4 `main.go` 中赋值入口同步更新

## 5. Redis collector(单点 + Sentinel)

- [x] 5.1 在 `model/types.go` 中新增 `RedisStandalone []RedisNodeStatus` 与 `RedisSentinel SentinelClusterStatus` 类型
- [x] 5.2 `collector/redis.go` 拆出 `CollectRedisStandalone(cfg)` 与 `CollectRedisSentinel(cfg)` 两个函数
- [x] 5.3 单点采集:对 `Config.RedisIPs` 每个节点 PING 探活,记录可达性 / 版本 / 信息
- [x] 5.4 Sentinel 采集:
  - [x] 5.4.1 对每个 sentinel 节点 PING,记录可达性
  - [x] 5.4.2 通过任一可达 sentinel 调用 `SENTINEL get-master-addr-by-name <master>`(master 名读取 `BK_APIGW_REDIS_SENTINEL_MASTER_NAME`,默认 `mymaster`)
  - [x] 5.4.3 对发现的 master 地址 PING,记录可达性
  - [x] 5.4.4 根据 sentinel 可达数 / master 可达性计算 `Status` 为 `ok` / `warn` / `critical`
- [x] 5.5 `main.go` 中替换原 `CollectRedis` 调用为两个新函数,并赋值到对应字段

## 6. MongoDB collector(副本集)

- [x] 6.1 检查 `go.mod` 中 mongo driver 版本,必要时升级到支持 `replicaSet=` 的版本(实现采用 `mongosh` / `mongo` CLI,URI 原生支持 `replicaSet=`,无需 Go driver)
- [x] 6.2 修改 `collector/mongo.go`,以 `Config.MongoDBIPs` 与 `Config.MongoRSName` 拼接副本集 URI
- [x] 6.3 连接成功后调用 `replSetGetStatus`(通过 `rs.status()`),记录每个成员的 `Name` 与 `StateStr`
- [x] 6.4 在 `model.InspectReport.MongoDB` 中新增 `Members []MongoMemberStatus` 字段(已存在)
- [x] 6.5 副本集连接失败时记录 `Error` 字段,不阻塞其他采集

## 7. Render / 输出

- [x] 7.1 更新 `render` / `output` 中 MySQL / Redis / MongoDB 部分的渲染逻辑以适配新切片 / 新字段
- [x] 7.2 Redis 报告分两段呈现:Standalone 节点列表与 Sentinel 集群状态
- [x] 7.3 MongoDB 报告增加成员状态列表(模板已有该结构,无需扩展)

## 8. 验证

- [ ] 8.1 用 `reference/bk.env` source 后跑一次完整巡检,核对 `AllHosts` 中包含 ES7 / RabbitMQ / MySQL / Mongo 节点 — **需真实环境**
- [ ] 8.2 核对 MySQL / Redis / Mongo 报告输出多节点结果 — **需真实环境**
- [ ] 8.3 核对 Redis 报告分 Standalone 与 Sentinel 两段 — **需真实环境**
- [ ] 8.4 核对 Mongo 报告含每个副本集成员状态 — **需真实环境**
- [ ] 8.5 验证阈值 env 覆盖生效;非法数字时进程退出 — **需真实环境**
- [ ] 8.6 验证 `INSPECT_SSH_PORT` / `INSPECT_SSH_KEY_PATH` / `INSPECT_SSH_USE_SUDO` 各自生效 — **需真实环境**
- [x] 8.7 `go build ./...` 与 `go vet ./...` 全部通过
