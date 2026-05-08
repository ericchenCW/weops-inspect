## Why

当前 RabbitMQ 队列采集对所有 vhost 一视同仁,且把"消息数 > 1000"硬编码为告警阈值。在蓝鲸生产环境中:

- `bk_usermgr` 这类业务 vhost 队列长期堆积属于正常工作模式,持续误报会淹没真正的异常。
- Celery 的事件队列(`celeryev*`)本质上是事件流,堆积量本就动辄上万,无运维价值。
- 1000 这个阈值在实际集群中过低,导致几乎每次巡检都会出现一片"积压"行,降低了报告可信度。

需要把告警维度收敛到**真正需要关注的业务队列**,并让阈值可被运维按集群体量调整。

## What Changes

- 在 `CollectRabbitMQ` 的队列遍历中,**采集时直接丢弃** `vhost == "bk_usermgr"` 以及队列名前缀为 `celeryev` 的队列,使其完全不出现在 `ExceedingQueues` / `NoConsumerQueues` / 报告中。
- 把"消息积压"判定阈值从硬编码的 `1000` 改为 `Config.Thresholds.RabbitMQQueueBacklog`,默认 `10000`。
- 新增环境变量 `INSPECT_RABBITMQ_QUEUE_BACKLOG_THRESHOLD` 用于覆盖默认阈值,行为与现有 `INSPECT_*_THRESHOLD` 一致(非法数字立即返回错误)。
- HTML 模板中"消息积压队列"一节的标题与阈值同步显示。
- HTML 模板补充渲染目前已采集但未展示的 `NoConsumerQueues`(无消费者队列)表格。
- "无消费者"判定保留现状语义:`consumers == 0 && messages > 0`,不对空队列误报。

## Capabilities

### New Capabilities
- (无)

### Modified Capabilities
- `infra-component-collection`:RabbitMQ 队列采集新增 vhost / 队列名前缀过滤规则,新增"无消费者队列"展示要求,积压阈值改为可配置。
- `threshold-config`:新增 `INSPECT_RABBITMQ_QUEUE_BACKLOG_THRESHOLD` 阈值配置项。

## Impact

- **代码**
  - [collector/rabbitmq.go](collector/rabbitmq.go) — 队列遍历逻辑,新增过滤与阈值引用。
  - [config/config.go](config/config.go) — `Thresholds` 结构与 `Load()` 解析新增 env。
  - [model/types.go](model/types.go) — 无需结构变更(`RabbitMQQueue` 已含 VHost / Consumers)。
  - [render/templates/opensources.html.tmpl](render/templates/opensources.html.tmpl) — 标题文案 + 新增"无消费者队列"表。
- **行为**
  - 报告中 `bk_usermgr` 与 `celeryev*` 队列彻底消失(包括无消费者列表)。
  - 旧默认 `1000` 提升到 `10000`,告警量显著下降 — **属于告警语义变更**,需在发布说明中点明。
- **依赖 / 接口**:无外部 API 变更;不影响 `model.RabbitMQStatus` 的 JSON schema。
