## Why

目前 weops-inspect 已经覆盖 paas / cmdb / job / gse / iam / usermgr / nodeman 等模块的进程巡检, 但 **bkmonitorv3 完全缺失** — 既没有子模块 systemctl/healthz 采集, 也没有像 `bk-install/health_check/deploy_check.py` 那样从模块视角校验依赖(Redis / MySQL / RabbitMQ / ZooKeeper / ES7 / InfluxDB)的连通性。这导致蓝鲸监控后台的健康状况无法体现在巡检报告里。

## What Changes

- **新增 bkmonitorv3 模块进程巡检**: 在 `ModuleRegistry` 注册 `influxdb-proxy`, `transfer`, `monitor`, `unify-query` 四个子模块, 走现有 service.go 流水线采集 systemctl 状态、healthz、worker 数。
- **新增"模块依赖连通性"采集器**: 从中控本机用 `bk.env` 中的凭据探 bkmonitorv3 配置的 6 类依赖能否登录/连通, 对齐 `deploy_check.py:bkmonitorv3()`。
- 配置加载新增 `BK_MONITORV3_IP_COMMA` → `Config.MonitorV3IPs`, 以及读取 `BK_MONITOR_*` / `BK_INFLUXDB_*` / `BK_GSE_ZK_*` 一组凭据/端点字段进入 `Config`。
- 报告 model 新增 `BKMonitorV3Dependency` 段, HTML 模板增加对应展示。
- **范围排除**: grafana 与 ingester 子模块本期不做。

## Capabilities

### New Capabilities

- `bkmonitorv3-collection`: 定义 bkmonitorv3 模块的进程级巡检(子模块 systemctl/healthz/worker)与依赖连通性采集(Redis/MySQL×2/RabbitMQ/ZooKeeper/ES7/InfluxDB)的规则。

### Modified Capabilities

- `bk-config-loading`: 新增 `BK_MONITORV3_IP_COMMA` 解析以及 bkmonitorv3 依赖凭据(`BK_MONITOR_REDIS_*`, `BK_MONITOR_MYSQL_*`, `BK_PAAS_MYSQL_*`, `BK_MONITOR_RABBITMQ_*`, `BK_GSE_ZK_*`, `BK_MONITOR_ES7_*`, `BK_INFLUXDB_IP0` + `BK_MONITOR_INFLUXDB_*`)字段进入 `Config`。

## Impact

- 代码:
  - `config/config.go`: 新增字段与 env 解析。
  - `collector/service_registry.go`: 新增 `bkmonitorv3` 注册。
  - `collector/`: 新增 `bkmonitor_dep.go`(模块依赖连通性采集), 内含 ZooKeeper/InfluxDB 轻探活逻辑(不独立成中间件 collector)。
  - `model/`: 新增 `BKMonitorV3Dependency` 结构。
  - `render/templates/`: HTML 模板新增展示段。
  - `main.go`: 串入新采集器。
- 依赖: 中控本机需要 `mysql`, `redis-cli`, `curl`, `nc`(用于 zookeeper `ruok`)。前三者本就是项目运行依赖, `nc` 在多数发行版默认存在; 缺失时该项标记为 `unreachable` 不影响其它采集。
- 行为: 当 `BK_MONITORV3_IP_COMMA` 为空时跳过, 与现有空模块处理一致。
