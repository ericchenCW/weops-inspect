## ADDED Requirements

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
