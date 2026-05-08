## ADDED Requirements

### Requirement: 渲染结构携带 Status 字段

系统 SHALL 在以下 collector 产出的渲染结构上携带 `Status string`（取值
`"ok"` / `"warn"` / `"unknown"` / `"notice"` / `""`）字段：

- `model.ESCluster`、`model.ESNode`、`model.ESNodeReach`
- `model.RedisStandaloneNode`、`model.RedisSentinelStatus`、`model.SentinelNode`
- `model.MongoMember`
- `model.RabbitMQQueue`、`model.RabbitMQAlarm`、`model.RabbitMQVHostSummary`
- `model.DependencyResult`
- `model.ServiceModule`、`model.DockerSummary`

`Status` 字段 MUST 由 checker 层填写，collector MUST 留空。
模板 MUST 仅依据 `Status` 字段决定红/绿/灰着色，MUST NOT 内联进行阈值或常量比较。

#### Scenario: collector 留空 Status
- **WHEN** `CollectRabbitMQ` 返回的 `RabbitMQStatus.ExceedingQueues[0].Status`
- **THEN** 该字段值 SHALL 为空字符串 `""`

#### Scenario: checker 填充 Status
- **WHEN** `CheckRabbitMQ` 处理同一 `RabbitMQStatus`
- **THEN** 每个 ExceedingQueues 元素的 `Status` SHALL 被回填为 `"warn"`

### Requirement: collector 阈值判定下沉到 checker

系统 SHALL 把"是否告警"的阈值比较职责下沉到 checker。collector MUST 仅负责采集与
必要的"汇总切片"产出（如 ExceedingQueues / NoConsumerQueues），MUST NOT 在采集阶段
回填 Status 字段。例外：已存在的 `RabbitMQQueueBacklog` 与
`RabbitMQNoConsumerVHostBlacklist` 仍由 collector 作为筛选切片的判定条件。

#### Scenario: ES heap/RAM 由 checker 判定
- **WHEN** ES 节点 `HeapPercent == 90`
- **THEN** `CollectES` MUST 不在该节点上设置任何 `Status` 字段
- **AND** `CheckES` SHALL 根据 `Thresholds.ESHeapPercent` 决定 Status

#### Scenario: RabbitMQ 切片筛选保留
- **WHEN** RabbitMQ 队列 `prod_bk_monitorv3.celery.MessageCount == 360547` 且 `Thresholds.RabbitMQQueueBacklog == 1000`
- **THEN** `CollectRabbitMQ` SHALL 仍然把该队列追加到 `ExceedingQueues`（保留现有筛选行为）
- **AND** `CheckRabbitMQ` SHALL 把该队列转换为一条 Warn CheckResult
