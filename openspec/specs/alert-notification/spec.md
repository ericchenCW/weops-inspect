# alert-notification Specification

## Purpose

定义巡检结束后基于告警状态对外通知的能力。基于 Summary 与历史状态判定是否发送，
提供邮件通道、签名去重、冷却窗口与恢复通知支持。
## Requirements
### Requirement: 通知配置加载

系统 SHALL 从 `~/.config/weops/config.json` 加载通知配置；当环境变量 `WEOPS_CONFIG`
非空时，SHALL 改用其指定的路径。配置缺失或显式禁用时，巡检主流程必须不受影响。

#### Scenario: 配置文件不存在
- **WHEN** 巡检结束、目标路径下不存在配置文件
- **THEN** 系统 SHALL 跳过通知阶段，不打印错误，巡检退出码不变

#### Scenario: 配置文件存在但 enabled=false
- **WHEN** 配置文件 `email.enabled` 为 `false`
- **THEN** 系统 SHALL 跳过通知阶段，不发送任何邮件

#### Scenario: 通过环境变量覆盖配置路径
- **WHEN** 环境变量 `WEOPS_CONFIG=/etc/weops/cfg.json` 已设置且文件存在
- **THEN** 系统 SHALL 加载 `/etc/weops/cfg.json` 而非 `~/.config/weops/config.json`

#### Scenario: 配置缺少必填字段
- **WHEN** 配置文件存在且 `enabled=true`，但 `smtp_host` / `to` 等必填字段缺失
- **THEN** 系统 SHALL 在 stderr 打印 warning，跳过通知，**不**改变巡检退出码

#### Scenario: 配置文件权限过宽
- **WHEN** 配置文件权限不是 `0600`
- **THEN** 系统 SHALL 在 stderr 打印 warning 提醒收紧权限，但仍继续加载并发送

### Requirement: 告警签名计算

系统 SHALL 基于本次巡检的 Warn 检查项计算稳定签名,用于跨次运行识别"告警集合是否
变化"。签名 MUST 仅由 `Status == Warn` 的检查项标识构成(host + field),不包含具体
数值,**不**包含 `Unknown` 或 `Notice` 项。

为避免 RabbitMQ 队列名漂移(如 celery 派生的瞬态队列名)造成签名抖动绕过冷却窗口,
系统 MUST 在签名计算前对 RabbitMQ 队列级 Field 做归一化:

- 形如 `rabbitmq.<vhost>.<queue>.no_consumer` 的 Field MUST 折叠为
  `rabbitmq.<vhost>.no_consumer`。
- 形如 `rabbitmq.<vhost>.<queue>.backlog` 的 Field MUST 折叠为
  `rabbitmq.<vhost>.backlog`。
- 其他 `rabbitmq.*` Field(集群级,如 `rabbitmq.error`、`rabbitmq.cluster_partition`、
  `rabbitmq.node.<n>.mem_alarm` 等)以及非 RabbitMQ Field MUST 保持原样。
- 折叠后产生的重复键 MUST 去重,使"同一 vhost 下 N 个队列触发同类告警"对签名贡献
  恒为单一键。

#### Scenario: 签名稳定性
- **WHEN** 两次巡检产生完全相同的 Warn 检查项集合(host + field 一致),但具体
  数值(如 CPU 76% vs 78%)不同
- **THEN** 两次计算得到的签名 MUST 相同

#### Scenario: 告警集合变化
- **WHEN** 本次新增或减少了任一 Warn 项(host 或 field 维度,且未被归一化规则吸收)
- **THEN** 本次签名 MUST 与上次签名不同

#### Scenario: Unknown 与 Notice 不影响签名
- **WHEN** 本次 Warn 集合不变,但 Unknown 项或 Notice 项有增减
- **THEN** 签名 MUST 与上次相同

#### Scenario: 无告警时的签名
- **WHEN** 本次 Warn 列表为空
- **THEN** 签名 MUST 为空字符串或固定哨兵值,与"有告警"场景可区分

#### Scenario: RabbitMQ 同 vhost 队列名漂移
- **WHEN** 两次巡检均在同一 vhost(如 `bk_bkmonitorv3`)报告 `no_consumer` 告警,
  但具体队列名集合不同(如 `{celery_api_cron, celery_cron, celery_service}` vs
  `{celery_alert_builder, celery_cron, celery_service_access_event}`)
- **THEN** 两次签名 MUST 相同

#### Scenario: RabbitMQ 同 vhost 队列数量变化
- **WHEN** 同一 vhost 下 `no_consumer` 队列数量在两次巡检间从 3 个变为 5 个,但 vhost
  本身保持不变
- **THEN** 两次签名 MUST 相同

#### Scenario: RabbitMQ 跨 vhost 新增告警
- **WHEN** 上次仅 vhost A 有 `no_consumer` 告警,本次新增 vhost B 也出现
  `no_consumer` 告警
- **THEN** 本次签名 MUST 与上次签名不同

#### Scenario: RabbitMQ backlog 与 no_consumer 互不合并
- **WHEN** 同一 vhost 上次仅 `backlog` 告警,本次仅 `no_consumer` 告警
- **THEN** 两次签名 MUST 不同

#### Scenario: RabbitMQ 集群级 Field 不被折叠
- **WHEN** 告警 Field 为 `rabbitmq.error`、`rabbitmq.cluster_partition` 或
  `rabbitmq.node.<n>.mem_alarm`
- **THEN** 这些 Field MUST 原样参与签名,不被三段式归一化规则吸收

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

### Requirement: 通知状态持久化

系统 SHALL 在通知发送成功后将状态写入 `~/.config/weops/state.json`，记录
`last_sent_at` / `last_signature` / `last_status`。发送失败或抑制时 MUST 不更新
state（保留旧基线）。

#### Scenario: 发送成功后写入 state
- **WHEN** 邮件发送成功（SMTP 返回 250 等成功码）
- **THEN** 系统 SHALL 写入 state.json，`last_sent_at` 为当前时间，`last_signature`
  为本次签名，`last_status` 为 `alert` 或 `ok`（恢复邮件场景）

#### Scenario: 抑制时不更新 state
- **WHEN** 决策结果为"抑制"
- **THEN** state.json MUST 保持不变

#### Scenario: 发送失败时不更新 state
- **WHEN** SMTP 发送过程中出现任何错误
- **THEN** state.json MUST 保持不变，下次运行仍按上次的 last_signature 判定

#### Scenario: state.json 损坏
- **WHEN** state.json 存在但 JSON 解析失败
- **THEN** 系统 SHALL 视为冷启动（last_status 为空），打印 warning 提示文件已重置，
  并按"首次告警"规则继续判定

### Requirement: 周期化运行支持

系统 SHALL 支持作为 crontab 周期任务运行（推荐 `*/5 * * * *`），但本身 MUST 不
修改用户的 crontab。文档（README）MUST 提供示例 cron 行与环境变量加载方式。

#### Scenario: 周期触发不产生重复邮件
- **WHEN** crontab 每 5 分钟调用一次 weops-inspect，持续告警状态稳定
- **THEN** 在 2 小时窗口内 SHALL 仅发送一封告警邮件

#### Scenario: 文档提供 cron 示例
- **WHEN** 用户阅读 README 部署章节
- **THEN** README SHALL 给出至少一种可直接复用的 cron 行示例，覆盖 `bk.env` 加载、
  `WEOPS_CONFIG` 设置与 stderr 重定向

