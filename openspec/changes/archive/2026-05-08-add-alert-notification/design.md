## Context

`weops-inspect` 现有形态是一次性 CLI：`main.go` 编排"采集 → 检查 → 写报告"三阶段，
最终调用 `output.Write()` 把 `weops_inspection.html` / `.json` 写到 `-o` 指定目录，
进程退出。配置全部走 `BK_*` / `INSPECT_*` 环境变量（见 [config.go:109](config/config.go:109)），
无配置文件。`checker.Summarize()` 产出 `Summary{Total, OK, Warn}`，没有严重度分级。

本次需要在不破坏既有契约的前提下，把一次性 CLI 改造成"crontab 周期触发 + 异常邮件
通知"的工作模式。crontab 部署、SMTP 凭据、收件人、抑制窗口与恢复策略是新引入的
关注点，需要一个**独立于巡检主流程**的通知子系统。

## Goals / Non-Goals

**Goals:**

- 巡检结束后自动判定是否需要通知，无需人工干预。
- 邮件正文给出 Summary + Warn 列表，HTML 报告作为附件。
- 同一组告警在 2 小时窗口内只发一次邮件，避免每 5 分钟一封刷屏。
- 当告警集合发生变化（新增/消失某项）时立即重发，不被冷却窗口压制。
- 当上次发过告警邮件、本次恢复正常时，发送一封"恢复通知"。
- 通知失败不影响巡检退出码与报告产出。
- 配置缺失/未启用时静默跳过通知，纯手跑场景零感知。

**Non-Goals:**

- 不自动安装/卸载 crontab，部署由 README 文档指导用户手工执行。
- 不实现 webhook/钉钉/企微/Slack 等其他通知通道（只为命名留出空间）。
- 不引入告警严重度分级（沿用 checker 现有的 OK/Warn 两态）。
- 不做密码加密或外部 secret 引用，`config.json` 直接放明文密码。
- 不修改既有 `BK_*` / `INSPECT_*` 环境变量契约，也不修改 HTML/JSON 报告 schema。

## Decisions

### 1. 通知子系统作为独立 `notify/` 包，由 `main.go` 末尾"挂载"

**选择**：新增 `notify/` 包，导出单一入口 `notify.Process(cfg, report, htmlPath)`，
内部完成"加载状态 → 计算签名 → 决策 → 发送 → 落盘状态"。`main.go` 在 `output.Write()`
之后追加 ~5 行调用，不改主流程结构。

**为什么不直接写在 main.go**：保持 `main.go` 的"三阶段编排"骨架清晰，通知是横切
关注点，独立成包便于单测决策矩阵（trigger 函数纯函数化）。

**为什么不放在 `output/`**：`output/` 的职责是"把报告序列化到磁盘"，邮件发送是
副作用与 I/O 都更重的外部动作，混在一起会让 `output.Write()` 失去单一职责。

### 2. 配置入口走 JSON 文件，与 `BK_*` 环境变量并存

**选择**：新增 `~/.config/weops/config.json`（路径可由 `WEOPS_CONFIG` 环境变量
覆盖），仅承载**通知相关**配置；`BK_*` / `INSPECT_*` 仍是巡检本体的真相源。

**为什么不把所有配置统一迁到 JSON**：`BK_*` 是与蓝鲸 `bk.env` 对齐的契约（见
[project.md](openspec/project.md) "配置全部走环境变量"约束），改造成本与风险都
远超本次需求；通知配置与巡检拓扑配置在语义上正交，分开放更干净。

**为什么不把通知配置也走环境变量**：SMTP 凭据塞进 crontab 的环境变量行不安全且
难维护；JSON 文件 + 0600 权限是更常规的运维形态。

**配置缺失策略**：文件不存在 → 静默跳过（保留纯手跑无副作用）；存在但 `enabled:
false` → 跳过；存在且必填项缺失 → stderr 打印 warning 但**不改变巡检退出码**。

### 3. 状态文件用于去重与恢复判定

**选择**：`~/.config/weops/state.json`，schema：

```json
{
  "last_sent_at":   "2026-05-08T10:30:00+08:00",
  "last_signature": "sha256...",
  "last_status":    "alert" | "ok" | ""
}
```

**告警签名**：对所有 `Status=Warn` 的 `CheckResult` 取 `host|field` 字段，排序后
求 SHA-256。值（如 CPU 百分比）**不进签名**——只关心"哪些项在告警"，避免数值的
微小波动造成签名抖动重发。

**决策矩阵**（输入：本次 `Summary.Warn`、本次签名 `s_now`、state 中的
`last_signature` `s_last` / `last_status` / `last_sent_at`）：

