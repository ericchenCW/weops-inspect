## Context

`weops-inspect` 由 crontab `*/5` 触发，每次巡检产生 `[]AlertItem`，经 `Signature →
Decide → Send` 决定是否发邮件，状态持久化在 `~/.config/weops/state.json`。当前
`Decide` 设计：**签名变化即立即发送（绕过冷却）**，目的是让"新增告警类别"快速可见。

实测发现两类抖动会绕过冷却：
1. **阈值贴边**：Redis master 内存 95% 阈值附近，96% ↔ 94% 在 1 分钟内反复跨越，
   告警集合反复进出。
2. **瞬时 Field**：RabbitMQ 单 vhost 单队列 `active=0` 是某次采样的瞬时值（消费者
   重连/掉线），归一化已折叠到 vhost 级，但 vhost 整体只有 1 个队列在边界时仍然
   抖动。

这些都不是真问题，但会按 spec 设计触发邮件。需要**在告警进入决策前**加一层时序
过滤：连续 N 次（默认 N=2）才算数。

并发约束：当前 `notify.Process` 的 `LoadState → Decide → SaveState` 没有任何锁，
两次巡检若并发执行（比如运维手动触发遇上 cron），TOCTOU 会让两个进程都读到同一
`prev`、各自决定发送、各自写 state，产生重复邮件且 pending 计数错乱。

## Goals / Non-Goals

**Goals:**
- 抖动指标不产生告警邮件；持续 ≥ 2 次巡检的告警 100% 可达。
- 单进程实例保证：同一 host 上同时只有一个 `weops-inspect` 在跑，避免 state.json
  污染。
- 报告 / 邮件视图与告警决策保持语义一致：未通过持续确认的项**不展示**，否则用户
  会困惑"为什么报告里有 warn 却没收到邮件"。
- recovery 邮件不被抖动欺骗：raw warns 连续 N 次为空才发"全部正常"。

**Non-Goals:**
- 不引入 SQLite / 时序数据库。`pending` map 量级仅几十到几百条，JSON 足够。
- 不改变现有签名归一化、冷却窗口（默认 120 min）逻辑。持续确认是**前置层**。
- 不实现"M-of-N"（任意 N 次中 M 次告警），仅"连续 N 次"。M-of-N 需要保留每个
  pending 项的窗口历史，复杂度跳升一个量级，本期不做。
- 不实现 per-field 立即告警白名单（`immediate_fields`）。所有告警一视同仁走 N 次
  确认；该需求后续单独提案。本期影响：进程缺失、端口不通、mysql 主从断等
  灾难类故障也会延迟 5~10 min（N=2, cron `*/5`）才告警，业务上接受这个权衡。
- 不修改 cron 间隔（仍 `*/5`）。

## Decisions

### Decision 1：用扩展 `state.json` 而非 SQLite

**选择**：在 `State` 结构体加 `Pending map[string]PendingItem`，键为 `Host|Field`
（与 signature 内部用的键一致），值含 `Count int` 与 `FirstSeen time.Time`。

**为什么不用 SQLite**：

- 数据量级：单次告警集合上限约 200 项；pending map 同量级。
- 写模式：每次巡检整体读、整体写，无并发查询、无范围扫描需求。
- 依赖成本：引入 mattn/go-sqlite3 = CGO 依赖 = 交叉编译复杂化。当前项目纯 Go +
  CGO_ENABLED=0 构建，是 release workflow 的硬约束。
- 备份/迁移：JSON 可以 `cat`、`jq`、手改、git 备份；SQLite 需要工具。
- 回滚：旧版本读到含 `pending` 字段的 state.json 会被 Go 的 JSON 解析器忽略多余
  字段，无缝降级。

### Decision 2：N 默认 2，可配但不推荐 ≥ 5

**选择**：`notify.persistence.consecutive_runs`，默认 2，最小 1（=禁用）。

**延迟矩阵**（cron `*/5`）：

