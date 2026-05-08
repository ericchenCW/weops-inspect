## Context

WeOps 巡检的"红字 = Summary 计数 = 邮件告警"三处判定彼此独立：

```
collector ──► raw data ──┬──► HTML 模板（内联 if gt/eq 着红）
                         ├──► checker.* (仅覆盖 host/service/replication，无 ES/Redis/Mongo/RMQ/BKDeps)
                         │      └──► allChecks ──► Summary
                         └──► (无)
                                ▲
                                │
notify.ExtractAlerts ───────────┘
  （对 service/replication 又自己判一次，与 checker 不一致）
```

两类已暴露问题：

1. `CheckService` 跳过空 Status，但 `ExtractAlerts` 把空状态判 Warn → 控制台 `0 告警`
   仍发邮件（job-analysis 案例）。
2. ES / Redis / MongoDB / RabbitMQ / bkmonitorv3 依赖完全没有 Checker，红字仅在模板里，
   不进 Summary 不进邮件（RabbitMQ 360k 积压不告警案例）。

阈值散落两处：`Thresholds` 结构体（被 collector 与 checker 部分使用）、`*.tmpl` 模板里
硬编码的 `gt 85` / `gt 95` / `gt 1000` 等常量。

本次改造目标是把判定收敛为单一事实源（`CheckResult.Status`），HTML 着色与邮件告警都
读它。

## Goals / Non-Goals

**Goals:**

- 状态枚举从二元 (OK/Warn) 扩展到四元 (OK/Warn/Unknown/Notice)，明确区分"采集不到"与"着色但不告警"
- 所有缺失的 Checker（ES/Redis/Mongo/RabbitMQ/BKDeps）一次性补齐
- HTML 模板的着色规则下沉到 `Status` 字段，删除模板里的所有内联阈值/常量比较
- 模板里硬编码的阈值搬入 `Thresholds`，统一通过 env 配置
- `notify.ExtractAlerts` 改为只读 `CheckResult.Status == Warn`

**Non-Goals:**

- 不改告警通道（仍是 SMTP）、不增加 webhook / IM 推送
- 不引入分级（critical/major/minor）：本轮只在"告警"内做粒度切分（Warn vs Notice）
- 不重构 collector 的字段命名与可达性表示（`Reachable bool` / `Status "unreachable"` 仍混用）
- 不实现 ES 索引级 / RMQ exchange 级深度检查
- 不动 `RabbitMQNoConsumerVHostBlacklist` 黑名单语义（按用户决定保持现状）

## Decisions

### D1：四元状态 OK / Warn / Unknown / Notice

**为什么不只用三元：** 单"Warn"无法表达"我希望红字提醒人，但不要发邮件 / 不计入异常率"
这种诉求。例如 ES RAM 95%、Redis CeleryQueue 超阈值——用户明确不希望它们发邮件。
如果把这些归为 OK 又显得报告"全绿很可疑"。

**为什么不直接复用 `Notice`-only：** Unknown 与 Notice 语义不同。Unknown 是"应该有数据
没采到"（job-analysis 空状态、Mongo 节点 Health 字段缺失）。Notice 是"采到了，超阈值，
但该阈值不重要到值得发邮件"。两者在 Summary 里也应分别可见。

**取舍：**

| 状态 | Summary.Total | Summary.桶 | 进邮件 | HTML 颜色 |
|------|---------------|-----------|--------|-----------|
| OK | + | OK+ | 否 | 绿 |
| Warn | + | Warn+ | 是 | 红 |
| Unknown | + | Unknown+ | 否 | 灰 (status-na) |
| Notice | **不计** | 无 | 否 | 红 |

Notice 不计 Total 是关键决策——确保"148 项检查 0 告警 Notice 5 项"这种输出在视觉上
不会让人误以为 Notice 是异常率分子。

### D2：模板单一事实源 = `Status` 字段

**为什么给渲染结构都加 Status 字段而不是把着色统一移到 checker 输出：**

模板里一行 `<td>` 显示一个原始字段（HeapPercent、CeleryQueue、Health 等），它在 `range`
里渲染。如果着色由"checker 输出 CheckResult 列表"决定，模板就要在 range 里反向查找
对应的 CheckResult，代码丑且低效。

直接给每个渲染结构加 `Status string` 字段最直接：collector 留空，checker 在产出 CheckResult
时同时回填到原结构上。模板侧就一行：

```go-template
<td class="{{statusClass .Status}}">{{.HeapPercent}}%</td>
```

`statusClass` 是一个 template func，把 `"warn" / "notice"` 都映射到 `status-warn`，
`"unknown"` 映射到 `status-na`，`"ok"` 映射到 `status-ok`，空映射到无类。

**Alternative 1（拒绝）：** checker 直接产出"模板专用结构"。模板和 collector 的产出结构
彻底分离，开销大、字段重复维护。

**Alternative 2（拒绝）：** 模板继续内联 + checker 单独写一遍。本次重构的初衷就是消灭这种
双源真相，自我打脸。

### D3：阈值搬迁的范围 = 全部，无论本轮是否告警

用户明确"告警与否都搬"。因此：

- `ESHeapPercent` (默认 85)
- `ESRAMPercent` (默认 95)
- `ESUnassignedShards` (默认 0)
- `RedisCeleryQueue` (默认 1000)
- `RedisMonitorQueue` (默认 10000)
- `ServiceContainersExited` (默认 0)

阈值由 checker 消费产出 Notice/Warn。本轮这些项都归 Notice，未来若想升级为 Warn，
只需改 Checker 一处。

