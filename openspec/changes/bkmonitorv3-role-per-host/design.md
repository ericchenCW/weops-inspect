## Context

bkmonitorv3 是蓝鲸的伞形监控后台模块,生产部署中 4 个子角色(monitor / influxdb-proxy / transfer / unify-query,以及未采集的 ingester / grafana)常被运维分散到不同主机以平衡 IO/CPU 与资源占用。

当前 `ModuleRegistry["bkmonitorv3"]` 把 4 个 sub-module 绑死在同一个 IP 列表 `BK_MONITORV3_IP_COMMA` 上,`collectServiceOnHost` 会在每台主机上对所有 4 个 unit 都跑一遍 `systemctl show` 与 healthz。当某主机只跑其中两个 sub-module 时,缺失的两个会被记为 `not-found / unreachable`,产生大量误报。

约束:
- 不能破坏 `BK_MONITORV3_IP_COMMA` 老语义 — 部分用户(单机部署)只设这一个变量。
- 依赖采集 `CollectBKMonitorV3Deps` 仍以"是否部署 bkmonitorv3"为开关,不应受拆分影响。
- HTML 报告与现有 `Services` map 渲染逻辑通过 module key 驱动,key 拆分会自动反映到 UI。

## Goals / Non-Goals

**Goals:**
- 4 个子角色按各自部署主机独立采集,消除 cross-host 误报。
- 兼容只设 `BK_MONITORV3_IP_COMMA` 的旧配置。
- 报告结构清晰,4 个角色独立成段。

**Non-Goals:**
- 不为 `ingester` / `grafana` 引入新角色采集(需求明确暂不做)。
- 不重构 `service.go` 流水线(`collectServiceOnHost` 沿用)。
- 不改 `CollectBKMonitorV3Deps` 的行为或字段。

## Decisions

### D1: 拆 `bkmonitorv3` 为 4 个 module key,而非保留单 key 在 sub-module 上加 IP 字段

`ModuleRegistry` 的 value 是 `[]SubModule`,每个 sub 的部署主机由所属 module 的 IPs 决定。要做"每 sub 独立 IPs",可选:

| 方案 | 改动 | 取舍 |
|---|---|---|
| **A. 拆 module key**(选定) | `bkmonitorv3` → 4 个独立 key,每个含 1 个 sub | 数据结构不变,`GetModuleHosts()` 返回 4 条记录,`CollectAllServices` 自然按 IP 调度,无需新增字段 |
| B. 给 SubModule 加 `IPs []string` | 改 `SubModule` 结构 + `collectServiceOnHost` 的循环语义 | 改动面更小,但语义与"module 是 IP 的归属"冲突,易引入回归 |

**选 A**:`SubModule` 不动,只在 `ModuleRegistry` 与 `GetModuleHosts()` 多列几行。报告 JSON 顶层 `Services` 多 4 个 key,渲染层按 key 自动产出 4 段。

### D2: 旧变量回退在 `GetModuleHosts()` 内做

`Config` 仍存 5 个独立切片(`MonitorV3IPs`、`MonitorV3MonitorIPs`、…)。`Load()` 解析时不做合并;`GetModuleHosts()` 在为每个 `bkmonitorv3-*` 条目取 IP 时,若该角色专属切片为空,则回退到 `MonitorV3IPs`。

```go
func pickRoleIPs(role, fallback []string) []string {
    if len(role) > 0 { return role }
    return fallback
}
```

集中在 `GetModuleHosts()` 处理,避免 `Load()` 阶段产生"已合并"的状态难以审视。

### D3: `AllHosts` 与 `CollectBKMonitorV3Deps` 的开关条件

- `buildAllHosts()` 把 5 个切片(`MonitorV3IPs` + 4 个角色)统一去重纳入。这样即便用户只设角色变量、没设 `BK_MONITORV3_IP_COMMA`,主机 OS 指标也能覆盖。
- `CollectBKMonitorV3Deps` 现在 `if len(cfg.MonitorV3IPs) == 0 { return nil }` 改为"4 个角色 IP 全空 **且** `MonitorV3IPs` 也空 → return nil"。这样新用户即使不再设 `BK_MONITORV3_IP_COMMA`,只要至少一个角色 IP 有,依赖采集仍跑。

### D4: 报告 JSON `Services["bkmonitorv3"]` 键消失 = BREAKING

不做"双写兼容键"。理由:本工具的 JSON 当前没有外部消费方契约保证;保留双键反而让 `bkmonitorv3` 的语义模糊(到底是哪个角色?)。在 proposal 中明确标注 BREAKING。

## Risks / Trade-offs

- [HTML 模板中如果有硬编码 `bkmonitorv3` 章节标题] → 改为根据 `Services` map 中前缀 `bkmonitorv3-` 聚合渲染,或为 4 个 key 各加标题。需 review [render/templates/](render/templates/) 现状。
- [`ProcessName: "supervisord"` 的 worker 计数仍只反映 supervisor 自身] → 无变化,沿用现状(已在 service_registry 注释里说明)。
- [回退兼容路径让旧用户感知不到拆分] → 如果旧用户的部署其实是分布式的,他们仍会看到误报。这是预期 — 提示在发布说明中建议拆分配置。

## Migration Plan

1. 部署新版本前,运维按实际拓扑设置 4 个 `BK_MONITORV3_*_IP_COMMA`,可保留 `BK_MONITORV3_IP_COMMA` 不动(用作 dep 探测开关)。
2. 若不设角色变量,行为完全等同旧版本(回退路径)。
3. 回滚:还原代码即可,env 变量是新增的,旧二进制忽略它们,无副作用。
