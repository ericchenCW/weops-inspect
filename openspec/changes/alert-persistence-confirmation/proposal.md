## Why

当前告警触发逻辑在签名变化时立即发送邮件，导致**贴边抖动指标**（如 Redis 内存
~95%、RabbitMQ 单队列瞬时 `active=0`）每次进入或离开告警集合都会绕过冷却窗口产生
邮件。实际诊断价值低、噪声高。需要在 `*/5` 巡检节奏下加入"持续确认"机制：只有同
一 `(Host, Field)` 在**连续 2 次**巡检都告警，才视为有效告警进入决策；同时为避免
两次巡检并发执行污染 `state.json` 的读-改-写过程，需要单例锁。

## What Changes

- 在 `state.json` 中新增 `pending` 字段，记录每个 `(Host, Field)` 的连续告警次数
  与首次出现时间；告警进入决策前 MUST 通过持续确认过滤（默认 N=2）。
- 在 `state.json` 中新增 `recovery_streak` 字段，记录"连续多少次巡检 raw warns
  为空"。当前为告警状态时，仅当 streak 累计达到 N 次（与 alert 端共用同一
  `consecutive_runs`）才发送 recovery 邮件，避免抖动产生虚假"全部正常"通知。
- 新增 `notify.persistence` 配置段，包含 `consecutive_runs`（默认 2，下限 1=禁用）。
- 新增进程级单例锁（基于 `flock`，路径 `~/.config/weops/inspect.lock`）：当检测到
  已有实例运行时，新实例 MUST 立即退出且退出码为 0，stderr 打印 warning。
- 邮件正文 / HTML 报告 MUST NOT 展示仍处于 `pending` 状态的告警项（避免"没发邮件
  但报告里能看到"的语义割裂）。Summary 的 warn 计数 SHALL 同样仅计入已通过确认的
  告警。
- README 与 docs/checks.md 更新：说明持续确认、recovery streak 语义、单例锁行为
  及覆盖方式。

不在本期范围（后续单独提案）：按 Field 前缀绕过持续确认的"立即告警白名单"
（previously discussed as `immediate_fields`）。本期所有告警一视同仁走 N 次确认。

不向下兼容方面：旧 `state.json` 缺少 `pending` / `recovery_streak` 字段时按"空 map
+ 0"读入，不视为损坏；首次启用本特性时**所有**告警从 `pending(1)` 起步，第一次
巡检不会发告警邮件；同理，已处于告警状态的部署在升级后第一次"raw 清零"时不会立刻
发 recovery，需累积 N 次——这是设计预期，不是 bug。

## Capabilities

### New Capabilities
- 无

### Modified Capabilities
- `alert-notification`: 新增 alert 端"持续确认"决策前置层、`pending` 状态持久化、
  recovery 端 N 次确认（`recovery_streak`）、单例锁运行约束。

## Impact

- `notify/state.go`：`State` 结构新增 `Pending map[string]PendingItem` 与
  `RecoveryStreak int`。
- `notify/trigger.go` 或新建 `notify/persistence.go`：在 `Decide` 之前对 `[]AlertItem`
  做持续确认过滤；返回过滤后的 items + 更新后的 pending map。
- `notify/config.go`：新增 `Persistence` 配置段（`consecutive_runs`）。
- `notify/notify.go`：调度顺序为 lock → 巡检 → ExtractAlerts → 持续确认过滤 →
  Signature → Decide → Send → SaveState（含 pending）。
- `main.go`：在巡检开始前获取单例锁，结束/异常退出时释放。
- `model.InspectReport` 渲染层（`render/`）：剔除 pending 项后再渲染 HTML / Summary。
- 测试：`notify/persistence_test.go` 新增；现有 `trigger_test.go`、`signature_test.go`
  保持原语义不变（持续确认是前置层，不动既有决策矩阵）。
- 文档：README 通知章节、docs/checks.md 告警判定流程。