| N | 最佳延迟 | 最坏延迟 | 抖动过滤强度 |
|---|---------|---------|------------|
| 1 | 0 min   | 5 min   | 不过滤      |
| 2 | 5 min   | 10 min  | 拦单次抖动  |
| 3 | 10 min  | 15 min  | 拦双次抖动  |

5 分钟采样下，连续 2 次都恰好命中边界的概率显著低于单次。N=2 是 cost/benefit
甜点。配置 ≥ 5 的延迟（≥ 25 min）在生产场景几乎不可接受，但保留可配性给极端
噪声场景。

### Decision 3：单例锁用 `flock(2)`

**选择**：在 `main.go` 入口处对 `~/.config/weops/inspect.lock` 加 `LOCK_EX |
LOCK_NB`；获取失败立即 stderr 打印并 `os.Exit(0)`。

**为什么 flock 而非 PID 文件**：

- PID 文件会留下"僵尸"（进程被 SIGKILL 后文件还在），需要解析+检测进程是否存活。
- flock 由内核管理，进程死亡自动释放，无僵尸。
- macOS / Linux 都支持，与现有部署目标一致（Windows 不在范围内）。
- Go 标准库提供 `golang.org/x/sys/unix.Flock`，无新外部依赖（`x/sys` 已在 go.sum）。

**退出码 0 而非 1**：cron 会把非 0 退出当成失败发系统邮件，重叠运行不是错误，是
保护性跳过。

### Decision 4：pending 项不进入报告渲染

**选择**：在 `model.InspectReport` 通过 notify 阶段过滤的项被标记为 `Pending`
状态（或直接从 Items 切片移除，再传给 render）。Summary 的 warn 计数同步剔除。

**为什么必须如此**：

```
   场景 A：报告含 pending 项, 邮件不含
   ──────────────────────────────────
   用户查看 HTML：  "看到 5 个 warn"
   用户检查邮箱：  "没收到邮件?"
   → 困惑、报错、丢失信任

   场景 B：报告与邮件一致 (推荐)
   ──────────────────────────
   用户查看 HTML：  "看到 3 个 warn"
   用户检查邮箱：  "收到 3 项告警邮件"
   → 一致、可信
```

代价：HTML 报告对运维"提前查看苗头"功能弱化。缓解：保留独立的"pending"区块（只
在 HTML 里展示），让运维知道"有 2 项正在累积观察"。本期可选实现；最低限度先把
pending 项从 Summary/邮件去掉。

### Decision 5：recovery 也走 N 次确认（系统级 streak）

**选择**：在 `state.json` 新增 `recovery_streak int`，记录"连续多少次巡检 raw warns
为空"。当 `prev.LastStatus == alert` 且本次原始 warn items 为空时累加；累加值达到
`consecutive_runs`（与 alert 端共享同一 N）才发送 recovery 邮件。任一巡检 raw warn
非空（即便全部进 pending）即归零。

**为什么镜像 alert 端**：

- 虚假 recovery 的危害**大于**虚假 alert：alert 误报让用户警觉变高（无害方向），
  recovery 误报让用户放下警惕（有害方向）。
- 灾难类故障（mysql、服务进程缺失）本身就可能在生命迹象边界抖动（mysql 偶发响应
  一次 ping 不代表已修），recovery 端的确认能避免"alert → 假 recovery → 真 alert"
  的振荡邮件序列。
- 噪声维度对称：解决了 alert 端的抖动噪声却放任 recovery 端抖动，等于把噪声搬家
  而不是消除。

**为什么 streak 跟踪 raw warns 而不是 filtered**：

```
   场景: T0 firing[mysql] → T1 raw=[mysql] 进 pending(1) filtered=[]
   
   定义 A (filtered 空):  T1 streak +1 → 看似要恢复, 实际 mysql 仍在
   定义 B (raw 空):       T1 raw 非空 → streak=0, 不会误报恢复  ✓
```

定义 B 严格镜像 alert 端"连续 N 次 (Host, Field) 出现"的语义，且不会被同期 pending
中的项欺骗。Unknown / Notice 不算 warn，与现有 spec 中"仅 Unknown 视为无告警"的
处理一致——全 Unknown 仍可推进 recovery streak。