### D4：collector 与 checker 的职责切分

| 职责 | 归属 |
|------|------|
| HTTP/SQL/RPC 采集 | collector |
| 错误归类（ErrNetwork/ErrAuth/...） | collector |
| 黑名单筛选（RMQ vhost、`SkipNoConsumerQueues`） | collector |
| "积压队列切片"产出（ExceedingQueues / NoConsumerQueues） | collector |
| 阈值比较产出 Status | checker |
| 错误字段（如 `cluster.Error 非空`）转 CheckResult | checker |
| Status 字段回填到渲染结构 | checker |

**为什么 RMQ 黑名单留在 collector：** 当前 `noConsumerVHostBL` 已经在 collector 实现，
它影响的是"切片本身有没有这条队列"。把它挪到 checker 等于在 checker 里再过一次黑名单，
增加一处判断点。保留现状，签名上更紧凑。

### D5：CheckResult Field 命名约定

各 Check 函数产出的 `CheckResult.Field` 采用统一前缀：

```
es.{ip}.{metric}                 # 例: es.10.10.26.235.heap, es.10.10.26.236.reachability
es.cluster.{metric}              # 例: es.cluster.error, es.cluster.unassigned_shards
redis.{ip}.{metric}              # 例: redis.10.10.26.235.celery_queue
redis_sentinel.{ip}.{metric}     # 例: redis_sentinel.10.10.26.235.reachable
redis_sentinel.{metric}          # 例: redis_sentinel.master_reachable, redis_sentinel.status
mongo.{name}.{metric}            # 例: mongo.10.10.26.236:27017.health
mongo.replica.{metric}           # 例: mongo.replica.error
rabbitmq.{metric}                # 例: rabbitmq.error, rabbitmq.cluster_partition
rabbitmq.node.{node}.{metric}
rabbitmq.{vhost}.{queue}.{metric}  # 例: rabbitmq.prod_bk_monitorv3.celery.backlog
bkdeps.{item}.status              # 例: bkdeps.mysql.status
service.{module}/{submodule}.{metric}  # 沿用现有
docker.{ip}.exited
```

签名稳定性依赖这个命名（host + field 拼接），新增 metric 时务必避免与历史 field 冲突。

### D6：BREAKING 行为：升级当次会触发告警

升级当次必然出现：

- 之前漏报的 RabbitMQ / ES / MongoDB / Redis 项进入 Warn
- 签名相对升级前完全变化
- ExtractAlerts 命中"签名变化→立即发送"分支

这是**预期行为**，不算回归。文档（README）需在升级章节提示运维。

## Risks / Trade-offs

- [告警风暴] 升级首跑会一次性把历史漏报项全部以 Warn 形式入邮 →
  Mitigation: README 升级章节加提示；建议先 `WEOPS_NOTIFY_ENABLED=false` 跑一次确认 Warn 列表
  再开启通知；冷却窗口未变（2h），后续不会反复发。

- [Notice 噪音] Notice 不进邮件但仍渲染红字，HTML 看起来"异常很多" →
  Mitigation: 报告头 summary card 区分 Warn / Notice / Unknown 三个数字（视觉上提示读者
  Notice 非紧急）。

- [模板一致性回归] 后续新增字段如果继续内联 `{{if gt ...}}` 会破坏单一事实源 →
  Mitigation: 加一个 grep 形式的 lint 检查（CI 跑 `! grep -n "{{if gt \\." render/templates/*.tmpl`，
  hits 视为失败）。

- [Status 字段污染 JSON 输出] `weops_inspection.json` 里渲染结构会多出 Status 字段 →
  Mitigation: 接受。Status 字段命名清晰，对外没有破坏现有解析（旧 key 不变）。如果担心可加
  `omitempty`。

- [字段名冲突] 多个采集结构上同时新增 `Status string`，可能与现有 Sentinel.Status / Redis.RoleConsistencyStatus 等同名字段混淆 →
  Mitigation: 现有的 `RedisSentinelStatus.Status` 已经是字符串值（"ok"/"warn"/"critical"），含义
  与新引入的渲染状态一致，可以**复用**而不新增字段；其他结构（ESNode、RabbitMQQueue 等）现在没有
  Status 字段，新增不会冲突。在 design.md 实现指引中要逐结构核对。

## Migration Plan

1. **代码层落地（按 tasks.md 顺序）**：先扩枚举与 Summary，再补 Checker，再改模板。
2. **本地验证**：跑一次完整巡检，比对升级前后 HTML 视觉一致（除新增 Notice 着色外）。
3. **Staging 灰度**：在 staging 环境关闭通知（`WEOPS_NOTIFY_ENABLED=false`）跑 1\~2 次，
   人工 review 邮件草稿（dryrun）。
4. **生产升级**：开启通知。第一次会发一封大邮件（包含此前漏报的所有 Warn 项），
   运维知情即可。
5. **回滚**：版本回退即可，state.json 兼容（多余字段被旧版本忽略）。

## Open Questions

- **Q1**：模板 `statusClass` template func 是否要支持组合？例如某单元格既有 "warn" 又有
  "stale"（数据过期但已告警）。当前回答：不支持，单一 Status 即可，避免复杂化。
- **Q2**：邮件正文是否要展示 Unknown 计数？当前 spec 说"展示 Unknown 计数但不展示明细"。
  实现时若发现明细对值班同学很有用（例如知道哪个子服务采集不到），可在二期补展示。
