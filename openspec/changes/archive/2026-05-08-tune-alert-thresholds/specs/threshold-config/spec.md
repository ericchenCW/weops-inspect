## MODIFIED Requirements

### Requirement: 阈值默认值

系统 SHALL 在 env 未设置时使用以下阈值默认值:

- CPU 使用率:95
- 磁盘使用率:95
- inode 使用率:95
- 内存使用率:95
- 最大文件句柄数:65536

#### Scenario: 全部使用默认值

- **WHEN** 所有 `INSPECT_*_THRESHOLD` / `INSPECT_MAX_OPEN_FILES` 都未设置
- **THEN** `Config.Thresholds` 各字段等于上述默认值

### Requirement: env 覆盖阈值

系统 SHALL 允许通过下列 env 覆盖阈值:

- `INSPECT_CPU_THRESHOLD` → CPU 使用率
- `INSPECT_DISK_THRESHOLD` → 磁盘使用率
- `INSPECT_INODE_THRESHOLD` → inode 使用率
- `INSPECT_MEM_THRESHOLD` → 内存使用率
- `INSPECT_MAX_OPEN_FILES` → 最大文件句柄数

#### Scenario: 覆盖 CPU 阈值

- **WHEN** `INSPECT_CPU_THRESHOLD=85`
- **THEN** `Config.Thresholds.CPUUsage` 等于 `85`

## ADDED Requirements

### Requirement: RabbitMQ 0 消费者 vhost 黑名单

系统 SHALL 支持 `INSPECT_RABBITMQ_NO_CONSUMER_VHOST_BLACKLIST`(逗号分隔字符串)以指定一组在 "队列 0 消费者" 检查中需被忽略的 vhost。该黑名单仅作用于 0 消费者告警, 队列堆积阈值告警(`INSPECT_RABBITMQ_QUEUE_BACKLOG_THRESHOLD`)对所有 vhost 仍照常生效。env 未设置时默认包含 `bk_bknodeman`。

#### Scenario: 默认黑名单包含 bk_bknodeman

- **WHEN** `INSPECT_RABBITMQ_NO_CONSUMER_VHOST_BLACKLIST` 未设置
- **THEN** `Config.Thresholds.RabbitMQNoConsumerVHostBlacklist` 等于 `["bk_bknodeman"]`

#### Scenario: env 覆盖黑名单

- **WHEN** `INSPECT_RABBITMQ_NO_CONSUMER_VHOST_BLACKLIST=foo,bar`
- **THEN** `Config.Thresholds.RabbitMQNoConsumerVHostBlacklist` 等于 `["foo", "bar"]`(完全替换默认值)

#### Scenario: 黑名单 vhost 下队列 0 消费者不告警

- **WHEN** vhost `bk_bknodeman` 下某队列 `consumers=0`
- **THEN** 该队列不产生 0 消费者告警

#### Scenario: 黑名单 vhost 下队列堆积仍告警

- **WHEN** vhost `bk_bknodeman` 下某队列 `messages` 超过 `INSPECT_RABBITMQ_QUEUE_BACKLOG_THRESHOLD`
- **THEN** 仍产生队列堆积告警

#### Scenario: 非黑名单 vhost 0 消费者照常告警

- **WHEN** vhost `/` 下某队列 `consumers=0`
- **THEN** 该队列产生 0 消费者告警
