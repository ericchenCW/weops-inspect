## Context

`CollectRabbitMQ`([collector/rabbitmq.go](collector/rabbitmq.go))调用 RabbitMQ Management API `/api/queues`,把消息数 > 1000 的队列写入 `ExceedingQueues`,把 `consumers==0 && messages>0` 的队列写入 `NoConsumerQueues`。模板([render/templates/opensources.html.tmpl](render/templates/opensources.html.tmpl))只渲染了 `ExceedingQueues`,`NoConsumerQueues` 字段已采集但未展示。

实际部署中:
- `bk_usermgr` vhost 内的队列长期保持高水位是业务模型决定的,不属于运维需要关注的异常。
- Celery 事件队列 `celeryev*` 由 worker 心跳消息组成,堆积量大但不影响业务。
- 阈值 1000 远低于实际集群业务量,导致每次巡检都产生大量"积压"行,信号被噪声淹没。

阈值与过滤需要在采集层一次解决,避免下游(模板/告警/外部消费方)各自实现一遍。

## Goals / Non-Goals

**Goals:**
- 把"是否进入告警/报告"的决策集中在采集层。
- 阈值通过 env 可调,与项目现有 `INSPECT_*_THRESHOLD` 风格保持一致。
- 在不改 `RabbitMQStatus` JSON schema 的前提下完成本次变更。
- 把已经采集但未渲染的"无消费者队列"补到 HTML 报告中。

**Non-Goals:**
- 不引入"按 vhost 汇总"或"按 vhost 分组渲染"等聚合视图(用户已确认现状一行一 vhost 列即可)。
- 不改"无消费者"的判定语义(保留 `consumers==0 && messages>0`,空队列不告警)。
- 不引入多档告警(只有"超阈值"一档,默认 10000)。
- 不为过滤规则做"白名单/黑名单可配置"——`bk_usermgr` 与 `celeryev` 直接写死在采集逻辑中,与蓝鲸项目场景强绑定。

## Decisions

### Decision 1: 过滤发生在采集层(`A` 方案),而非渲染/告警层

被过滤的队列彻底不进入 `ExceedingQueues` / `NoConsumerQueues`,JSON 输出与 HTML 报告里都看不到。

**为什么:** 用户明确选择 A,且渲染层过滤会带来"采集结构里有但报告里没有"的不一致,反而增加排查成本。集中在采集层,语义干净。

**代价:** 如果未来有人需要看 `bk_usermgr` 的队列,只能改代码或新增 env。考虑到这两类过滤都是确定性业务决策,不算实质损失。

### Decision 2: 过滤规则硬编码在 `CollectRabbitMQ` 中,而非走配置

判定逻辑近似:

```
if vhost == "bk_usermgr"            → skip
if HasPrefix(name, "celeryev")      → skip
```

**为什么:** 这是对蓝鲸部署形态的特化决策,跟"凭据/IP/阈值"这类环境变量是不同维度的东西。塞到 env 里反而模糊了"这是项目级约定"这一信息;真要变更,从代码层改更显眼。

**备选:** 提供 `INSPECT_RABBITMQ_VHOST_BLACKLIST` / `INSPECT_RABBITMQ_QUEUE_NAME_PREFIX_BLACKLIST` 两个 env。被否决,理由同上 — 当前需求里没有"按集群定制过滤"的场景。

### Decision 3: 阈值通过 env 配置,默认 10000

新增 `INSPECT_RABBITMQ_QUEUE_BACKLOG_THRESHOLD`,落到 `Config.Thresholds.RabbitMQQueueBacklog`。复用现有阈值解析路径(同 `INSPECT_MYSQL_REPL_LAG_THRESHOLD`):未设置走默认,非法数字硬退出。

**为什么:** 阈值是与集群体量相关的运维参数,跟现有可配置阈值同构。

### Decision 4: 模板新增"无消费者队列"表

`NoConsumerQueues` 字段从一开始就在采集,只是模板没用。这次顺手补上,避免再起一次 change 的开销;新增表与现有"消息积压"表样式一致(VHost / Queue / MessageCount / Consumers)。

**为什么:** 用户在第 4 点确认"消费者为 0 时也是异常的",虽然采集端早就标了,但报告里看不到等于"没有"。

### Decision 5: 阈值字段命名 `RabbitMQQueueBacklog`

不带单位后缀(对比 `MySQLReplLagSec` 带 `Sec`),因为消息数本身就是无单位整数,加后缀反而拗口。

## Risks / Trade-offs

- **[默认阈值从 1000 抬到 10000,语义变更]** → 在 release notes / commit message 中明确"默认阈值变化"; 升级后产生的告警量会显著下降,需要让运维感知到这是预期行为而非告警丢失。
- **[硬编码 `bk_usermgr` / `celeryev` 过滤]** → 一旦未来部署形态变化(如重命名 vhost),需要改代码而非改配置。可接受 — 改动小且语义透明。
- **[`celeryev` 前缀匹配可能误伤]** → 任何以 `celeryev` 开头的业务队列都会被过滤。约定俗成 celery 事件队列就是这个前缀,误伤风险极低;若出现可在文档中说明。
- **[模板新增"无消费者队列"表可能让报告变长]** → 由于过滤已经把 celeryev / bk_usermgr 排除,实际剩下的"无消费者"队列基本是真异常,行数可控。

## Migration Plan

1. 部署新版本前,在变更说明中告知运维:阈值默认从 1000 → 10000,`bk_usermgr` 与 `celeryev*` 不再出现在报告中。
2. 若特定环境希望保留 1000 阈值,显式设置 `INSPECT_RABBITMQ_QUEUE_BACKLOG_THRESHOLD=1000`。
3. 回滚:无数据迁移,直接回滚二进制即可,行为完全兼容。
