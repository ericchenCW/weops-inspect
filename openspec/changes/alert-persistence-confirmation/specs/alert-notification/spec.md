## ADDED Requirements

### Requirement: 持续确认前置过滤

系统 SHALL 在告警进入决策（`Decide`）前，对 `[]AlertItem` 执行"连续 N 次"
过滤：仅当某 `(Host, Field)` 在最近连续 N 次巡检中均出现，才视为有效告警进入
后续签名计算与决策。N 由 `notify.persistence.consecutive_runs` 配置，默认值
MUST 为 2，允许的最小值 MUST 为 1（等同禁用本特性）。

未通过持续确认的告警项 MUST NOT 进入：
- 签名计算 `Signature(items)` 的输入
- 决策函数 `Decide` 的 `warnCount` 与 `sigNow`
- 邮件正文与主题
- HTML 报告的 warn 明细与 Summary 的 warn 计数

`pending` 累积计数 MUST 持久化在 `state.json` 的 `pending` 字段，结构为
`map[string]{count, first_seen}`，键为 `Host|Field`（与签名内部键一致，但
**不**经过 RabbitMQ 队列归一化——持续确认在归一化之前作用于原始 Field）。

#### Scenario: 首次出现的告警进入 pending
- **WHEN** 某 `(Host, Field)` 在 `prev.Pending` 中不存在，本次巡检报告告警
- **THEN** 系统 SHALL 在新 state 中写入 `pending[Host|Field] = {count: 1, first_seen: now}`
- **AND** 该项 MUST NOT 进入告警决策
- **AND** 该项 MUST NOT 出现在邮件或 HTML 报告的 warn 明细中

#### Scenario: 连续 N 次告警晋升为 firing
- **WHEN** N=2，某 `(Host, Field)` 上次 `pending.count=1`，本次巡检仍报告告警
- **THEN** 该项 SHALL 通过持续确认，进入 `Decide` 的输入
- **AND** 该项 MUST 出现在邮件与 HTML 报告中
- **AND** 新 state 中该键 MUST 从 `pending` 中移除（已晋升）

#### Scenario: 告警在 pending 阶段消失（抖动）
- **WHEN** 某 `(Host, Field)` 上次 `pending.count=1`，本次巡检不再报告告警
- **THEN** 系统 SHALL 在新 state 中删除该 `pending` 键
- **AND** 该项 MUST NOT 触发任何邮件
- **AND** 不视为"恢复"事件（此前未发出过告警）

#### Scenario: N=1 等同禁用
- **WHEN** `notify.persistence.consecutive_runs=1`
- **THEN** 任何告警项 SHALL 第一次出现即通过过滤，进入决策

#### Scenario: pending 状态独立于签名归一化
- **WHEN** 同一 vhost 下两次巡检的 RabbitMQ 队列名集合变化（如 File 1 触发
  `rabbitmq.bk_bkmonitorv3.celery_api_cron.no_consumer`，File 2 触发
  `rabbitmq.bk_bkmonitorv3.celery_alert_builder.no_consumer`）
- **THEN** 这两次的 `pending` 键 MUST 不同（基于原始 Field）
- **AND** 各自独立累积；任一单独抖动不应被另一个意外晋升

#### Scenario: pending 项的 24 小时 GC
- **WHEN** `pending` 中某项的 `first_seen` 早于当前时间 24 小时，但本次巡检
  未再次出现该 `(Host, Field)`
- **THEN** 系统 SHALL 在新 state 中删除该键

### Requirement: Recovery 持续确认

系统 SHALL 在已处于告警状态时，对 recovery 邮件的发送施加"连续 N 次 raw warns
为空"的前置确认。引入 `state.recovery_streak` 整型字段持久化连续清零次数，N 复用
`notify.persistence.consecutive_runs`（与 alert 端共享同一配置项）。

streak 演化规则：
- 当 `prev.LastStatus == alert` 且本次原始 warn items（即 `ExtractAlerts` 输出，
  在持续确认过滤之前）为空，`recovery_streak` MUST 在原值基础上 +1。
- 当 raw warns 非空（即便所有项都被持续确认过滤为 pending），`recovery_streak`
  MUST 重置为 0。
- 当 `prev.LastStatus != alert`，`recovery_streak` MUST 保持为 0（不参与决策）。
- 成功发送 recovery 邮件后，`recovery_streak` MUST 重置为 0。

