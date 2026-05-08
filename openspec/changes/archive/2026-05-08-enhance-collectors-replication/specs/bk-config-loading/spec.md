## ADDED Requirements

### Requirement: MySQL 主从 IP 字段加载

系统 SHALL 新增 `Config.MySQLMasterIPs []string` 与 `Config.MySQLSlaveIPs []string` 字段,分别从 `BK_MYSQL_MASTER_IP_COMMA` 与 `BK_MYSQL_SLAVE_IP_COMMA` 解析为多节点 IP 列表;变量未设置时为空切片。

#### Scenario: 同时存在 master 与 slave

- **WHEN** `BK_MYSQL_MASTER_IP_COMMA=10.97.20.21` 且 `BK_MYSQL_SLAVE_IP_COMMA=10.97.20.22,10.97.20.23`
- **THEN** `Config.MySQLMasterIPs=["10.97.20.21"]` 且 `Config.MySQLSlaveIPs=["10.97.20.22","10.97.20.23"]`

#### Scenario: 仅有全集 IP 无主从划分

- **WHEN** `BK_MYSQL_IP_COMMA=10.97.20.21,10.97.20.22` 但 `BK_MYSQL_MASTER_IP_COMMA` 与 `BK_MYSQL_SLAVE_IP_COMMA` 未设置
- **THEN** `Config.MySQLMasterIPs` 与 `Config.MySQLSlaveIPs` 均为空切片,`Config.MySQLIPs` 仍按姊妹 change 行为加载

### Requirement: Redis 主从 IP 字段加载

系统 SHALL 新增 `Config.RedisMasterIPs []string` 与 `Config.RedisSlaveIPs []string` 字段,分别从 `BK_REDIS_MASTER_IP_COMMA` 与 `BK_REDIS_SLAVE_IP_COMMA` 解析为多节点 IP 列表;变量未设置时为空切片。

#### Scenario: 主从 + 哨兵全配

- **WHEN** `BK_REDIS_MASTER_IP_COMMA=10.97.20.17`、`BK_REDIS_SLAVE_IP_COMMA=10.97.20.18`、`BK_REDIS_SENTINEL_IP_COMMA=10.97.20.20,10.97.20.21,10.97.20.23` 同时设置
- **THEN** `Config.RedisMasterIPs / RedisSlaveIPs / RedisSentinelIPs` 分别保留各自列表,互不影响

#### Scenario: 仅哨兵无主从

- **WHEN** `BK_REDIS_SENTINEL_IP_COMMA` 有值,`BK_REDIS_MASTER_IP_COMMA` 与 `BK_REDIS_SLAVE_IP_COMMA` 未设置
- **THEN** `Config.RedisMasterIPs` 与 `Config.RedisSlaveIPs` 为空切片,Sentinel 采集仍正常运行;角色校验跳过
