## MODIFIED Requirements

### Requirement: 告警签名计算

系统 SHALL 基于本次巡检的 Warn 检查项计算稳定签名，用于跨次运行识别"告警集合是否
变化"。签名 MUST 仅由 `Status == Warn` 的检查项标识构成（host + field），不包含具体
数值，**不**包含 `Unknown` 或 `Notice` 项。

#### Scenario: 签名稳定性
- **WHEN** 两次巡检产生完全相同的 Warn 检查项集合（host + field 一致），但具体
  数值（如 CPU 76% vs 78%）不同
- **THEN** 两次计算得到的签名 MUST 相同

#### Scenario: 告警集合变化
- **WHEN** 本次新增或减少了任一 Warn 项（host 或 field 维度）
- **THEN** 本次签名 MUST 与上次签名不同

#### Scenario: Unknown 与 Notice 不影响签名
- **WHEN** 本次 Warn 集合不变，但 Unknown 项或 Notice 项有增减
- **THEN** 签名 MUST 与上次相同

#### Scenario: 无告警时的签名
- **WHEN** 本次 Warn 列表为空
- **THEN** 签名 MUST 为空字符串或固定哨兵值，与"有告警"场景可区分

### Requirement: 通知决策

系统 SHALL 综合本次告警状态、上次状态（state.json）、签名是否变化、距上次发送的
时间间隔，决定本次执行三种动作之一：发送告警邮件、发送恢复邮件、抑制不发。冷却
窗口 MUST 为 2 小时。Unknown 与 Notice 项 MUST 不触发任何邮件动作。

#### Scenario: 首次告警（冷启动）
- **WHEN** state.json 不存在或 last_status 为空，且本次 Warn>0
- **THEN** 系统 SHALL 发送告警邮件

#### Scenario: 仅有 Unknown 项
- **WHEN** 本次 Warn==0 且 Unknown>0
- **THEN** 系统 SHALL 视为"无告警"，按"持续正常"或"告警恢复"分支处理

#### Scenario: 持续告警且签名相同、未超冷却窗口
- **WHEN** 上次状态为 alert，本次 Warn>0 且签名与上次相同，距上次发送 < 2 小时
- **THEN** 系统 SHALL 抑制本次通知，不发送邮件

#### Scenario: 持续告警但签名变化
- **WHEN** 上次状态为 alert，本次 Warn>0 但签名与上次不同（告警集合变化）
- **THEN** 系统 SHALL 立即发送告警邮件，不受冷却窗口限制

#### Scenario: 持续告警且签名相同但已超冷却窗口
- **WHEN** 上次状态为 alert，本次 Warn>0 且签名相同，距上次发送 ≥ 2 小时
- **THEN** 系统 SHALL 重新发送告警邮件

#### Scenario: 告警恢复
- **WHEN** 上次状态为 alert，本次 Warn=0
- **THEN** 系统 SHALL 发送恢复通知邮件

#### Scenario: 持续正常
- **WHEN** 上次状态为 ok 或空，本次 Warn=0
- **THEN** 系统 SHALL 不发送任何邮件

### Requirement: 邮件发送

系统 SHALL 通过 SMTP（支持 TLS / STARTTLS / 明文 AUTH PLAIN）发送邮件，HTML 巡检
报告嵌入正文同时作为附件携带。发送阶段 MUST 受超时保护，且失败不得改变巡检主流程
退出码。

#### Scenario: 告警邮件结构
- **WHEN** 发送告警邮件
- **THEN** 邮件主题 SHALL 形如 `[WeOps 巡检告警] {warn}/{total} 项异常`
- **AND** 正文 SHALL 包含巡检时间戳、Summary 数字（含 Unknown 计数）、所有 Warn 项的 host/field/value
- **AND** 正文 MUST 不展示 Unknown 或 Notice 项的明细
- **AND** 附件 SHALL 包含本次生成的 `weops_inspection.html`

#### Scenario: 恢复邮件结构
- **WHEN** 发送恢复邮件
- **THEN** 邮件主题 SHALL 形如 `[WeOps 巡检恢复] 全部正常`
- **AND** 正文 SHALL 说明已从告警状态恢复
- **AND** 附件 SHALL 仍包含本次 `weops_inspection.html`

#### Scenario: SMTP 超时
- **WHEN** SMTP 连接或发送超过 30 秒未完成
- **THEN** 系统 SHALL 中止发送、在 stderr 打印错误，且巡检退出码 MUST 保持为 0

#### Scenario: SMTP 鉴权失败
- **WHEN** SMTP 服务器拒绝凭据
- **THEN** 系统 SHALL 在 stderr 打印错误，state.json MUST 不被更新，下次运行
  按旧基线判定