#### Scenario: recovery streak 累积
- **WHEN** N=2，prev.LastStatus=alert，T0 巡检 raw warns 为空（首次清零）
- **THEN** 系统 SHALL 在新 state 中写入 `recovery_streak=1`
- **AND** 不发送 recovery 邮件
- **AND** `last_status` MUST 仍为 `alert`
- **AND** `last_sent_at` / `last_signature` MUST 保持不变

#### Scenario: recovery streak 达到 N 次
- **WHEN** N=2，prev.LastStatus=alert 且 `prev.recovery_streak=1`，T1 巡检 raw warns
  仍为空
- **THEN** 系统 SHALL 发送 recovery 邮件
- **AND** 新 state 中 `last_status` MUST 为 `ok`、`last_signature` MUST 为空、
  `recovery_streak` MUST 重置为 0

#### Scenario: streak 被 raw warns 重置（pending 期间也重置）
- **WHEN** prev.LastStatus=alert 且 `prev.recovery_streak=1`，本次巡检 raw warns
  非空（无论这些项是否通过持续确认过滤）
- **THEN** 新 state 中 `recovery_streak` MUST 为 0
- **AND** 系统 MUST NOT 发送 recovery 邮件

#### Scenario: 全 Unknown 推进 streak
- **WHEN** prev.LastStatus=alert，本次 Warn==0 但 Unknown>0
- **THEN** 视为 raw warns 为空，`recovery_streak` MUST +1（与现有"仅 Unknown 视为
  无告警"语义一致）

#### Scenario: prev.LastStatus 不是 alert 时 streak 不参与
- **WHEN** prev.LastStatus 为 ok 或空字符串
- **THEN** `recovery_streak` MUST 在新 state 中保持为 0
- **AND** 系统 MUST 不进入 recovery 路径

### Requirement: 单实例运行锁

系统 SHALL 在 `main.go` 启动早期获取一个建议性文件锁，路径为
`~/.config/weops/inspect.lock`（或 `WEOPS_CONFIG` 同目录下的 `inspect.lock`），
使用 `flock(2)` 的 `LOCK_EX | LOCK_NB`。锁 MUST 在进程退出时由内核自动释放。

当锁获取失败（已被另一实例持有）时，新实例 MUST：
1. 在 stderr 打印 `weops-inspect: another instance is running, exiting`；
2. 立即退出，退出码 MUST 为 0（保护性跳过，不触发 cron 系统邮件）；
3. MUST NOT 执行任何采集、渲染、通知逻辑；
4. MUST NOT 修改 `state.json` 或写入 HTML 报告。

#### Scenario: 串行运行（正常）
- **WHEN** 上一次 `weops-inspect` 已退出，新一次 cron 触发
- **THEN** 新实例 SHALL 成功获取锁并完整执行

#### Scenario: 并发运行被拒绝
- **WHEN** 一次 `weops-inspect` 仍在运行（如执行时间超过 cron 间隔），下一次
  cron 触发新实例
- **THEN** 新实例 SHALL 立即退出，退出码为 0，stderr 打印警告
- **AND** state.json MUST 不被修改
- **AND** HTML 报告 MUST 不被覆盖

#### Scenario: 异常退出后锁自动释放
- **WHEN** 持有锁的进程被 `SIGKILL` 或崩溃
- **THEN** 内核 SHALL 自动释放 flock，下次运行可正常获取

#### Scenario: 锁路径不可写
- **WHEN** 锁文件父目录不存在或无写权限
- **THEN** 系统 SHALL 在 stderr 打印 warning 并继续运行（降级为无锁），
  巡检 MUST 不因锁不可用而失败

## MODIFIED Requirements

### Requirement: 通知决策

系统 SHALL 综合本次告警状态、上次状态（state.json）、签名是否变化、距上次发送的
时间间隔，决定本次执行三种动作之一：发送告警邮件、发送恢复邮件、抑制不发。冷却
窗口 MUST 为 2 小时。Unknown 与 Notice 项 MUST 不触发任何邮件动作。

`Decide` 的输入 `warnCount` 与 `sigNow` MUST 为**经持续确认过滤后**的告警集合
计算结果（参见"持续确认前置过滤"要求）。

恢复路径 MUST 走 N 次确认：当上次状态为 `alert` 且本次原始 warn items 为空，
仅当 `recovery_streak` 累积达到 N 次（参见"Recovery 持续确认"要求）才触发
recovery 邮件；未达 N 次时 MUST 抑制本次通知，但 `recovery_streak` 仍按规则
更新。

#### Scenario: 首次告警（冷启动）
- **WHEN** state.json 不存在或 last_status 为空，且本次过滤后 Warn>0
- **THEN** 系统 SHALL 发送告警邮件

