## ADDED Requirements

### Requirement: RabbitMQ 队列积压阈值

系统 SHALL 支持 `INSPECT_RABBITMQ_QUEUE_BACKLOG_THRESHOLD`(单位:消息条数)以覆盖 RabbitMQ 队列积压告警阈值,默认 `10000`,落到 `Config.Thresholds.RabbitMQQueueBacklog`。解析行为遵循"非法数字硬退出"通用约定。

#### Scenario: 默认阈值

- **WHEN** `INSPECT_RABBITMQ_QUEUE_BACKLOG_THRESHOLD` 未设置
- **THEN** `Config.Thresholds.RabbitMQQueueBacklog` 等于 `10000`

#### Scenario: env 覆盖

- **WHEN** `INSPECT_RABBITMQ_QUEUE_BACKLOG_THRESHOLD=20000`
- **THEN** `Config.Thresholds.RabbitMQQueueBacklog` 等于 `20000`

#### Scenario: 非法数字

- **WHEN** `INSPECT_RABBITMQ_QUEUE_BACKLOG_THRESHOLD=not-a-number`
- **THEN** `Config.Load()` 返回错误,错误信息包含变量名 `INSPECT_RABBITMQ_QUEUE_BACKLOG_THRESHOLD`
