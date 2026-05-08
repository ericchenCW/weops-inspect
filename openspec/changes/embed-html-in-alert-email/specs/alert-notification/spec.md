## MODIFIED Requirements

### Requirement: 邮件发送

系统 SHALL 通过 SMTP（支持 TLS / STARTTLS / 明文 AUTH PLAIN）发送邮件，正文为
`multipart/alternative` 结构，同时包含纯文本与 HTML 两个版本；HTML 巡检报告同时
作为附件携带。发送阶段 MUST 受超时保护，且失败不得改变巡检主流程退出码。

#### Scenario: 告警邮件结构
- **WHEN** 发送告警邮件
- **THEN** 邮件主题 SHALL 形如 `[WeOps 巡检告警] {warn}/{total} 项异常`
- **AND** 纯文本正文 SHALL 包含巡检时间戳、Summary 数字、所有 Warn 项的 host/field/value
- **AND** HTML 正文 SHALL 嵌入本次 `weops_inspection.html` 的 `<body>` 内部内容
- **AND** 附件 SHALL 包含本次生成的 `weops_inspection.html`

#### Scenario: 恢复邮件结构
- **WHEN** 发送恢复邮件
- **THEN** 邮件主题 SHALL 形如 `[WeOps 巡检恢复] 全部正常`
- **AND** 纯文本正文 SHALL 说明已从告警状态恢复
- **AND** HTML 正文 SHALL 嵌入本次 `weops_inspection.html` 的 `<body>` 内部内容
- **AND** 附件 SHALL 仍包含本次 `weops_inspection.html`

#### Scenario: HTML 报告读取或解析失败
- **WHEN** 读取 `weops_inspection.html` 出错，或文件中找不到 `<body>` 标签
- **THEN** 系统 SHALL 在 stderr 打印 warning，退化为仅纯文本正文 + HTML 附件
- **AND** 仍 SHALL 完成本次发送，巡检退出码 MUST 保持为 0

#### Scenario: SMTP 超时
- **WHEN** SMTP 连接或发送超过 30 秒未完成
- **THEN** 系统 SHALL 中止发送、在 stderr 打印错误，且巡检退出码 MUST 保持为 0
  （即不影响主流程）

#### Scenario: SMTP 鉴权失败
- **WHEN** SMTP 服务器拒绝凭据
- **THEN** 系统 SHALL 在 stderr 打印错误，state.json MUST 不被更新，下次运行
  按旧基线判定
