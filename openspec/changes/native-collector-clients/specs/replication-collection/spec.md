## ADDED Requirements

### Requirement: Redis 主从复制采集走原生驱动

系统 SHALL 使用 `github.com/redis/go-redis/v9` 的原生客户端对 `Config.RedisMasterIPs` / `Config.RedisSlaveIPs` 中节点执行 `INFO replication`,SHALL NOT 通过 `exec.Command("redis-cli", ...)` 调用外部二进制。INFO 解析 MUST 与原 CLI 文本解析行为对齐(同一 key 的取值与判定逻辑保持一致)。

#### Scenario: 原生驱动采集 master

- **WHEN** master 节点通过 go-redis 客户端调用 `INFO replication`
- **THEN** 返回的 `role` / `connected_slaves` 等字段被正确写入报告,与原行为一致

#### Scenario: 原生驱动采集 slave

- **WHEN** slave 节点通过 go-redis 客户端调用 `INFO replication`
- **THEN** `master_host` / `master_port` / `master_link_status` / `master_last_io_seconds_ago` / `master_sync_in_progress` 字段被正确写入报告

#### Scenario: 巡检机无 redis-cli

- **WHEN** 巡检机未安装 `redis-cli`
- **THEN** 主从采集仍能完成,不再因二进制缺失返回错误

### Requirement: MySQL 主从采集走原生驱动

系统 SHALL 通过 `database/sql` + `github.com/go-sql-driver/mysql` 对 `Config.MySQLSlaveIPs` 执行 `SHOW SLAVE STATUS` / `SHOW REPLICA STATUS`,对 `Config.MySQLMasterIPs` 查询 `read_only`,SHALL NOT 调用外部 `mysql` 二进制。

#### Scenario: 原生驱动 slave 状态

- **WHEN** slave 节点通过原生驱动查询复制状态
- **THEN** 报告中 `Slave_IO_Running` / `Slave_SQL_Running` / `Seconds_Behind_Master` / `Last_IO_Error` / `Last_SQL_Error` / `Master_Host` 字段值与原 CLI 行为一致

#### Scenario: 原生驱动 master read_only

- **WHEN** master 节点通过原生驱动查询 `@@global.read_only`
- **THEN** 报告中 `read_only` 字段被正确填充

### Requirement: 复制采集错误携带 ErrorClass

系统 SHALL 在 MySQL / Redis 主从采集结果中,把每节点的错误结果按 `collector-probe-framework` 的 `ErrorClass` 枚举打标(`network` / `auth` / `protocol` / `timeout` / `unknown`)。

#### Scenario: slave 节点不可达

- **WHEN** slave 节点 TCP 不可达
- **THEN** 该节点复制状态项 `ErrorClass = "network"`,整体复制段不因此中断

#### Scenario: 鉴权失败

- **WHEN** 凭据不正确
- **THEN** 该节点复制状态项 `ErrorClass = "auth"`
