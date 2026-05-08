## Context

weops-inspect 现有进程巡检走 [collector/service.go](collector/service.go) 的固定流水线: 按 host 拼一段 shell, 远端执行后解析 systemctl/healthz/worker。bkmonitorv3 之前完全没有接入, 而 `bk-install/health_check/deploy_check.py:bkmonitorv3()` 又只校验依赖、不查 systemd, 导致这块巡检空白。

bkmonitorv3 在 bk.env 真实拓扑下的子模块来自 [reference/bk-install/bin/install_bkmonitorv3.sh:27](reference/bk-install/bin/install_bkmonitorv3.sh:27) 的 `PROJECTS=(influxdb-proxy transfer grafana monitor unify-query ingester)`。本 change 仅做前 4 个 + monitor, 排除 grafana/ingester。

## Goals / Non-Goals

**Goals:**
- 让报告里能看到 bkmonitorv3 各子模块的 systemctl/healthz/worker 状态。
- 从模块视角校验 bkmonitorv3 依赖能否连通(对齐 deploy_check.py)。
- 加入 ZK / InfluxDB 的轻量探活能力, 用最小成本闭环依赖检查。

**Non-Goals:**
- 不做 grafana / ingester 子模块。
- 不把 ZooKeeper / InfluxDB 升级为通用中间件 collector(无主从、无慢日志维度需求)。
- 不做 cmdb / paas / job 等其它模块的依赖连通性采集 — 本期只覆盖 bkmonitorv3。
- 不修改 deploy_check.py 同等的 ZK 节点遍历(deploy_check 本身也只检 host 字符串拼成的连接串)。

## Decisions

### D1. 子模块进程巡检复用现有 ModuleRegistry, 不开新结构

只在 [collector/service_registry.go](collector/service_registry.go) 增 `"bkmonitorv3": []SubModule{...}`, 在 [config/config.go](config/config.go) 的 `GetModuleHosts()` 加一行 `{Module: "bkmonitorv3", IPs: cfg.MonitorV3IPs}`。service.go 流水线零改动。

**Why**: 这套流水线已经验证过 cmdb/paas/job/iam/usermgr/nodeman, 复用收益最大, 没必要为一个模块开新通道。

**Alternative considered**: 给 bkmonitorv3 写独立 collector — 否决, 重复劳动且降低一致性。

### D2. healthz 类型统一选 `http_alive`

4 个子模块在 bk-install 里都没显式探活路径, 端口能通就视作存活。

**Why**: `http_alive` 在 [collector/service.go:74](collector/service.go:74) 已实现 — 任意非 000 HTTP 码即 OK, `unreachable` 才报错。比 `http_status` 宽容(子模块可能在根路径返回 404), 比 `none` 信息量大。

**Alternative considered**: 逐个调研真实 healthz 路径 — 后续可在真实环境跑一次后再细化为 `http_status` 或 `json_ok`, 不阻塞当前 change。

### D3. monitor 子模块的 ProcessName 取 `supervisord`

`bk-monitor.service` 通过 supervisord 管多个 Python 进程, 没有单一 `monitor` 二进制。

**Why**: ps grep `supervisord` 至少能反映 supervisor 是否在跑(后续 worker 数也以此为准)。直接 grep `monitor` 会撞到太多无关进程, grep `bk-monitor` 在 ps 里看不到。

**Trade-off**: worker 数语义略弱(只看 supervisord 自身, 不看子进程), 接受。

### D4. 模块依赖采集器在中控本机执行, 不走 SSH

新增 `collector/bkmonitor_dep.go`, 内部串行调用 `mysql/redis-cli/curl/nc` 在本机探各依赖端点, 与 `collector/mysql.go` 等已有中间件 collector 同源同 CLI。

**Why**:
- 与 deploy_check.py 行为一致, 跑一次报告就能跨工具对照。
- 现有 mysql/redis CLI 已是中控本机依赖, 不引入新部署要求。
- 走 SSH 就要在远端有同样 CLI, 多 host 还要去重, 复杂度收益比不划算。

**Alternative considered**: 在 `BK_MONITORV3_IP0` 主机上 SSH 探依赖 — 反映服务真实视角更准, 但成本明显更高, 留作后续扩展。

### D5. ZK / InfluxDB 用最轻量的探活, 不抽象为独立 collector

- ZK: `echo ruok | nc <host> <port>` 期待 `imok`。
- InfluxDB: `curl -s -o /dev/null -w "%{http_code}" http://host:port/ping`, 期望 204。

**Why**: 项目里没有其它消费这俩组件的诉求, 抽象成 collector 没价值。函数挨在 `bkmonitor_dep.go` 内即可, 后续若要扩展再上提。

**Risk**: `nc` 在极简发行版可能缺失 → 当 nc 不存在时该项 `Status="unreachable"`, `Detail="nc not available"`, 不让整轮巡检失败。

### D6. 依赖结果结构: 七元组扁平表

```go
type DependencyResult struct {
    Item     string  // "redis", "mysql-paas", "mysql-monitor", "rabbitmq", "zookeeper", "es7", "influxdb"
    Endpoint string  // "10.97.20.17:6379"
    Status   string  // "ok" | "fail" | "unreachable" | "skip"
    Detail   string  // 错误摘要或版本信息
}
```

**Why**: 扁平结构最容易渲染表格、最容易序列化 JSON、最容易扩展(将来给 cmdb/paas 复用)。

### D7. 缺失凭据时 Status = `skip`, 不视为失败

阈值层面也按 `skip` 不计入失败计数。

**Why**: 部分环境可能不部署某些组件(例如不开启某条 Redis), 与"配置错误"语义不同。

## Risks / Trade-offs

- **healthz 误报**: `http_alive` 对 `transfer` 等服务过于宽松, 端口被某无关进程占用也会算 OK。
  → 缓解: ProcessName grep + systemctl Status 双重信号, 三者综合人工可辨。后续真实数据出来再升级到 `http_status`。
- **monitor supervisord ProcessName**: 与其它子模块语义不齐(其它是直接二进制)。
  → 缓解: 在报告渲染时这一行的 worker 数标注 "supervisord 进程数"。
- **InfluxDB 用 BK_INFLUXDB_IP0**: deploy_check.py 也是这么写, 多实例时仅探第一个。
  → 缓解: 接受现状, 与 deploy_check 一致即可。如有诉求另起 change 改为遍历。
- **MySQL 登录探活会留下连接日志**: 频繁巡检可能在 MySQL `general_log` 留痕。
  → 缓解: 现有 `collector/mysql.go` 已经在做同样事, 不引入新风险。

## Migration Plan

无数据迁移。新加的 `Config` 字段对老 bk.env(没有 bkmonitorv3)是空切片/空串, 全程跳过, 行为零差异。
回滚: revert change 即可, 报告字段是新增字段, 不影响老报告解析。

## Open Questions

- **真实环境 healthz 路径**: 第一次跑通后, 是否要把 `http_alive` 升级为 `http_status` + 真路径? 留作后续 PR。
- **ingester 端口**: 后续若要补 ingester, 需在 supervisor 配置或运行时确认其 HTTP 端口(env 没明示)。
