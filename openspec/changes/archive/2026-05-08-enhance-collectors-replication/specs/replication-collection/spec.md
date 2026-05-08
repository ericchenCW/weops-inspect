## ADDED Requirements

### Requirement: MySQL slave 复制状态采集

系统 SHALL 对 `Config.MySQLSlaveIPs` 中每个节点执行 `SHOW SLAVE STATUS`,记录以下字段到报告:`Slave_IO_Running`、`Slave_SQL_Running`、`Seconds_Behind_Master`、`Last_IO_Error`、`Last_SQL_Error`、`Master_Host`。

#### Scenario: slave 复制健康

- **WHEN** slave 节点 `Slave_IO_Running=Yes` 且 `Slave_SQL_Running=Yes` 且 `Seconds_Behind_Master <= INSPECT_MYSQL_REPL_LAG_THRESHOLD`
- **THEN** 报告中该节点复制状态标记为 `ok`

#### Scenario: 复制线程停止

- **WHEN** slave 节点 `Slave_IO_Running=No` 或 `Slave_SQL_Running=No`
- **THEN** 报告中该节点复制状态标记为 `critical`,并附带 `Last_IO_Error` / `Last_SQL_Error` 文本

#### Scenario: 复制延迟超阈值

- **WHEN** `Slave_IO_Running=Yes` 且 `Slave_SQL_Running=Yes`,但 `Seconds_Behind_Master > INSPECT_MYSQL_REPL_LAG_THRESHOLD`
- **THEN** 报告中该节点复制状态标记为 `warn`,记录实际秒数

#### Scenario: Seconds_Behind_Master 为 NULL

- **WHEN** `SHOW SLAVE STATUS` 返回 `Seconds_Behind_Master` 为 NULL
- **THEN** 报告中视为 `0` 处理,不报错

#### Scenario: 节点不是 slave

- **WHEN** 对 `Config.MySQLSlaveIPs` 中节点执行 `SHOW SLAVE STATUS` 但返回空结果集
- **THEN** 报告中标记 `not-configured-as-slave`,警告级 warn

### Requirement: MySQL master read_only 校验

系统 SHALL 对 `Config.MySQLMasterIPs` 中每个节点查询 `read_only` 全局变量。

#### Scenario: master 可写

- **WHEN** master 节点 `read_only=OFF`
- **THEN** 报告中该节点标记为 `ok`

#### Scenario: master 被设为只读

- **WHEN** master 节点 `read_only=ON`
- **THEN** 报告中该节点标记为 `warn`,提示 "master configured as read_only"

### Requirement: Redis 角色校验与同步状态采集

系统 SHALL 对 `Config.RedisMasterIPs` 与 `Config.RedisSlaveIPs` 中每个节点执行 `INFO replication`,记录:

- 所有节点:`role`
- master 节点:`connected_slaves`
- slave 节点:`master_host`、`master_port`、`master_link_status`、`master_last_io_seconds_ago`、`master_sync_in_progress`

#### Scenario: 角色与 env 一致

- **WHEN** `Config.RedisMasterIPs` 中节点 `INFO replication` 返回 `role:master`
- **THEN** 报告中角色一致性标记为 `ok`

#### Scenario: 角色与 env 不一致

- **WHEN** `Config.RedisMasterIPs` 中节点 `INFO replication` 返回 `role:slave`,或 `Config.RedisSlaveIPs` 中节点返回 `role:master`
- **THEN** 报告中角色一致性标记为 `warn`,提示 "actual role does not match env"

#### Scenario: slave 与 master 链路正常

- **WHEN** slave 节点 `master_link_status=up` 且 `master_last_io_seconds_ago <= INSPECT_REDIS_REPL_IO_THRESHOLD`
- **THEN** 报告中该 slave 同步状态标记为 `ok`

#### Scenario: slave 与 master 链路断开

- **WHEN** slave 节点 `master_link_status != up`
- **THEN** 报告中该 slave 同步状态标记为 `critical`

#### Scenario: slave 同步延迟超阈值

- **WHEN** slave 节点 `master_link_status=up` 但 `master_last_io_seconds_ago > INSPECT_REDIS_REPL_IO_THRESHOLD`
- **THEN** 报告中该 slave 同步状态标记为 `warn`,记录实际秒数

### Requirement: Sentinel 选举与 env master 交叉校验

系统 SHALL 把姊妹 change 中 `CollectRedisSentinel` 通过 `SENTINEL get-master-addr-by-name` 发现的 master 地址,与 `Config.RedisMasterIPs` 做集合对照。

#### Scenario: Sentinel 与 env 一致

- **WHEN** Sentinel 发现的 master IP 存在于 `Config.RedisMasterIPs` 列表中
- **THEN** 报告中交叉校验项标记为 `ok`

#### Scenario: Sentinel 选出的 master 不在 env 配置中

- **WHEN** Sentinel 发现的 master IP 不在 `Config.RedisMasterIPs` 列表中
- **THEN** 报告中交叉校验项标记为 `warn`,提示 "actual master differs from env config",同时记录两个值

### Requirement: env 缺失主从配置时的兼容行为

系统 SHALL 在 `Config.MySQLMasterIPs / MySQLSlaveIPs / RedisMasterIPs / RedisSlaveIPs` 任一为空时,跳过对应的复制采集步骤,不报错。

#### Scenario: 旧版 bk.env 无主从配置

- **WHEN** `BK_MYSQL_MASTER_IP_COMMA` 与 `BK_MYSQL_SLAVE_IP_COMMA` 都未设置
- **THEN** 报告中 MySQL 复制段标记为 `N/A — env 未配置主从信息`,平权探活仍正常运行

#### Scenario: 仅 master 配置但无 slave

- **WHEN** `BK_MYSQL_MASTER_IP_COMMA` 有值,`BK_MYSQL_SLAVE_IP_COMMA` 为空
- **THEN** 系统执行 master `read_only` 校验,跳过 slave 复制状态采集
