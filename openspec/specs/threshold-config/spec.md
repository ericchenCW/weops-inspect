# threshold-config Specification

## Purpose

定义巡检报告中各类资源使用率与运行天数的阈值默认值与 env 覆盖规则。

## Requirements

### Requirement: 阈值默认值

系统 SHALL 在 env 未设置时使用以下阈值默认值:

- CPU 使用率:75
- 磁盘使用率:75
- inode 使用率:75
- 内存使用率:75
- 最大文件句柄数:102400
- 主机运行天数:365

#### Scenario: 全部使用默认值

- **WHEN** 所有 `INSPECT_*_THRESHOLD` / `INSPECT_MAX_OPEN_FILES` / `INSPECT_RUN_DAYS` 都未设置
- **THEN** `Config.Thresholds` 各字段等于上述默认值

### Requirement: env 覆盖阈值

系统 SHALL 允许通过下列 env 覆盖阈值:

- `INSPECT_CPU_THRESHOLD` → CPU 使用率
- `INSPECT_DISK_THRESHOLD` → 磁盘使用率
- `INSPECT_INODE_THRESHOLD` → inode 使用率
- `INSPECT_MEM_THRESHOLD` → 内存使用率
- `INSPECT_MAX_OPEN_FILES` → 最大文件句柄数
- `INSPECT_RUN_DAYS` → 主机运行天数

#### Scenario: 覆盖 CPU 阈值

- **WHEN** `INSPECT_CPU_THRESHOLD=85`
- **THEN** `Config.Thresholds.CPUUsage` 等于 `85`

### Requirement: 阈值解析失败硬退出

系统 SHALL 在 env 提供了非法数字时(如 `INSPECT_CPU_THRESHOLD=abc`)立即返回错误,使主进程退出,不静默退回默认值。

#### Scenario: 非法数字

- **WHEN** `INSPECT_DISK_THRESHOLD=not-a-number`
- **THEN** `Config.Load()` 返回错误,错误信息包含变量名 `INSPECT_DISK_THRESHOLD`

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
