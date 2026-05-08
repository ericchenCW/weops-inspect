## MODIFIED Requirements

### Requirement: 邮件发送

系统 SHALL 通过 SMTP（支持 TLS / STARTTLS / 明文 AUTH PLAIN）发送邮件，HTML 巡检
报告嵌入正文同时作为附件携带。发送阶段 MUST 受超时保护，且失败不得改变巡检主流程
退出码。告警邮件正文 SHALL 在每条 Warn 明细行展示触发该告警的阈值或期望值
（当该规则存在单一阈值时）。

#### Scenario: 告警邮件结构
- **WHEN** 发送告警邮件
- **THEN** 邮件主题 SHALL 形如 `[WeOps 巡检告警] {warn}/{total} 项异常`
- **AND** 正文 SHALL 包含巡检时间戳、Summary 数字（含 Unknown 计数）、所有 Warn 项的
  host / field / value
- **AND** 对存在单一阈值或期望值的 Warn 项，正文 SHALL 在该行追加阈值描述
  （如 `(阈值 ≥ 95%)`、`(阈值 期望 active)`）
- **AND** 对无单一阈值的规则项（如 `load_average`、`mysql_master.read_only`、`redis.role`、
  `rabbitmq.<vhost>.<queue>.no_consumer`），正文 MUST NOT 追加阈值描述
- **AND** 正文 MUST 不展示 Unknown 或 Notice 项的明细
- **AND** 附件 SHALL 包含本次生成的 `weops_inspection.html`

#### Scenario: 阈值不影响告警签名
- **WHEN** 两次巡检 Warn 集合的 host + field 完全一致，但任一项的阈值描述发生变化
  （例如 CPU 阈值从 `≥ 95%` 调整为 `≥ 90%`）
- **THEN** 两次签名 MUST 相同，冷却窗口与抑制行为 MUST 不受阈值变更影响

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
