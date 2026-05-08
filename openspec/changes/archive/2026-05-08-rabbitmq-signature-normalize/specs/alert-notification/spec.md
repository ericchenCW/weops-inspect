## MODIFIED Requirements

### Requirement: 告警签名计算

系统 SHALL 基于本次巡检的 Warn 检查项计算稳定签名,用于跨次运行识别"告警集合是否
变化"。签名 MUST 仅由 `Status == Warn` 的检查项标识构成(host + field),不包含具体
数值,**不**包含 `Unknown` 或 `Notice` 项。

为避免 RabbitMQ 队列名漂移(如 celery 派生的瞬态队列名)造成签名抖动绕过冷却窗口,
系统 MUST 在签名计算前对 RabbitMQ 队列级 Field 做归一化:

- 形如 `rabbitmq.<vhost>.<queue>.no_consumer` 的 Field MUST 折叠为
  `rabbitmq.<vhost>.no_consumer`。
- 形如 `rabbitmq.<vhost>.<queue>.backlog` 的 Field MUST 折叠为
  `rabbitmq.<vhost>.backlog`。
- 其他 `rabbitmq.*` Field(集群级,如 `rabbitmq.error`、`rabbitmq.cluster_partition`、
  `rabbitmq.node.<n>.mem_alarm` 等)以及非 RabbitMQ Field MUST 保持原样。
- 折叠后产生的重复键 MUST 去重,使"同一 vhost 下 N 个队列触发同类告警"对签名贡献
  恒为单一键。

#### Scenario: 签名稳定性
- **WHEN** 两次巡检产生完全相同的 Warn 检查项集合(host + field 一致),但具体
  数值(如 CPU 76% vs 78%)不同
- **THEN** 两次计算得到的签名 MUST 相同

#### Scenario: 告警集合变化
- **WHEN** 本次新增或减少了任一 Warn 项(host 或 field 维度,且未被归一化规则吸收)
- **THEN** 本次签名 MUST 与上次签名不同

#### Scenario: Unknown 与 Notice 不影响签名
- **WHEN** 本次 Warn 集合不变,但 Unknown 项或 Notice 项有增减
- **THEN** 签名 MUST 与上次相同

#### Scenario: 无告警时的签名
- **WHEN** 本次 Warn 列表为空
- **THEN** 签名 MUST 为空字符串或固定哨兵值,与"有告警"场景可区分

#### Scenario: RabbitMQ 同 vhost 队列名漂移
- **WHEN** 两次巡检均在同一 vhost(如 `bk_bkmonitorv3`)报告 `no_consumer` 告警,
  但具体队列名集合不同(如 `{celery_api_cron, celery_cron, celery_service}` vs
  `{celery_alert_builder, celery_cron, celery_service_access_event}`)
- **THEN** 两次签名 MUST 相同

#### Scenario: RabbitMQ 同 vhost 队列数量变化
- **WHEN** 同一 vhost 下 `no_consumer` 队列数量在两次巡检间从 3 个变为 5 个,但 vhost
  本身保持不变
- **THEN** 两次签名 MUST 相同

#### Scenario: RabbitMQ 跨 vhost 新增告警
- **WHEN** 上次仅 vhost A 有 `no_consumer` 告警,本次新增 vhost B 也出现
  `no_consumer` 告警
- **THEN** 本次签名 MUST 与上次签名不同

#### Scenario: RabbitMQ backlog 与 no_consumer 互不合并
- **WHEN** 同一 vhost 上次仅 `backlog` 告警,本次仅 `no_consumer` 告警
- **THEN** 两次签名 MUST 不同

#### Scenario: RabbitMQ 集群级 Field 不被折叠
- **WHEN** 告警 Field 为 `rabbitmq.error`、`rabbitmq.cluster_partition` 或
  `rabbitmq.node.<n>.mem_alarm`
- **THEN** 这些 Field MUST 原样参与签名,不被三段式归一化规则吸收
