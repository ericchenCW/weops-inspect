## ADDED Requirements

### Requirement: RabbitMQ 队列采集 vhost / 队列名过滤

系统 SHALL 在 `CollectRabbitMQ` 遍历 `/api/queues` 返回结果时,采集阶段直接丢弃以下队列,使其不进入 `ExceedingQueues`、`NoConsumerQueues` 或任何下游(JSON / HTML 报告)字段:

- `vhost` 等于 `bk_usermgr` 的队列。
- 队列名(`name` 字段)以 `celeryev` 为前缀的队列。

#### Scenario: bk_usermgr vhost 被过滤

- **WHEN** RabbitMQ 返回的队列列表中存在 `vhost=bk_usermgr` 且 `messages=20000` 的队列
- **THEN** 该队列既不出现在 `RabbitMQStatus.ExceedingQueues`,也不出现在 `NoConsumerQueues`

#### Scenario: celeryev 前缀队列被过滤

- **WHEN** RabbitMQ 返回的队列名为 `celeryev.worker-01`,`messages=50000`,`consumers=0`
- **THEN** 该队列既不出现在 `ExceedingQueues`,也不出现在 `NoConsumerQueues`

#### Scenario: 普通业务队列正常进入判定

- **WHEN** RabbitMQ 返回的队列 `vhost=bk_cmdb`、`name=task_queue`、`messages=15000`、`consumers=2`
- **THEN** 该队列被正常计入消息积压判定流程,不被过滤

### Requirement: RabbitMQ 队列积压阈值可配置

系统 SHALL 在判定队列消息积压时,使用 `Config.Thresholds.RabbitMQQueueBacklog` 作为阈值,默认 `10000`,并支持通过环境变量覆盖。当队列消息数大于等于该阈值时,SHALL 写入 `RabbitMQStatus.ExceedingQueues`。

#### Scenario: 默认阈值

- **WHEN** `INSPECT_RABBITMQ_QUEUE_BACKLOG_THRESHOLD` 未设置,且队列 `messages=10000`、未被 vhost / 前缀过滤规则丢弃
- **THEN** 该队列写入 `ExceedingQueues`

#### Scenario: 低于默认阈值不告警

- **WHEN** 队列 `messages=9999`、未被过滤规则丢弃
- **THEN** 该队列不写入 `ExceedingQueues`

#### Scenario: env 覆盖

- **WHEN** `INSPECT_RABBITMQ_QUEUE_BACKLOG_THRESHOLD=1000`,队列 `messages=1500`、未被过滤规则丢弃
- **THEN** 该队列写入 `ExceedingQueues`

### Requirement: RabbitMQ 无消费者队列展示

系统 SHALL 在 HTML 报告中渲染 `RabbitMQStatus.NoConsumerQueues`,展示通过过滤规则后仍出现 `consumers==0 && messages>0` 的队列,字段至少包含 VHost、Queue、MessageCount、Consumers。

#### Scenario: 无消费者队列出现在报告中

- **WHEN** 队列 `vhost=bk_cmdb`、`name=task_queue`、`messages=5`、`consumers=0`、未被过滤规则丢弃
- **THEN** HTML 报告 RabbitMQ 区块出现"无消费者队列"表格,包含上述队列一行

#### Scenario: 空队列无消费者不告警

- **WHEN** 队列 `messages=0`、`consumers=0`
- **THEN** 该队列不写入 `NoConsumerQueues`,HTML 中也不显示

#### Scenario: 被过滤队列不计入无消费者

- **WHEN** 队列 `name=celeryev.foo`、`messages=10`、`consumers=0`
- **THEN** 该队列不出现在"无消费者队列"表中
