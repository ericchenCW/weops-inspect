## 1. 数据结构与配置

- [x] 1.1 在 `notify/state.go` 的 `State` 结构体新增 `Pending map[string]PendingItem` 与 `RecoveryStreak int` 字段；定义 `PendingItem{Count int; FirstSeen time.Time}`
- [x] 1.2 调整 `LoadState`：旧 state.json 缺少 `pending` 字段时返回空 map 而非 nil（避免 nil map 写入 panic）；缺少 `recovery_streak` 时按 0 处理
- [x] 1.3 在 `notify/config.go` 新增 `Persistence` 配置段：`ConsecutiveRuns int`（默认 2，下限 1=禁用本特性）
- [x] 1.4 `Validate` 增加：`consecutive_runs >= 1` 检查；越界给 stderr warning 并 fallback 到默认 2

## 2. 持续确认核心逻辑

- [x] 2.1 新建 `notify/persistence.go`，定义入口 `ApplyPersistence(items []AlertItem, prev map[string]PendingItem, n int, now time.Time) (firing []AlertItem, nextPending map[string]PendingItem)`
- [x] 2.2 主循环：对每个 item 查 prev pending，count+1 ≥ N 进 firing 并从 next 删除，否则进 next（FirstSeen 沿用或本次 now）
- [x] 2.3 GC 逻辑：prev 中本轮未出现的键全部丢弃（自然 reset）；防御性地，FirstSeen 早于 now-24h 的键也强制丢弃
- [x] 2.4 单元测试 `notify/persistence_test.go` 覆盖：首次告警进 pending、连续 N 次晋升、抖动 reset、N=1 立即通过、24h GC、原始 Field（未归一化）作为键

## 3. Recovery streak 与 notify.Process 接入

- [x] 3.1 实现 recovery streak 演化函数 `UpdateRecoveryStreak(prevStreak int, prevStatus string, rawWarnsEmpty bool) int`：prev=alert + raw 空 → +1；raw 非空 → 0；prev≠alert → 0
- [x] 3.2 在 `notify/notify.go` 的 `Process` 中编排顺序：`ExtractAlerts` → 计算 `rawWarnsEmpty` → `ApplyPersistence` 得 filtered → 计算 newStreak → 决策
- [x] 3.3 决策修改：当 prev.LastStatus=alert 且 rawWarnsEmpty 时，不直接发 recovery，先看 newStreak ≥ N 才发；未达 N 则视为抑制，但仍写 streak
- [x] 3.4 用过滤后 items 计算 `Signature` 与 `len(items)` 喂给 `Decide`（仅 alert 路径）
- [x] 3.5 `SaveState` 写入新 `Pending` 与 `RecoveryStreak`：成功发送时同时更新所有字段（recovery 邮件后 streak 归零）；抑制时仅更新 `Pending` 与 `RecoveryStreak`；发送失败时全部回滚（不写）
- [x] 3.6 单元测试覆盖端到端：(a) 抖动告警在 `*/5` × N=2 下不发邮件; (b) 持续 2 次告警发邮件; (c) alert → 单次清零 → 不发 recovery; (d) alert → 连续 2 次清零 → 发 recovery; (e) alert → 清零 → raw 非空 → streak 重置 → 不发虚假 recovery

## 4. 报告 / 邮件视图同步

- [x] 4.1 决定过滤位置：在 `notify.Process` 调用 `ApplyPersistence` 之前，把过滤结果同步到 `model.InspectReport`（移除 pending 项的 Items 行 + 重算 Summary.Warn）
- [x] 4.2 修改 `render/` 调用顺序：HTML 渲染发生在过滤之后；如果当前 main.go 是"先渲染 HTML 后通知"，需要倒置——确保 HTML 与邮件看到的 warn 集合一致
- [x] 4.3 验证 `BuildAlertSubject` / `BuildAlertBody` / HTML Summary 数字均反映过滤后计数
- [x] 4.4 集成测试或手工验证：构造含 5 项 warn（其中 2 项 pending）的 report，HTML 与邮件正文均仅含 3 项

## 5. 单实例锁

- [x] 5.1 新建 `lock/lock.go`，用 `golang.org/x/sys/unix.Flock` 实现 `Acquire(path string) (release func(), err error)`
- [x] 5.2 锁路径：默认 `filepath.Join(filepath.Dir(notifyConfigPath), "inspect.lock")`，与 state.json 同目录；目录不存在时 `MkdirAll(0700)` 创建（失败则降级）
- [x] 5.3 `LOCK_EX | LOCK_NB`：失败时区分"已被占用"（返回 `ErrBusy`）与"系统错误"；前者 stderr warning + 退出码 0；后者 warning 后降级继续运行
- [x] 5.4 在 `main.go` 早期（配置加载之后、采集开始之前）调用，使用 `defer release()` 在正常退出时释放（崩溃/SIGKILL 由内核回收）
- [x] 5.5 单元测试：串行获取锁成功；并发尝试时第二个返回 `ErrBusy`；嵌套目录自动创建

## 6. 文档更新

- [x] 6.1 README 通知章节新增"持续确认"小节：解释 N、首次部署延迟、recovery 端的 N 次确认、"长期 pending 噪声会阻塞 recovery"语义
- [x] 6.2 README 增加 `cron 间隔 × consecutive_runs → 延迟` 推荐组合表（至少 4 行：`*/5` × {1,2,3} 与 `*/1` × 3）
- [x] 6.3 README 单实例锁小节：默认锁路径、为何并发时退出码 0、NFS/共享盘风险提醒
- [x] 6.4 `docs/checks.md` 告警判定流程图加入持续确认前置层（新增 4.3 节并 renumber）
- [x] 6.5 在示例 `config.json` 中加 `notify.persistence` 字段，注释说明默认值（README 内嵌示例已更新；项目无独立 example 文件）

## 7. 回归与验证

- [x] 7.1 现有 `notify/trigger_test.go` 与 `signature_test.go` MUST 全部通过（持续确认是前置层，决策矩阵不变）
- [x] 7.2 新增端到端场景测试：模拟连续巡检（含 alert 端抖动、真实持续告警、recovery 端抖动、连续清零、raw 非空重置）的 state 演化（5 个 E2E 测试覆盖）
- [x] 7.3 离线复现持续确认行为：`TestE2E_RealWorldFlapping` 模拟 /tmp/a File 1→2 的告警集合差异，验证 flap-out 与 flap-in 项不会进入邮件
- [x] 7.4 `make build` + `make test` 全绿
- [ ] 7.5 在测试机用 `*/5` cron 跑 30 分钟，观察 state.json 中 pending map 的演化与邮件触发时机（手工验证，待部署后由用户执行）
