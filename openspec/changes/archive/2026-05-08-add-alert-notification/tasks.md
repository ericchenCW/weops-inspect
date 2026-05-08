## 1. 依赖与脚手架

- [x] 1.1 在 `go.mod` 引入 `github.com/wneessen/go-mail`（最新稳定版）并 `go mod tidy`
- [x] 1.2 创建 `notify/` 目录骨架：`config.go` / `state.go` / `signature.go` / `trigger.go` / `email.go` / `notify.go`（实现时额外拆出 `alerts.go` 用于把 report 中的 Warn 项转成带 host 上下文的结构）

## 2. 配置加载（notify/config.go）

- [x] 2.1 定义 `Config` 结构：`Email{Enabled, SMTPHost, SMTPPort, UseTLS, Username, Password, From, To[]}` + `Trigger{MinIntervalMinutes, SendRecovery}`
- [x] 2.2 实现 `Load() (*Config, error)`：解析 `WEOPS_CONFIG` 或回退到 `~/.config/weops/config.json`
- [x] 2.3 文件不存在时返回 `(nil, nil)`，调用方据此跳过通知
- [x] 2.4 `enabled=true` 时校验必填字段（smtp_host/smtp_port/from/to），缺失时返回 warning 错误而非 fatal（通过 `Validate()` 在 `Process` 中调用，失败仅打印 stderr）
- [x] 2.5 加载时检查文件 mode，非 `0600` 在 stderr 打印 warning

## 3. 告警签名（notify/signature.go）

- [x] 3.1 实现 `Signature(items []AlertItem) string`：对 `host|field` 排序后 SHA-256 取 hex（输入由 `ExtractAlerts` 从 report 中筛 Warn 得来）
- [x] 3.2 Warn 列表为空时返回空字符串
- [x] 3.3 编写单测覆盖：稳定性、集合变化、空集合、顺序无关四种场景

## 4. 状态文件（notify/state.go）

- [x] 4.1 定义 `State{LastSentAt time.Time, LastSignature string, LastStatus string}`，`LastStatus` 取值 `"alert"` / `"ok"` / `""`
- [x] 4.2 实现 `LoadState(path) *State`：文件不存在或解析失败均返回零值并打印 warning
- [x] 4.3 实现 `SaveState(path, *State) error`：原子写入（temp file + rename，0600 权限）

## 5. 决策函数（notify/trigger.go）

- [x] 5.1 定义 `Action` 枚举：`ActionNone` / `ActionSendAlert` / `ActionSendRecovery`
- [x] 5.2 实现纯函数 `Decide(now time.Time, prev *State, warnCount int, sigNow string, cooldown time.Duration) Action`
- [x] 5.3 实现决策矩阵中的全部规则（首次告警、持续告警冷却内/外、签名变化、恢复、持续正常）
- [x] 5.4 编写单测覆盖矩阵全部分支，含冷启动 prev 为 nil 的场景

## 6. 邮件发送（notify/email.go）

- [x] 6.1 实现 `BuildAlertSubject(summary)` / `BuildRecoverySubject(summary)`
- [x] 6.2 实现 `BuildAlertBody(report, items)`：纯文本，含时间戳、Summary、Warn 明细列表
- [x] 6.3 实现 `BuildRecoveryBody(report)`
- [x] 6.4 实现 `Send(cfg, subject, body, attachmentPath)`：使用 `go-mail`，30s 超时，TLS（隐式 SSL）/STARTTLS 按 `UseTLS` 选择，含 SMTP AUTH PLAIN

## 7. 入口编排（notify/notify.go）

- [x] 7.1 实现 `Process(cfg *Config, report *model.InspectReport, htmlPath string)`（与 spec 比简化掉 `allChecks` 参数，直接从 report 提取告警，避免参数冗余）
- [x] 7.2 流程：加载 state → 提取 alerts → 计算签名 → `Decide` → 按 Action 构造主题/正文 → 发送 → 成功后保存 state
- [x] 7.3 任何错误仅 stderr 输出 warning，函数无返回值（保护巡检退出码）

## 8. main.go 接入

- [x] 8.1 修改 `output.Write(...)` 签名为 `(htmlPath string, err error)`，返回 HTML 绝对路径
- [x] 8.2 在 `main.go` 末尾调用 `notify.Load()`；非 nil 时调用 `notify.Process(...)`
- [x] 8.3 `report` 已包含 host 与 services 完整数据，`notify.ExtractAlerts(report)` 直接从中提取 Warn，无需额外传 `allChecks`

## 9. 文档

- [x] 9.1 在 README 新增 "邮件告警通知" 章节，给出 `config.json` 完整示例（项目原本无 README，本次新增）
- [x] 9.2 在 README 新增 "Crontab 周期巡检部署" 章节，给出 cron 行示例（覆盖 `bk.env` source、`WEOPS_CONFIG`、stderr 重定向）
- [x] 9.3 文档强调 `chmod 600 ~/.config/weops/config.json`

## 10. 验证

- [x] 10.1 `go build ./...` 通过
- [x] 10.2 `go test ./notify/...` 全部通过（签名 4 例 + 决策矩阵 7 例）
- [ ] 10.3 手工冒烟：构造模拟 report，配置指向 MailHog/maildev，验证告警/恢复/抑制三条路径（**待用户在有 SMTP 的环境下执行**）
- [x] 10.4 验证 SMTP 不可达时巡检退出码仍为 0：`notify.Process` 无返回值，所有错误路径仅 `fmt.Fprintf(os.Stderr,...)`；`main.go` 在调用后直接退出，不会因通知失败而 `os.Exit(1)`