#### Scenario: 仅有 Unknown 项
- **WHEN** 本次 Warn==0 且 Unknown>0
- **THEN** 系统 SHALL 视为"无告警"，按"持续正常"或"告警恢复"分支处理

#### Scenario: 持续告警且签名相同、未超冷却窗口
- **WHEN** 上次状态为 alert，本次过滤后 Warn>0 且签名与上次相同，距上次发送 < 2 小时
- **THEN** 系统 SHALL 抑制本次通知，不发送邮件

#### Scenario: 持续告警但签名变化
- **WHEN** 上次状态为 alert，本次过滤后 Warn>0 但签名与上次不同（告警集合变化）
- **THEN** 系统 SHALL 立即发送告警邮件，不受冷却窗口限制

#### Scenario: 持续告警且签名相同但已超冷却窗口
- **WHEN** 上次状态为 alert，本次过滤后 Warn>0 且签名相同，距上次发送 ≥ 2 小时
- **THEN** 系统 SHALL 重新发送告警邮件

#### Scenario: 告警恢复（达到 N 次 streak）
- **WHEN** 上次状态为 alert，本次原始 warn items 为空，且 `recovery_streak` 在
  本次累加后达到 N
- **THEN** 系统 SHALL 发送恢复通知邮件

#### Scenario: 告警状态下首次清零（streak 不足）
- **WHEN** 上次状态为 alert，本次原始 warn items 为空，但 `recovery_streak` 累加
  后仍 < N
- **THEN** 系统 MUST NOT 发送任何邮件
- **AND** `last_status` MUST 仍为 `alert`，下一次巡检从 `recovery_streak` 当前值
  继续累积或归零

#### Scenario: 告警状态下 raw 非空但 filtered 为空
- **WHEN** 上次状态为 alert，本次原始 warn items 非空（全部进 pending），过滤后
  filtered=[]
- **THEN** 系统 MUST NOT 发送任何邮件
- **AND** `recovery_streak` MUST 归零（不视为推进恢复）

#### Scenario: 持续正常
- **WHEN** 上次状态为 ok 或空，本次过滤后 Warn=0
- **THEN** 系统 SHALL 不发送任何邮件

#### Scenario: 抖动告警不触发邮件
- **WHEN** N=2，某告警项在巡检 T0 首次出现进入 pending(1)，巡检 T1（5 min 后）
  不再出现
- **THEN** 系统 SHALL 在 T0 与 T1 都不发送任何邮件
- **AND** 上次状态为 ok 时，T1 不触发恢复邮件（未发过告警，无可恢复）

### Requirement: 通知状态持久化

系统 SHALL 在通知发送成功后将状态写入 `~/.config/weops/state.json`，记录
`last_sent_at` / `last_signature` / `last_status` / `pending` / `recovery_streak`。
发送失败或抑制时，`last_sent_at` / `last_signature` / `last_status` MUST 不变
（保留旧基线），但 `pending` 与 `recovery_streak` 字段 MUST 仍按本次巡检结果更新
（持续确认计数、recovery 累积与告警发送解耦）。

#### Scenario: 发送告警邮件后写入 state
- **WHEN** 告警邮件发送成功（SMTP 返回 250 等成功码）
- **THEN** 系统 SHALL 写入 state.json，`last_sent_at` 为当前时间，`last_signature`
  为本次签名，`last_status` 为 `alert`，`pending` 为本次更新后的 map，
  `recovery_streak` 为 0

#### Scenario: 发送 recovery 邮件后写入 state
- **WHEN** recovery 邮件发送成功
- **THEN** 系统 SHALL 写入 state.json，`last_sent_at` 为当前时间，`last_signature`
  为空字符串，`last_status` 为 `ok`，`pending` 按本次更新（通常为空 map），
  `recovery_streak` MUST 重置为 0

#### Scenario: 抑制时仍更新 pending 与 recovery_streak
- **WHEN** 决策结果为"抑制"（冷却窗口内、全部告警仍在 pending、或 recovery streak
  未达 N）
- **THEN** `last_sent_at` / `last_signature` / `last_status` MUST 保持不变
- **AND** `pending` MUST 按本次巡检的告警集合更新
- **AND** `recovery_streak` MUST 按规则累加或归零

#### Scenario: 发送失败时保留全部历史
- **WHEN** SMTP 发送过程中出现错误
- **THEN** `last_sent_at` / `last_signature` / `last_status` / `pending` /
  `recovery_streak` MUST 全部保持不变，下次运行从同一基线重新累积

