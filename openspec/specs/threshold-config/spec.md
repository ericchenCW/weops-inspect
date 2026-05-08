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

### Requirement: ES Heap 阈值

系统 SHALL 支持 `INSPECT_ES_HEAP_THRESHOLD` (单位：百分比) 以覆盖 ES 节点 `HeapPercent`
的 Notice 阈值，默认 `85`。

#### Scenario: 默认阈值
- **WHEN** `INSPECT_ES_HEAP_THRESHOLD` 未设置
- **THEN** `Config.Thresholds.ESHeapPercent` 等于 `85`

#### Scenario: env 覆盖
- **WHEN** `INSPECT_ES_HEAP_THRESHOLD=90`
- **THEN** `Config.Thresholds.ESHeapPercent` 等于 `90`

#### Scenario: 非法数字
- **WHEN** `INSPECT_ES_HEAP_THRESHOLD=oops`
- **THEN** `Config.Load()` 返回错误，错误信息包含变量名

### Requirement: ES RAM 阈值

系统 SHALL 支持 `INSPECT_ES_RAM_THRESHOLD` (单位：百分比) 以覆盖 ES 节点 `RAMPercent`
的 Notice 阈值，默认 `95`。

#### Scenario: 默认阈值
- **WHEN** `INSPECT_ES_RAM_THRESHOLD` 未设置
- **THEN** `Config.Thresholds.ESRAMPercent` 等于 `95`

#### Scenario: env 覆盖
- **WHEN** `INSPECT_ES_RAM_THRESHOLD=98`
- **THEN** `Config.Thresholds.ESRAMPercent` 等于 `98`

### Requirement: ES 未分配分片阈值

系统 SHALL 支持 `INSPECT_ES_UNASSIGNED_SHARDS_THRESHOLD` 以覆盖 ES 集群
`UnassignedShards` 的 Notice 阈值，默认 `0`（即只要大于 0 即 Notice）。

#### Scenario: 默认阈值
- **WHEN** `INSPECT_ES_UNASSIGNED_SHARDS_THRESHOLD` 未设置
- **THEN** `Config.Thresholds.ESUnassignedShards` 等于 `0`

### Requirement: Redis Celery 队列长度阈值

系统 SHALL 支持 `INSPECT_REDIS_CELERY_QUEUE_THRESHOLD` 以覆盖 Redis 节点 `CeleryQueue`
的 Notice 阈值，默认 `1000`。

#### Scenario: 默认阈值
- **WHEN** `INSPECT_REDIS_CELERY_QUEUE_THRESHOLD` 未设置
- **THEN** `Config.Thresholds.RedisCeleryQueue` 等于 `1000`

#### Scenario: env 覆盖
- **WHEN** `INSPECT_REDIS_CELERY_QUEUE_THRESHOLD=5000`
- **THEN** `Config.Thresholds.RedisCeleryQueue` 等于 `5000`

### Requirement: Redis Monitor 队列长度阈值

系统 SHALL 支持 `INSPECT_REDIS_MONITOR_QUEUE_THRESHOLD` 以覆盖 Redis 节点 `MonitorQueue`
的 Notice 阈值，默认 `10000`。

#### Scenario: 默认阈值
- **WHEN** `INSPECT_REDIS_MONITOR_QUEUE_THRESHOLD` 未设置
- **THEN** `Config.Thresholds.RedisMonitorQueue` 等于 `10000`

### Requirement: Docker 退出容器数阈值

系统 SHALL 支持 `INSPECT_DOCKER_EXITED_THRESHOLD` 以覆盖 Service 段 Docker
`ContainersExited` 的 Notice 阈值，默认 `0`。

#### Scenario: 默认阈值
- **WHEN** `INSPECT_DOCKER_EXITED_THRESHOLD` 未设置
- **THEN** `Config.Thresholds.ServiceContainersExited` 等于 `0`

#### Scenario: env 覆盖
- **WHEN** `INSPECT_DOCKER_EXITED_THRESHOLD=5`
- **THEN** `Config.Thresholds.ServiceContainersExited` 等于 `5`

#### Scenario: 非法数字
- **WHEN** `INSPECT_DOCKER_EXITED_THRESHOLD=abc`
- **THEN** `Config.Load()` 返回错误
