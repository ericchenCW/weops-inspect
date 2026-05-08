## ADDED Requirements

### Requirement: MySQL 复制延迟阈值

系统 SHALL 支持 `INSPECT_MYSQL_REPL_LAG_THRESHOLD`(单位:秒)以覆盖 MySQL slave `Seconds_Behind_Master` 的告警阈值,默认 `60`。

#### Scenario: 默认阈值

- **WHEN** `INSPECT_MYSQL_REPL_LAG_THRESHOLD` 未设置
- **THEN** `Config.Thresholds.MySQLReplLagSec` 等于 `60`

#### Scenario: env 覆盖

- **WHEN** `INSPECT_MYSQL_REPL_LAG_THRESHOLD=120`
- **THEN** `Config.Thresholds.MySQLReplLagSec` 等于 `120`

#### Scenario: 非法数字

- **WHEN** `INSPECT_MYSQL_REPL_LAG_THRESHOLD=abc`
- **THEN** `Config.Load()` 返回错误,错误信息包含变量名

### Requirement: Redis 复制 IO 阈值

系统 SHALL 支持 `INSPECT_REDIS_REPL_IO_THRESHOLD`(单位:秒)以覆盖 Redis slave `master_last_io_seconds_ago` 的告警阈值,默认 `10`。

#### Scenario: 默认阈值

- **WHEN** `INSPECT_REDIS_REPL_IO_THRESHOLD` 未设置
- **THEN** `Config.Thresholds.RedisReplIOSec` 等于 `10`

#### Scenario: env 覆盖

- **WHEN** `INSPECT_REDIS_REPL_IO_THRESHOLD=30`
- **THEN** `Config.Thresholds.RedisReplIOSec` 等于 `30`

#### Scenario: 非法数字

- **WHEN** `INSPECT_REDIS_REPL_IO_THRESHOLD=oops`
- **THEN** `Config.Load()` 返回错误,错误信息包含变量名