#### Scenario: state.json 损坏
- **WHEN** state.json 存在但 JSON 解析失败
- **THEN** 系统 SHALL 视为冷启动（last_status 为空、pending 为空 map、
  recovery_streak 为 0），打印 warning 提示文件已重置，并按"首次告警"规则继续判定

#### Scenario: 旧版本 state.json 兼容
- **WHEN** state.json 来自不含 `pending` / `recovery_streak` 字段的旧版本
- **THEN** 系统 SHALL 将 `pending` 解析为空 map、`recovery_streak` 解析为 0，所有
  当次告警从 `pending(1)` 起步；若 prev.LastStatus=alert 且本次 raw 清零，从
  `recovery_streak=1` 起步累积

### Requirement: 邮件发送

系统 SHALL 通过 SMTP（支持 TLS / STARTTLS / 明文 AUTH PLAIN）发送邮件，HTML 巡检
报告嵌入正文同时作为附件携带。发送阶段 MUST 受超时保护，且失败不得改变巡检主流程
退出码。告警邮件正文 SHALL 在每条 Warn 明细行展示触发该告警的阈值或期望值
（当该规则存在单一阈值时）。

邮件正文与 HTML 报告 MUST NOT 包含未通过持续确认的告警项；Summary 的 warn 计数
MUST 仅反映通过过滤的告警数量。

#### Scenario: 告警邮件结构
- **WHEN** 发送告警邮件
- **THEN** 邮件主题 SHALL 形如 `[WeOps 巡检告警] {warn}/{total} 项异常`，其中
  `warn` 仅计入通过持续确认的告警数
- **AND** 正文 SHALL 包含巡检时间戳、Summary 数字（含 Unknown 计数）、所有通过过滤
  的 Warn 项的 host / field / value
- **AND** 对存在单一阈值或期望值的 Warn 项，正文 SHALL 在该行追加阈值描述
  （如 `(阈值 ≥ 95%)`、`(阈值 期望 active)`）
- **AND** 对无单一阈值的规则项（如 `load_average`、`mysql_master.read_only`、`redis.role`、
  `rabbitmq.<vhost>.<queue>.no_consumer`），正文 MUST NOT 追加阈值描述
- **AND** 正文 MUST 不展示 Unknown、Notice 项以及任何 pending 项的明细
- **AND** 附件 SHALL 包含本次生成的 `weops_inspection.html`

#### Scenario: 阈值不影响告警签名
- **WHEN** 两次巡检 Warn 集合的 host + field 完全一致，但任一项的阈值描述发生变化
  （例如 CPU 阈值从 `≥ 95%` 调整为 `≥ 90%`）
- **THEN** 两次签名 MUST 相同，冷却窗口与抑制行为 MUST 不受阈值变更影响

#### Scenario: HTML 报告与邮件视图一致
- **WHEN** 本次巡检产生 5 项 warn，其中 3 项通过持续确认、2 项处于 pending
- **THEN** HTML 报告的 warn 明细 SHALL 仅展示 3 项
- **AND** Summary 的 warn 计数 SHALL 为 3
- **AND** 邮件正文 SHALL 同样仅展示 3 项

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

### Requirement: 周期化运行支持

系统 SHALL 支持作为 crontab 周期任务运行（推荐 `*/5 * * * *`），但本身 MUST 不
修改用户的 crontab。文档（README）MUST 提供示例 cron 行与环境变量加载方式，并
SHALL 提供"cron 间隔 × `consecutive_runs` → 检测延迟"的推荐组合表，帮助用户在
延迟与噪声之间选择。

文档 MUST 明示：启用持续确认后，首次部署时第一次巡检不会发送任何邮件，所有现存
告警从 pending(1) 起步。

#### Scenario: 周期触发不产生重复邮件
- **WHEN** crontab 每 5 分钟调用一次 weops-inspect，持续告警状态稳定
- **THEN** 在 2 小时窗口内 SHALL 仅发送一封告警邮件

#### Scenario: 文档提供 cron 示例
- **WHEN** 用户阅读 README 部署章节
- **THEN** README SHALL 给出至少一种可直接复用的 cron 行示例，覆盖 `bk.env` 加载、
  `WEOPS_CONFIG` 设置与 stderr 重定向

#### Scenario: 文档说明持续确认延迟
- **WHEN** 用户阅读 README 通知章节
- **THEN** 文档 SHALL 提供至少一组 `cron 间隔 × consecutive_runs` 的延迟示例
  （如 `*/5` × N=2 → 5~10 min；`*/1` × N=3 → 2~3 min）
