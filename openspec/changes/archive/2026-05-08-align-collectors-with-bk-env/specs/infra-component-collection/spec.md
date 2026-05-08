## ADDED Requirements

### Requirement: MySQL 多节点采集

系统 SHALL 对 `Config.MySQLIPs` 中每个节点执行单独连接探活,使用 `Config.Creds.MySQLUser/MySQLPassword`,端口取 `Config.MySQLPort`。

#### Scenario: 单节点 MySQL

- **WHEN** `Config.MySQLIPs = ["10.11.24.61"]`
- **THEN** 报告中 `MySQL` 字段包含 1 条节点结果,记录连接成功 / 失败、版本、运行天数等

#### Scenario: 多节点 MySQL

- **WHEN** `Config.MySQLIPs = ["10.11.24.61", "10.11.24.62"]`
- **THEN** 报告中 `MySQL` 字段包含 2 条节点结果,逐个连接

### Requirement: 单点 Redis 多节点采集

系统 SHALL 对 `Config.RedisIPs` 每个节点执行单独连接(非 sentinel 模式),使用 `Config.Creds.RedisPassword`,端口取 `Config.RedisPort`。

#### Scenario: 多节点单点 Redis

- **WHEN** `Config.RedisIPs = ["10.11.24.60", "10.11.24.61"]`
- **THEN** 报告中 `RedisStandalone` 字段包含 2 条节点结果,逐个 PING 探活

### Requirement: Redis Sentinel 集群级状态

系统 SHALL 对 `Config.RedisSentinelIPs` 走集群级检查,具体包括:

- 对每个 sentinel 节点执行 `PING`,记录可达性
- 通过任一可达 sentinel 调用 `SENTINEL get-master-addr-by-name <master>`(master 名取自 `BK_APIGW_REDIS_SENTINEL_MASTER_NAME`,默认 `mymaster`),获取 master 地址
- 对发现的 master 地址执行 PING,记录可达性

#### Scenario: 哨兵集群健康

- **WHEN** 所有 sentinel 都可达且能发现 master,且 master 可达
- **THEN** 报告中 `RedisSentinel.Status` 标记为 `ok`

#### Scenario: 部分哨兵不可达

- **WHEN** 至少一个 sentinel 不可达,但仍能从其他 sentinel 发现 master 且 master 可达
- **THEN** 报告中 `RedisSentinel.Status` 标记为 `warn`,并记录每个 sentinel 的可达性

#### Scenario: master 不可达

- **WHEN** 所有 sentinel 可达但发现的 master 不可达,或所有 sentinel 都不可达
- **THEN** 报告中 `RedisSentinel.Status` 标记为 `critical`

### Requirement: MongoDB 副本集采集

系统 SHALL 使用形如 `mongodb://<user>:<pwd>@<ip1>:<port>,<ip2>:<port>,<ip3>:<port>/?replicaSet=<rs>` 的 URI 连接副本集,IP 列表来自 `Config.MongoDBIPs`,`<rs>` 来自 `Config.MongoRSName`,凭据来自 `Config.Creds.MongoDBUser/MongoDBPassword`。连接成功后调用 `replSetGetStatus` 并记录每个成员的 `name` 与 `stateStr`。

#### Scenario: 全部成员健康

- **WHEN** 副本集中 1 个 PRIMARY、2 个 SECONDARY 都健康
- **THEN** 报告中 `MongoDB.Members` 包含 3 条记录,每条含 `Name` 与 `StateStr`

#### Scenario: 副本集连接失败

- **WHEN** URI 中所有节点都不可达
- **THEN** 报告中 `MongoDB.Error` 记录连接失败,不阻塞其他组件采集

### Requirement: ES7 与 RabbitMQ 多节点采集保留

系统 SHALL 维持现有 ES7 与 RabbitMQ 的多节点采集行为,IP 列表分别来自 `Config.ES7IPs` 与 `Config.RabbitMQIPs`,凭据使用 `Config.Creds.ES7Password` 与 `Config.Creds.RabbitMQUser/RabbitMQPassword`。

#### Scenario: 现有 ES7 采集不变

- **WHEN** 仅 `BK_ES7_IP_COMMA` 与 `BK_ES7_ADMIN_PASSWORD` 改变
- **THEN** ES7 采集行为与本 change 之前一致
