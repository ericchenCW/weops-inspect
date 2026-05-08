## Context

告警邮件正文目前只有"host / field / value"三列，触发该告警的阈值/期望值对收件人不可见。
阈值知识唯一存在于 `checker/rules.go` 的判定瞬间——比较完即丢，不会进入 `model.CheckResult`，
也不会随 `notify.AlertItem` 流动到邮件渲染层。

## Goals / Non-Goals

**Goals**
- 让告警邮件每一行能自解释"为什么被判异常"
- 数据流改动可逆、对 JSON / HTML 现有消费者零破坏
- 阈值的描述靠近"判定真相源"（rules.go），避免在 notify 层做反查

**Non-Goals**
- 不改变 HTML 附件模板（已由 [render](render) 模板自带颜色和对照，本期不重排）
- 不引入结构化 `(operator, value, unit)` 类型——本期只用人类可读字符串，未来若需 i18n 再升级
- 不改变阈值数值（那是 `tune-alert-thresholds` change 的事）
- 不展示 Unknown 项的"为什么 unknown"——本期仍只覆盖 Warn/Notice

## Decisions

### Decision 1: 阈值表达方式 — 自由文本字符串

把 `Threshold` 设计成 `string`，由 `checker/rules.go` 直接写入，例如：

| 检查项 | Threshold 字符串 |
|---|---|
| cpu_usage / mem_usage / disk_usage / inode_usage | `≥ 95%` |
| max_open_files | `< 65536` |
| selinux | `期望 Disabled` |
| firewalld | `期望 inactive` |
| chronyd / service.*.status | `期望 active` |
| service.*.healthz | `期望 ok` |
| mysql_slave.replication | `lag > 30s` |
| redis.link | `io > 30s` |
| rabbitmq.<vhost>.<queue>.backlog | `> 1000` |
| service.*.docker.exited | `> 0` |
| load_average / mysql_master.read_only / redis.role / rabbitmq.no_consumer | `""`（留空，规则已在 value 自描述）|

**Why string vs structured**: 结构化类型要新增 `model.Threshold{Op, Value, Unit}` 之类的
类型 + 序列化逻辑，本期没有第二个消费者（HTML 模板自带阈值，前端不存在），收益小于成本。
未来若要 i18n 或前端复用再升级为结构化。

**Why 含期望值（"期望 X"）也叫 Threshold**: 在用户视角"为什么这一项被判异常"是同一个问题；
强行区分 `Threshold` 和 `Expected` 两个字段会让 rules.go 多一层分支，邮件渲染也要分两列。
统一用 `Threshold string`，rules.go 选用合适措辞即可。

### Decision 2: 数据流路径 — model → notify 透传

```
checker/rules.go               model/types.go               notify/alerts.go            notify/email.go
   │                              │                            │                           │
   add(field,value,status,thr)    CheckResult{                 AlertItem{Host,Field,       BuildAlertBody:
   │                                Field,Value,Status,        Value, Threshold}           "  %-16s %-40s = %s  (%s)\n"
   ▼                                Threshold ← 新增           ↑ 由 ExtractAlerts 透传      ↑ Threshold 为空则不打括号
                                  }
```

**Why 不在 notify 层查表**: 阈值知识两处定义会随时间漂移；让阈值字符串与判定逻辑
同步在一处（rules.go）声明可避免漂移，配置变更（含 env 覆盖）也能自动反映到邮件里。

### Decision 3: 签名（signature）显式排除 Threshold

`notify/signature.go` 当前基于 `Field`（按规则归一化后）计算签名，本来就不读 Value 也
不读 Threshold。新增字段后**不**改动 signature 逻辑，仅在代码注释中显式标注"Threshold
故意不参与签名"，并在 spec 增加 Scenario 兜底。

**Why**: 如果 Threshold 进签名，未来调阈值（如 95% → 90%）会立刻刷掉所有抑制态，触发
全量重发——这违背"signature 标识告警**集合**"的语义。

### Decision 4: 渲染格式 — 行尾追加，可选

```
现状:    host_a    cpu_usage     = 96.50%
新版:    host_a    cpu_usage     = 96.50%  (阈值 ≥ 95%)
留空时:  host_a    load_average  = 5.2/4.1/3.8 (cores: 4)
```

阈值为空时**不**追加 `(阈值 )` 占位，避免视觉噪声。列对齐沿用现有 `%-16s %-40s` 格式。

## Risks / Trade-offs

- **Risk: rules.go 阈值字符串与 config 数值漂移**
  - 缓解：阈值字符串里直接引用 `thresholds.X` 数值（`fmt.Sprintf("≥ %.0f%%", thresholds.CPUUsage)`），
    避免硬编码。状态类期望值（"Disabled"/"inactive"/"active"）来自代码常量，由测试覆盖。
- **Risk: 邮件正文变长，弱网客户端折行难看**
  - 缓解：阈值串短（一般 < 12 字节），追加在行尾不破坏现有结构。
- **Risk: 测试样本要更新**
  - 影响：`notify/email_test.go`（如有）、`checker/rules_test.go` 的断言需要扩展。
    所有 testdata 都在 [docs/testdata](docs/testdata) 之外，改动可控。

## Migration Plan

无运行期迁移。代码层面：

1. 先在 `model.CheckResult` 加字段（向后兼容）
2. 在 `checker/rules.go` 逐项填充——可分多个 commit 按检查类目推进
3. 最后在 `notify` 层透传 + 渲染——一次性切换
4. 旧的 JSON 输出（如已 archive 的 `output/*.json`）不会回填 Threshold 字段；新一次巡检
   的输出新增字段，旧消费者忽略即可

## Open Questions

- HTML 内嵌邮件正文（`embed-html-in-alert-email` 已交付）天然带阈值（render 模板自带），
  本期是否需要把"纯文本与 HTML 阈值表达一致性"作为约束写进 spec？暂定**不**约束，因为
  HTML 模板的阈值列由模板自治，和纯文本只需"都展示"即可，不要求字面一致。
