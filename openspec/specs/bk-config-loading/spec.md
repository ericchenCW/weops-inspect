# bk-config-loading Specification

## Purpose

定义从蓝鲸 `bk.env` 与 `INSPECT_*` 环境变量加载巡检工具配置的规则,包括基础设施 IP、端口、副本集名等。

## Requirements

### Requirement: 数据库类组件 IP 从 *_IP_COMMA 读取

系统 SHALL 从环境变量 `BK_MYSQL_IP_COMMA`、`BK_REDIS_IP_COMMA`、`BK_MONGODB_IP_COMMA` 解析逗号分隔的多节点 IP 列表,且 NOT 读取裸的 `BK_MYSQL_IP` / `BK_REDIS_IP` / `BK_MONGODB_IP`(后者在 `bk.env` 中是 bash 数组字面量,不可作为环境变量解析)。

#### Scenario: 从 IP_COMMA 解析多节点

- **WHEN** `BK_MONGODB_IP_COMMA=10.11.24.60,10.11.24.62,10.11.24.64`
- **THEN** `Config.MongoDBIPs` 等于 `["10.11.24.60", "10.11.24.62", "10.11.24.64"]`

#### Scenario: 单节点列表

- **WHEN** `BK_MYSQL_IP_COMMA=10.11.24.61`
- **THEN** `Config.MySQLIPs` 等于 `["10.11.24.61"]`

#### Scenario: 空字符串

- **WHEN** `BK_REDIS_IP_COMMA` 未设置
- **THEN** `Config.RedisIPs` 为空切片(长度 0),不报错

### Requirement: Redis Sentinel IP 独立加载

系统 SHALL 从 `BK_REDIS_SENTINEL_IP_COMMA` 解析独立的哨兵节点列表到 `Config.RedisSentinelIPs`,与 `Config.RedisIPs`(单点 Redis)互不干扰。

#### Scenario: 同时存在两套 Redis 拓扑

- **WHEN** `BK_REDIS_IP_COMMA=10.11.24.60,10.11.24.61` 且 `BK_REDIS_SENTINEL_IP_COMMA=10.11.24.62,10.11.24.63,10.11.24.64`
- **THEN** `Config.RedisIPs` 与 `Config.RedisSentinelIPs` 分别保存各自的 IP 列表

### Requirement: 端口默认值与 env 覆盖

系统 SHALL 在 `BK_MYSQL_PORT` / `BK_REDIS_PORT` 这类全局端口 env 不存在的前提下,使用以下默认值,并允许通过 `INSPECT_*_PORT` 覆盖:

- MySQL 默认 `3306`,可由 `INSPECT_MYSQL_PORT` 覆盖
- Redis 默认 `6379`,可由 `INSPECT_REDIS_PORT` 覆盖

#### Scenario: 使用默认端口

- **WHEN** `INSPECT_MYSQL_PORT` 未设置
- **THEN** `Config.MySQLPort` 等于 `"3306"`

#### Scenario: env 覆盖端口

- **WHEN** `INSPECT_REDIS_PORT=6380`
- **THEN** `Config.RedisPort` 等于 `"6380"`

### Requirement: MongoDB 副本集名称配置

系统 SHALL 通过 `INSPECT_MONGO_RS_NAME` 配置副本集名称,默认 `rs0`,以避免依赖模块特定的 `BK_GSE_MONGODB_RSNAME`。

#### Scenario: 默认副本集名

- **WHEN** `INSPECT_MONGO_RS_NAME` 未设置
- **THEN** `Config.MongoRSName` 等于 `"rs0"`

#### Scenario: 自定义副本集名

- **WHEN** `INSPECT_MONGO_RS_NAME=rsCmdb`
- **THEN** `Config.MongoRSName` 等于 `"rsCmdb"`

### Requirement: 配置校验保留

系统 SHALL 在 `Config.Validate()` 中继续要求至少存在一台主机,即 `len(AllHosts) > 0`,否则返回错误。

#### Scenario: 无任何主机配置

- **WHEN** 所有 `BK_*_IP_COMMA` 都未设置
- **THEN** `Config.Validate()` 返回 "no hosts found" 错误

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

### Requirement: bkmonitorv3 模块 IP 加载

系统 SHALL 从 `BK_MONITORV3_IP_COMMA` 解析为 `Config.MonitorV3IPs []string`, 未设置时为空切片; 该字段同时纳入 `Config.AllHosts` 去重集合, 用于 SSH 主机探活与进程巡检。

#### Scenario: 多节点 bkmonitorv3

- **WHEN** `BK_MONITORV3_IP_COMMA=10.97.20.18,10.97.20.19`
- **THEN** `Config.MonitorV3IPs = ["10.97.20.18","10.97.20.19"]` 且这两台 IP 出现在 `Config.AllHosts`

#### Scenario: 未部署 bkmonitorv3

- **WHEN** `BK_MONITORV3_IP_COMMA` 未设置
- **THEN** `Config.MonitorV3IPs` 为空切片, 不影响其它模块加载

### Requirement: bkmonitorv3 依赖凭据加载

系统 SHALL 在 `Config` 中新增以下字段并从对应环境变量读取, 用于 bkmonitorv3 依赖连通性采集; 任一变量未设置时对应字段为空字符串, 由采集器在运行时按 "missing config → skip" 处理:

- `MonitorRedisHost / MonitorRedisPort / MonitorRedisPassword` ← `BK_MONITOR_REDIS_HOST / _PORT / _PASSWORD`
- `MonitorMySQLHost / MonitorMySQLPort / MonitorMySQLUser / MonitorMySQLPassword` ← `BK_MONITOR_MYSQL_*`
- `PaasMySQLHost / PaasMySQLPort / PaasMySQLUser / PaasMySQLPassword` ← `BK_PAAS_MYSQL_*`
- `MonitorRabbitMQHost / MonitorRabbitMQPort / MonitorRabbitMQUser / MonitorRabbitMQPassword / MonitorRabbitMQVHost` ← `BK_MONITOR_RABBITMQ_*`
- `GseZKHost / GseZKPort` ← `BK_GSE_ZK_HOST / _PORT`
- `MonitorES7Host / MonitorES7RestPort / MonitorES7User / MonitorES7Password` ← `BK_MONITOR_ES7_*`
- `MonitorInfluxDBHost / MonitorInfluxDBPort` ← `BK_INFLUXDB_IP0` 与 `BK_MONITOR_INFLUXDB_PORT`

#### Scenario: 凭据齐全

- **WHEN** 所有 `BK_MONITOR_*` / `BK_PAAS_MYSQL_*` / `BK_GSE_ZK_*` / `BK_INFLUXDB_IP0` 在 env 中设置
- **THEN** `Config` 对应字段非空且与 env 值一致

#### Scenario: 部分凭据缺失

- **WHEN** 仅设置 `BK_MONITOR_REDIS_HOST` 而 `BK_MONITOR_REDIS_PASSWORD` 缺失
- **THEN** `Config.MonitorRedisHost` 非空, `Config.MonitorRedisPassword` 为 `""`, 加载阶段不报错
