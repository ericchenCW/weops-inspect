## ADDED Requirements

### Requirement: bkmonitorv3 模块进程巡检

系统 SHALL 在 `ModuleRegistry` 中注册 `bkmonitorv3` 模块, 包含以下 4 个子模块, 并复用现有 service.go 流水线对每个子模块按 host 采集 systemctl 状态、healthz、worker 数:

| Sub-Module      | ServiceUnit          | ProcessName       | Port  | HealthzType  |
|-----------------|----------------------|-------------------|-------|--------------|
| influxdb-proxy  | bk-influxdb-proxy    | influxdb-proxy    | 10203 | http_alive   |
| transfer        | bk-transfer          | transfer          | 10202 | http_alive   |
| monitor         | bk-monitor           | supervisord       | 10204 | http_alive   |
| unify-query     | bk-unify-query       | unify-query       | 10206 | http_alive   |

`grafana` 与 `ingester` 子模块本期不采集。

#### Scenario: bkmonitorv3 模块单 host 采集

- **WHEN** `BK_MONITORV3_IP_COMMA=10.97.20.18` 且 SSH 可达
- **THEN** 报告中 `Services["bkmonitorv3"]` 包含 1 条 host 结果, 内含 4 个子模块的 `Status / MainPID / StartTime / HealthzAPI / Workers` 字段

#### Scenario: bkmonitorv3 模块多 host 采集

- **WHEN** `BK_MONITORV3_IP_COMMA=10.97.20.18,10.97.20.19`
- **THEN** 报告中 `Services["bkmonitorv3"]` 包含 2 条 host 结果, 每条都覆盖 4 个子模块

#### Scenario: bkmonitorv3 模块未部署

- **WHEN** `BK_MONITORV3_IP_COMMA` 未设置或为空
- **THEN** 跳过 bkmonitorv3 进程巡检, 不写入报告 `Services["bkmonitorv3"]` 字段, 不报错

### Requirement: bkmonitorv3 子模块 healthz 探活方式

系统 SHALL 对 4 个子模块使用 `http_alive` 类型的 healthz 探活, 即从中控用 SSH 在目标 host 上 `curl http://127.0.0.1:<port>` 与 `curl http://<host>:<port>` 二选一成功即算 OK; 仅当两路径都连不上时记 `unreachable`。

#### Scenario: 端口已监听但无 healthz 路径

- **WHEN** `transfer` 在 `:10202` 监听并对根路径返回任意非 000 HTTP 码
- **THEN** `HealthzAPI = "ok"`

#### Scenario: 端口不监听

- **WHEN** `unify-query` 进程未启动 `:10206` 不监听
- **THEN** `HealthzAPI = "unreachable"`

### Requirement: bkmonitorv3 模块依赖连通性采集

系统 SHALL 新增模块依赖采集器, 在中控本机依次探测 bkmonitorv3 配置的依赖端点, 对齐 `bk-install/health_check/deploy_check.py:bkmonitorv3()` 的检查范围:

- Redis 单点登录 (`BK_MONITOR_REDIS_HOST` / `BK_MONITOR_REDIS_PORT` / `BK_MONITOR_REDIS_PASSWORD`)
- MySQL × 2 登录 (`BK_PAAS_MYSQL_*` 与 `BK_MONITOR_MYSQL_*`)
- RabbitMQ AMQP 登录 (`BK_MONITOR_RABBITMQ_*`)
- ZooKeeper `ruok` 探活 (`BK_GSE_ZK_HOST:BK_GSE_ZK_PORT`)
- Elasticsearch 7 登录 (`BK_MONITOR_ES7_HOST:BK_MONITOR_ES7_REST_PORT` + `BK_MONITOR_ES7_USER/PASSWORD`)
- InfluxDB `/ping` (`BK_INFLUXDB_IP0:BK_MONITOR_INFLUXDB_PORT`)

每项产出 `{Item, Endpoint, Status, Detail}`, 其中 `Status ∈ {ok, fail, unreachable, skip}`。

#### Scenario: 全部依赖正常

- **WHEN** 所有依赖凭据正确且端点可达
- **THEN** 报告 `BKMonitorV3.Dependencies` 包含 7 条结果(MySQL 两条), 每条 `Status="ok"`

#### Scenario: 单项依赖失败

- **WHEN** `BK_MONITOR_RABBITMQ_PASSWORD` 错误, 其它依赖均正常
- **THEN** RabbitMQ 项 `Status="fail"`, `Detail` 记录鉴权失败信息, 其它依赖项不受影响

#### Scenario: 依赖凭据缺失

- **WHEN** `BK_MONITOR_ES7_HOST` 未设置
- **THEN** ES7 项 `Status="skip"`, `Detail="missing config"`, 不阻塞其它探测

#### Scenario: bkmonitorv3 未部署时跳过依赖采集

- **WHEN** `BK_MONITORV3_IP_COMMA` 为空
- **THEN** 不执行依赖连通性采集, `BKMonitorV3.Dependencies` 不出现在报告

### Requirement: ZooKeeper 与 InfluxDB 探活仅服务于本采集器

系统 SHALL 在 bkmonitorv3 依赖采集器内部实现 ZooKeeper `ruok` 探活与 InfluxDB `/ping` 探活, 不将其抽象为独立的中间件 collector, 也不进入 `Config.*IPs` 主机集合。

#### Scenario: ZK 探活实现位置

- **WHEN** 实现完成
- **THEN** `collector/bkmonitor_dep.go`(或同名文件) 中存在 ZK / InfluxDB 探活函数, 且 `collector/` 下不新增 `zookeeper.go` / `influxdb.go` 通用文件

### Requirement: 报告与渲染包含 bkmonitorv3 段

系统 SHALL 在 `model.Report` 中新增 `BKMonitorV3 *BKMonitorV3Section` 字段, 其下含 `Dependencies []DependencyResult`; HTML 模板 SHALL 在子模块进程表格之后追加 "bkmonitorv3 依赖连通性" 段落渲染该结果。

#### Scenario: 报告 JSON 结构

- **WHEN** 巡检结束写出 `weops_inspection.json`
- **THEN** 顶层包含 `BKMonitorV3.Dependencies` 数组, 每元素含 `Item / Endpoint / Status / Detail`

#### Scenario: HTML 渲染

- **WHEN** 打开 `weops_inspection.html`
- **THEN** 页面存在标题为 "bkmonitorv3 依赖连通性" 的表格, 至少包含 `Item / Endpoint / Status / Detail` 四列