| 上次状态  | 本次 Warn | 条件                          | 动作              |
|-----------|-----------|-------------------------------|-------------------|
| `""`/`ok` | `=0`      | —                             | 不发              |
| `""`/`ok` | `>0`      | —                             | 发告警邮件        |
| `alert`   | `=0`      | —                             | 发恢复邮件        |
| `alert`   | `>0`      | `s_now != s_last`             | 发告警邮件（变化）|
| `alert`   | `>0`      | `s_now == s_last` 且 <2h      | 抑制              |
| `alert`   | `>0`      | `s_now == s_last` 且 ≥2h      | 发告警邮件（重发）|

**冷启动**：state 文件不存在视为 `last_status=""`，等价于"上次 ok"，因此第一次
跑就告警会立即发，第一次跑无告警则不发恢复（避免开机即发一封莫名其妙的恢复邮件）。

**state 写入时机**：仅在**发送成功**后更新 state。发送失败保留旧 state，下次仍按
旧基线判定，避免一次 SMTP 抖动让告警从此被错误抑制。

### 4. 引入 `github.com/wneessen/go-mail` 而非 `net/smtp` 手搓

**选择**：在 `go.mod` 增加 `go-mail` 依赖。

**为什么**：标准库 `net/smtp` 仅覆盖 PLAIN/CRAM-MD5 鉴权，TLS、STARTTLS、MIME
附件、HTML 正文都要自己拼接，约 150 行模板代码且容易踩坑（CRLF、Content-Transfer
-Encoding）。`go-mail` API 干净、活跃维护、零额外架构成本。项目当前依赖很少，
为这一项引入第三方包是值得的取舍。

**为什么不用 `gopkg.in/gomail.v2`**：长期未更新，Go 1.20+ 上有 TLS 配置过期问题。

### 5. 邮件正文：纯文本 + HTML 附件

**选择**：邮件 body 为**纯文本**，结构如下；HTML 报告作为附件。

```
[巡检告警] 2026-05-08 10:30:00
Summary: 158 项检查, 152 正常, 6 告警

告警明细：
  10.0.0.1 / cpu_usage           = 87%
  10.0.0.1 / disk_usage:/data    = 92%
  10.0.0.3 / mysql_repl_lag      = 320s
  ...

报告附件：weops_inspection.html
```

**为什么不直接把 HTML 当邮件正文**：现有模板包含外链 CSS/JS 资源与较重 DOM，
作为邮件 body 兼容性差（Outlook/Gmail 渲染不可控）；纯文本一眼可读，附件留给
真要看细节的人。

**主题模板**：
- 告警：`[WeOps 巡检告警] {warn}/{total} 项异常`
- 恢复：`[WeOps 巡检恢复] 全部正常`

### 6. 报告文件路径回传

**选择**：修改 `output.Write()` 签名为 `Write(...) (htmlPath string, err error)`，
把 HTML 文件绝对路径返回给 `main.go`，再传给 `notify.Process()`。

**为什么不让 notify 自己拼路径**：`output.Write()` 已经掌握 `outputDir + 文件名`
的真相，重新拼一遍是隐式契约重复，未来改文件名会两边漏改。

## Risks / Trade-offs

- **明文密码落盘**：`config.json` 含 SMTP 密码，泄漏风险靠文件权限兜底
  → 加载时检查权限，非 0600 在 stderr 打印 warning（不阻断），README 明确告知
  `chmod 600`。
- **crontab 环境变量裸跑**：crontab 默认不 source `bk.env`
  → README 示例 cron 行明确写出 `bash -lc 'source /path/to/bk.env && weops-inspect ...'`
  或在 crontab 顶部 `BASH_ENV=...` 的两种写法。
- **state.json 损坏导致后续告警全部抑制**：解析失败时按"冷启动"处理（视为
  `last_status=""`），并打印 warning，避免坏文件长期屏蔽告警。
- **SMTP 阻塞拖慢巡检**：通知阶段加 30s 超时；超时/失败仅 stderr 警告，不影响
  巡检退出码（cron 兼容）。
- **签名仅取 host+field 可能漏掉"同 field 数值跨阈值反复抖动"的情况**：这是有意
  取舍——把数值放进签名会让正常波动持续触发邮件，违背"2 小时去重"的初衷。如未来
  需要更细粒度，可在 trigger 层加可选的"include_value_in_signature"开关。
- **多用户/多实例运行同一台机器**：state.json 路径基于 `$HOME`，不同用户互不干扰；
  同一用户起多个实例并发跑会有 state 竞态，但这违背单机周期巡检的部署假设，
  本次不处理。
