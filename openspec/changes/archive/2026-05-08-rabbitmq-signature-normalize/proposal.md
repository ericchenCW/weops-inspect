## Why

RabbitMQ `no_consumer` 与 `backlog` 告警的 Field 把队列名编进了告警身份（`rabbitmq.<vhost>.<queue>.no_consumer`）。在蓝鲸场景下,celery 的事件类/广播类队列名(如 `celery_api_cron`、`celery_alert_builder`、`celery_service_*`)是 worker 派生的瞬态名,**18 分钟内**就可能轮换一组,导致签名漂移、冷却窗口被绕过、同一类问题在短时间内重复推送。

## What Changes

- 签名计算阶段对 RabbitMQ 队列级告警的 Field 做**归一化折叠**: 同一 vhost 下的所有 `rabbitmq.<vhost>.<queue>.no_consumer` 折叠为单一签名键 `rabbitmq.<vhost>.no_consumer`; `backlog` 同理折叠为 `rabbitmq.<vhost>.backlog`。
- 仅影响**签名计算**这一处。`AlertItem` 列表、HTML 报告渲染、邮件正文明细、底层 `model.CheckResult` 全部保持原状,运维仍能在邮件中看到逐队列的具体告警。
- **BREAKING(行为)**: 同一 vhost 内队列轮换不再触发立即重发;运维需在冷却窗口(2 小时)结束后才会收到反映最新队列清单的邮件。属于**告警语义收敛**,需要在发布说明中点明。
- 非 RabbitMQ 的 Field 不受影响;Host 维度参与签名的语义不变。

## Capabilities

### New Capabilities
- (无)

### Modified Capabilities
- `alert-notification`: 「告警签名计算」 Requirement 增补一条 RabbitMQ 队列级 Field 的归一化规则,以及对应 Scenario(队列名漂移时签名稳定 / 跨 vhost 时签名变化)。

## Impact

- **代码**
  - [notify/signature.go](notify/signature.go) — 在哈希前对每个 key 应用 RabbitMQ 折叠规则。
  - [notify/signature_test.go](notify/signature_test.go) — 新增队列名漂移、跨 vhost、混合非 rabbitmq Field 的覆盖。
- **行为**
  - 同一 vhost 内 celery 派生队列轮换不再绕过冷却,推送频次降低。
  - 跨 vhost 新增 `no_consumer` / `backlog` 告警仍会立即触发(签名变化)。
  - 报告 HTML 与邮件正文显示**不变**——展示与去重解耦。
- **依赖 / 接口**: 无外部 API 变更; `model.CheckResult` schema 不变; state.json 格式不变(仍为同一字段,只是值域因归一化而稳定)。
- **与既有变更的关系**: 与 in-progress 的 `rabbitmq-queue-backlog-filtering` 正交并互补——后者治"采集层噪声",本提案治"通知层签名抖动"。
