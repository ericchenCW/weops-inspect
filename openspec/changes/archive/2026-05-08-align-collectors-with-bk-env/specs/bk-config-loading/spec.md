## ADDED Requirements

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
