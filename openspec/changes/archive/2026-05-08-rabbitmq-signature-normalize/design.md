## Context

`notify/signature.go` 当前以每个 Warn 检查项的 `Host + "|" + Field` 作为签名键,做 sha256 后用于跨次去重。在 RabbitMQ 队列级告警里,Field 形如 `rabbitmq.<vhost>.<queue>.no_consumer` 与 `rabbitmq.<vhost>.<queue>.backlog`(见 [checker/rabbitmq.go:80](checker/rabbitmq.go:80) / [checker/rabbitmq.go:90](checker/rabbitmq.go:90))。

bk_bkmonitorv3 等 vhost 中,celery 的事件类/广播类队列名是 worker 派生、随调度漂移的(`celery_api_cron` ↔ `celery_alert_builder`,`celery_service` ↔ `celery_service_access_event`)。在两次相邻巡检之间,即便业务影响完全相同(同一 vhost 内仍有若干队列无消费者),签名也会因为 Field 不同而变化,从而绕过 [notify/trigger.go:35](notify/trigger.go:35) 的冷却判断,触发立即重发。

实测样本: [/private/tmp/a/weops_inspection.html](/private/tmp/a/weops_inspection.html) (22:44) 与 [/private/tmp/a/weops_inspection_1.html](/private/tmp/a/weops_inspection_1.html) (23:02) 相隔 18 分钟,9 项告警在数量与业务含义上一致,仅因队列名漂移就连发两封。

## Goals / Non-Goals

**Goals:**

- 同一 vhost 内 RabbitMQ 队列名漂移时签名稳定 → 冷却窗口生效。
- 跨 vhost 出现新的 `no_consumer` / `backlog` 告警时签名仍变化 → 立即触发新告警。
- 改动仅限 `notify/signature.go`,不影响采集层、报告渲染、邮件正文展示。
- 行为可被现有单元测试与新增测试一并覆盖。

**Non-Goals:**

- 不修改 `model.CheckResult` / `AlertItem` 结构,不裁剪报告中已渲染的逐队列细节。
- 不引入额外配置项(归一化规则是固定的,不依赖运维选择)。
- 不替代 `rabbitmq-queue-backlog-filtering` 在采集层做的噪声过滤,两者各司其职。
- 不调整冷却窗口长度(仍 2 小时)或 `min_interval_minutes` 默认值。

## Decisions

### 决策 1: 在签名层归一化,而非在 Field 层改写

**选项 A**: 修改 `checker/rabbitmq.go`,把 `Field` 直接定义为 `rabbitmq.<vhost>.no_consumer`,Value 写"3 个队列: a, b, c"。

**选项 B(采纳)**: 保留 checker / model / 渲染层的逐队列 Field,仅在 `notify/signature.go` 计算签名时把 RabbitMQ 队列级 Field 折叠到 vhost 维度。

**理由:**

- 报告 HTML 与邮件正文的核心价值是"出问题的具体队列名",B 不会损失这个细节。
- 展示与去重是两件事——B 让二者解耦,后续若想再调整去重策略(例如再加 `prod_*` 维度白名单)只需改一个文件。
- A 会牵动 `render/templates/opensources.html.tmpl`、相关 collector 测试、可能还有 `model` 层注释,变更面更大。
- B 的归一化函数纯函数、易测试,失败模式被局限在签名计算这一处。

### 决策 2: 折叠规则的精确形态

对每个签名键 `Host|Field`,在哈希前应用以下变换:

```
若 Field 匹配正则  ^rabbitmq\.([^.]+)\.(.+)\.(no_consumer|backlog)$
则将 Field 重写为  rabbitmq.$1.$3
```

即 vhost 段保留、`no_consumer`/`backlog` 后缀保留、中间的队列名段被吃掉。Host 字段对 RabbitMQ 告警通常为空(集群级 finding),保留原状参与签名。

**为什么不把 backlog 与 no_consumer 也合并?** 二者代表不同问题(积压 vs 无消费者),合并会导致"积压恢复但仍无消费者"这种状态变化无法通过签名感知。保留后缀级区分。

**为什么不引入运维可调的归一化规则配置?** YAGNI——目前只有 RabbitMQ 一类 Field 出现"队列名漂移"问题,引入通用规则引擎会过度设计。如未来出现第二个类似源,再抽象。

### 决策 3: 折叠后的多重计数处理

折叠后,同一 vhost 的 N 个队列产生的 Field 全部塌缩为同一个 key。已有的 `sort.Strings(keys)` + 哈希流程对重复 key 是稳定的(同一字符串写两次和写一次会得到不同哈希),需要在折叠后**去重**,否则"3 个队列无消费者"与"5 个队列无消费者"会得到不同签名,违背设计意图。

实现: 折叠后用 `map[string]struct{}` 去重,再 `sort.Strings` 后哈希。

### 决策 4: 折叠前提失败时的回退

正则不匹配的 Field(包括所有非 RabbitMQ Field、以及 `rabbitmq.error` / `rabbitmq.queues_error` / `rabbitmq.cluster_partition` 等集群级 Field)**保持原样**进入签名。这些集群级 Field 的告警身份本就稳定,无需折叠。

## Risks / Trade-offs

- **[风险] 同一 vhost 内问题转移期间运维感知滞后**(例: T0 queueX 无消费者→发送; T1 queueX 恢复但 queueY 无消费者→签名相同→不发) → **缓解**: 冷却窗口结束(2 小时)后会发送一次反映最新清单的邮件;对持续漂移场景,这是设计上接受的代价(与无脑刷屏相比更可取)。
- **[风险] 折叠规则误命中其他 `rabbitmq.*` Field** → **缓解**: 正则锚定 `^rabbitmq\.<vhost>\.<queue>\.(no_consumer|backlog)$` 三段式且后缀闭集,新增测试覆盖 `rabbitmq.error`、`rabbitmq.node.<n>.mem_alarm` 等不应被命中的 Field。
- **[风险] state.json 中已有的"老签名"在升级后第一次跑会失配,触发一次额外发送** → **缓解**: 一次性影响,可接受;运维若敏感可手动清空 state.json,但无操作也不会进入异常分支。

## Migration Plan

无数据迁移。版本升级后第一次巡检若已有持续告警,会因签名值变化触发一次重发,之后行为收敛。无需回滚脚本——回滚即恢复旧 `signature.go`。
