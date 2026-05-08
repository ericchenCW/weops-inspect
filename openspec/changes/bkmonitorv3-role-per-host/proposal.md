## Why

`bkmonitorv3` 是一个伞形模块,其 4 个子角色(`monitor` / `influxdb-proxy` / `transfer` / `unify-query`)在生产环境通常被运维拆到不同主机部署:

```
10.10.26.235  monitorv3(monitor), monitorv3(influxdb-proxy), monitorv3(ingester)
10.10.26.236  monitorv3(transfer), monitorv3(unify-query)
```

但当前实现把 `BK_MONITORV3_IP_COMMA` 一组 IP 同时作为 4 个子模块的部署主机,会在 .235 上找 `bk-transfer.service`、在 .236 上找 `bk-monitor.service`,导致 systemctl `not-found` 与 healthz `unreachable` 大量误报,巡检报告几乎全红。

需要让每个子角色按自己的部署主机独立采集。

## What Changes

- 新增 4 个环境变量,分别表示每个子角色的部署主机:
  - `BK_MONITORV3_MONITOR_IP_COMMA`
  - `BK_MONITORV3_INFLUXDB_PROXY_IP_COMMA`
  - `BK_MONITORV3_TRANSFER_IP_COMMA`
  - `BK_MONITORV3_UNIFY_QUERY_IP_COMMA`
- `Config` 增加 4 个对应字段,`Load()` 解析这些 env;原 `MonitorV3IPs`(`BK_MONITORV3_IP_COMMA`)保留语义为"是否部署 bkmonitorv3",继续作为 `CollectBKMonitorV3Deps` 的开关与 `AllHosts` 主机来源(回退:若未设置任何角色变量,则把 `MonitorV3IPs` 当作所有 4 个角色的部署主机以兼容旧配置)。
- `ModuleRegistry` 把 `bkmonitorv3` 这一项拆为 4 个独立模块键:`bkmonitorv3-monitor` / `bkmonitorv3-influxdb-proxy` / `bkmonitorv3-transfer` / `bkmonitorv3-unify-query`,每个键下只含自己的一个 sub-module。
- `GetModuleHosts()` 输出 4 条 `bkmonitorv3-*` 记录,IP 来自对应的角色变量。
- HTML 报告 `bkmonitorv3` 章节按 4 个角色分别渲染对应主机的进程巡检结果(每个 sub-module 表只列其实际部署主机)。
- **不**采集 `ingester` 角色,该角色暂不在 registry 中(明确决策,避免与 monitor 进程混淆)。
- **BREAKING(行为)**:报告中 `Services["bkmonitorv3"]` 这个键将不再出现;改由 `Services["bkmonitorv3-monitor"]` 等 4 个键替代。下游若直接消费 JSON 字段名需同步调整。

## Capabilities

### New Capabilities
- (无)

### Modified Capabilities
- `bkmonitorv3-collection`: 子模块采集从"统一 IP 列表 × 4 子模块"改为"每子模块独立 IP 列表"。
- `bk-config-loading`: 新增 4 个 `BK_MONITORV3_*_IP_COMMA` 解析。

## Impact

- **代码**
  - [config/config.go](config/config.go) — `Config` 结构、`Load()`、`GetModuleHosts()`、`buildAllHosts()`。
  - [collector/service_registry.go](collector/service_registry.go) — 拆分 `bkmonitorv3` 为 4 个键。
  - [collector/bkmonitor_dep.go](collector/bkmonitor_dep.go) — 依赖采集开关条件不变(仍看 `MonitorV3IPs`),但需处理"未配 `BK_MONITORV3_IP_COMMA` 但配了角色 IP"的回退。
  - [render/templates/](render/templates/) — bkmonitorv3 章节渲染要识别 4 个新 key。
- **行为**
  - 报告 JSON 顶层 `Services` 子键变化(见上)。
  - 旧用户若只设了 `BK_MONITORV3_IP_COMMA` 不设角色变量 → 仍按原来"四角色都在这批 IP 上"采集,以保持兼容。
- **依赖 / 接口**:无外部 API 变化。
