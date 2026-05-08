## Why

`weops-inspect` 目前是一次性 CLI，跑完只把报告落盘到本地，运维需要主动去看。期望以
`*/5 * * * *` 的节奏由 crontab 周期触发，并在出现告警时主动把报告通过邮件推到运维
邮箱，做到"无人值守 + 异常即知"。

## What Changes

- 新增"告警邮件通知"能力：巡检结束后根据 `Summary.Warn` 与历史状态决定是否发邮件，
  发送时把 HTML 报告作为附件、正文给出 Summary 与 Warn 列表。
- 新增配置文件入口 `~/.config/weops/config.json`（路径可由 `WEOPS_CONFIG` 环境变量
  覆盖），承载 SMTP 凭据、收件人、抑制窗口等参数。配置缺失或 `enabled:false` 时
  通知静默跳过，不影响巡检退出码与既有 `BK_*` 环境变量配置契约。
- 新增通知状态文件 `~/.config/weops/state.json`，记录上次发送时间、上次告警签名、
  上次状态（alert/ok），用于 2 小时去重窗口与恢复通知判定。
- 引入第三方依赖 `github.com/wneessen/go-mail` 处理 SMTP/TLS/MIME 附件。
- README 新增"crontab 周期巡检 + 邮件通知"使用章节，给出示例 cron 行；工具本身
  **不**自动安装/卸载 cron。

## Capabilities

### New Capabilities

- `alert-notification`：巡检结束后基于 Summary 与历史状态判定是否对外通知，提供
  邮件通道、签名去重、冷却窗口与恢复通知能力。命名留出未来扩展 webhook/IM 通道
  的空间。

### Modified Capabilities

（无：本次新增能力与既有 capabilities 正交，不修改 host/service/component 等
采集与检查规则。）

## Impact

- **代码**：新增 `notify/` 包（config / state / signature / trigger / email）；
  `main.go` 末尾追加通知调用；`go.mod`/`go.sum` 引入 `go-mail`。
- **运行时依赖**：新增可选的出站 SMTP 连接；不影响巡检主流程的退出码。
- **文件系统**：在用户家目录读写 `~/.config/weops/{config,state}.json`。
- **文档**：README 增加配置示例与 cron 部署说明。
- **既有契约**：HTML/JSON 报告 schema 不变；`BK_*` 与 `INSPECT_*` 环境变量行为
  不变。
