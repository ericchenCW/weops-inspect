## Why

新版 `bk.env`(WeOps 部署)显式给出了 `BK_MYSQL_MASTER_IP_COMMA / BK_MYSQL_SLAVE_IP_COMMA` 与 `BK_REDIS_MASTER_IP_COMMA / BK_REDIS_SLAVE_IP_COMMA`,说明 MySQL 与 Redis 都是主从架构而非平权多节点。仅做"每个节点能连接"的探活会漏掉真实环境最高频的故障 — **复制中断**(`Slave_IO/SQL_Running=No`)与**主从延迟**(`Seconds_Behind_Master` / `master_last_io_seconds_ago`)。这类故障不影响连接探活但会导致从库数据滞后或长时间不可用,是 DBA/SRE 真正想看到的指标。

姊妹 change [`align-collectors-with-bk-env`](../align-collectors-with-bk-env/) 已覆盖 Redis Sentinel、MongoDB 副本集、平权多节点,本 change 在其完成后追加复制健康检查能力,且不重复其工作。

## What Changes

- **Config 新增主从字段**:`MySQLMasterIPs / MySQLSlaveIPs / RedisMasterIPs / RedisSlaveIPs`(`[]string`),从对应 `BK_*_MASTER_IP_COMMA / BK_*_SLAVE_IP_COMMA` 加载;旧 env 没有这些键时为空切片,collector 退化到平权探活,不破坏向下兼容
- **MySQL slave 复制状态采集**:对每个 slave 节点执行 `SHOW SLAVE STATUS`,记录 `Slave_IO_Running` / `Slave_SQL_Running` / `Seconds_Behind_Master` / `Last_Error`
- **MySQL master 角色校验**:对每个 master 节点查询 `read_only` 变量,若 `read_only=ON` 在报告中标记异常
- **Redis 主从角色校验**:对 `RedisMasterIPs / RedisSlaveIPs` 各节点执行 `INFO replication`,验证 `role` 字段与 env 配置一致;slave 节点额外采集 `master_link_status` / `master_last_io_seconds_ago` / `master_sync_in_progress`
- **Sentinel 与 env 交叉校验**:Sentinel 集群发现的 master 地址与 `BK_REDIS_MASTER_IP_COMMA` 做对照,不一致时在报告中标记 warn
- **新增复制相关阈值**:`INSPECT_MYSQL_REPL_LAG_THRESHOLD`(默认 60 秒)、`INSPECT_REDIS_REPL_IO_THRESHOLD`(默认 10 秒)
- **报告渲染**:MySQL / Redis 段落新增"主从健康"子节,呈现复制状态、角色校验、延迟数值

## Capabilities

### New Capabilities

- `replication-collection`:MySQL 主从复制状态、Redis 主从角色与同步状态、Sentinel 选举一致性的采集与判定

### Modified Capabilities

- `bk-config-loading`:新增 `MySQLMasterIPs / MySQLSlaveIPs / RedisMasterIPs / RedisSlaveIPs` 字段及对应 env 解析(在姊妹 change 已建立的 capability 基础上扩展)
- `infra-component-collection`:MySQL collector 增加复制状态采集;Redis collector 增加角色校验与同步指标;Sentinel 采集增加与 env master 的对照
- `threshold-config`:新增两条复制相关阈值

## Impact

- **依赖**:本 change 依赖 `align-collectors-with-bk-env` 已 archive(`MySQLIPs / RedisIPs / RedisSentinelIPs` 字段及 collector 拆路必须就位)
- **代码**:`config/config.go`(新增 4 字段 + 解析 + 阈值)、`collector/mysql.go`(SHOW SLAVE STATUS / read_only 校验)、`collector/redis.go`(INFO replication 解析、Sentinel 交叉校验)、`model/types.go`(新增复制状态结构体)、`render/output`(渲染主从健康段)、`checker/rules.go`(复制相关规则)
- **环境变量**:新增 `INSPECT_MYSQL_REPL_LAG_THRESHOLD`、`INSPECT_REDIS_REPL_IO_THRESHOLD`;消费已存在的 `BK_MYSQL_MASTER_IP_COMMA` / `BK_MYSQL_SLAVE_IP_COMMA` / `BK_REDIS_MASTER_IP_COMMA` / `BK_REDIS_SLAVE_IP_COMMA`
- **行为变化**:MySQL/Redis 报告体积增加(每节点多一段复制状态);`Slave_IO/SQL_Running=No` 或主从延迟超阈值会触发新告警
- **不变**:不引入新的数据库 driver;不做按业务模块凭据(paas/bklog/job)的 DB/vhost 级体检;不扩展 `ModuleRegistry`
- **向下兼容**:`*_MASTER/SLAVE_IP_COMMA` env 缺失时 collector 跳过复制采集,不报错