**为什么不做 per-(Host, Field) recovery 确认（方向 A）**：

- 需要在 state.json 持久化"已 firing 项集合"，复杂度跳一档。
- 当前 spec 的 recovery 语义本来就是"集合整体清零"，不区分单项恢复。系统级 streak
  与现有语义同构，最小变更。
- 个别项恢复的语义可以未来再加（"集合非空但有项掉出"产生 partial recovery 邮件
  之类），与本期解耦。

**N 复用而非独立配置**：保持配置面简洁。如果未来有人要求"recovery 比 alert 更快/
更慢"，再拆 `recovery_consecutive_runs`。

### Decision 6：pending 数据 GC

**选择**：每次 `SaveState` 时，删除 `pending` 中本轮未出现的键（自然 reset）；
另对 `FirstSeen` 早于 24 小时的孤儿项强制删除（防御编码 bug）。

## Risks / Trade-offs

- **[风险] 首次启用首次巡检不发任何邮件**（所有项从 pending(1) 起步）→ **缓解**：
  文档明示 + 升级时打印 stderr `notify: persistence confirmation enabled, first
  alerts will be delayed by N runs`。
- **[风险] N=2 + cron `*/5` 导致最坏 15 min 延迟，对短时灾难（5~10 min 自愈型）
  完全错过** → **缓解**：业务上接受这个权衡；后续提案再加"立即告警白名单"覆盖
  灾难类（本期 Non-Goals）。
- **[风险] flock 锁在某些 NFS / overlay 文件系统上行为异常** → **缓解**：默认锁
  路径在用户 home（本地盘），不放到 `/tmp` 或共享存储；文档说明限制。
- **[风险] pending map 长期保留过期键导致 state.json 膨胀** → **缓解**：Decision 6
  的 24h GC + 每轮自然 reset。
- **[风险] 报告与邮件去掉 pending 项可能让 SRE 误以为"系统正常"，但其实有指标在
  累积** → **缓解**：HTML 报告底部展示 pending 区块（参见 Decision 4 增强方案，
  本期可选）。
- **[风险] 用户改了 cron 间隔（如 `*/1`）但 N 没改，等价于过度敏感** → **缓解**：
  README 提供推荐组合表（cron × N → 延迟）。
- **[风险] recovery 邮件延迟 5~10 min（N=2 + `*/5`）** → **缓解**：业务上 recovery
  的紧迫性远低于 alert，延迟可接受；文档明示。
- **[风险] 长期不稳定的"小抖动"项让 recovery streak 永远不归零，导致主告警的
  recovery 邮件永远不发** → **缓解**：这是设计意图——系统不是"完全干净"就不宣布
  全员恢复；用户接受此语义。运维若发现某指标长期处于 pending 噪声态，应主动调阈值
  解决。文档需明示。

## Migration Plan

1. 升级二进制，默认配置 `consecutive_runs=2`。
2. 第一次巡检：所有现存告警进入 pending(1)，**不发任何邮件**。stderr 提示。
3. 第二次巡检（5 min 后）：仍存在的告警晋升 firing → 走原决策矩阵 → 发邮件。
4. 抖动告警：第二次巡检时已不在集合 → pending 项被删除 → 不发邮件。
5. recovery 路径：当持续告警状态下 raw warns 首次清零，进入 `recovery_streak=1`，
   仍不发邮件；连续清零达到 N 次后发送 recovery，状态转 ok 并清零 streak。

回滚：旧二进制读到含 `pending` / `recovery_streak` 字段的 state.json，多余字段被
忽略，行为退回"立即告警 + 立即恢复"。无破坏性。

## Open Questions

- pending 区块是否纳入 HTML 渲染：本期 spec 不强制，留给后续小改动决定。
- 是否需要给 `consecutive_runs` 设上限（如 ≤ 10）来防误配？倾向加，但不阻塞 spec。
