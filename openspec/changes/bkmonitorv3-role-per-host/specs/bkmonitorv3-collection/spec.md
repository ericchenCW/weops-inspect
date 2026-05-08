## MODIFIED Requirements

### Requirement: bkmonitorv3 模块进程巡检

系统 SHALL 在 `ModuleRegistry` 中以**每个子角色一个独立 module 键**的形式注册 bkmonitorv3,各子角色的部署主机由其专属 IP 列表决定,允许同一物理集群中各子角色分布在不同主机:

| Module 键                      | Sub-Module      | ServiceUnit          | ProcessName       | Port  | HealthzType  | IP 来源                                    |
|--------------------------------|-----------------|----------------------|-------------------|-------|--------------|--------------------------------------------|
| `bkmonitorv3-monitor`          | monitor         | bk-monitor           | supervisord       | 10204 | http_alive   | `BK_MONITORV3_MONITOR_IP_COMMA`            |
| `bkmonitorv3-influxdb-proxy`   | influxdb-proxy  | bk-influxdb-proxy    | influxdb-proxy    | 10203 | http_alive   | `BK_MONITORV3_INFLUXDB_PROXY_IP_COMMA`     |
| `bkmonitorv3-transfer`         | transfer        | bk-transfer          | transfer          | 10202 | http_alive   | `BK_MONITORV3_TRANSFER_IP_COMMA`           |
| `bkmonitorv3-unify-query`      | unify-query     | bk-unify-query       | unify-query       | 10206 | http_alive   | `BK_MONITORV3_UNIFY_QUERY_IP_COMMA`        |

`grafana` 与 `ingester` 子模块不采集。

向后兼容:若**任一**角色专属 IP 列表为空,则回退至 `BK_MONITORV3_IP_COMMA` 作为该角色的部署主机;若 `BK_MONITORV3_IP_COMMA` 也为空,则跳过该角色采集。

#### Scenario: 角色分布在不同主机

- **WHEN** `BK_MONITORV3_MONITOR_IP_COMMA=10.10.26.235`、`BK_MONITORV3_INFLUXDB_PROXY_IP_COMMA=10.10.26.235`、`BK_MONITORV3_TRANSFER_IP_COMMA=10.10.26.236`、`BK_MONITORV3_UNIFY_QUERY_IP_COMMA=10.10.26.236`
- **THEN** 报告 `Services["bkmonitorv3-monitor"]` 与 `Services["bkmonitorv3-influxdb-proxy"]` 仅含 `.235` 主机记录;`Services["bkmonitorv3-transfer"]` 与 `Services["bkmonitorv3-unify-query"]` 仅含 `.236` 主机记录;两台主机都**不**对其上不存在的角色做 systemctl/healthz 探测

#### Scenario: 旧配置兼容

- **WHEN** 仅设置 `BK_MONITORV3_IP_COMMA=10.97.20.18`,未设置任何 `BK_MONITORV3_*_IP_COMMA`
- **THEN** 4 个角色都把 `[10.97.20.18]` 当作部署主机,在该主机上分别采集 4 个 sub-module(等价于本 change 之前的行为)

#### Scenario: 单角色未部署

- **WHEN** `BK_MONITORV3_TRANSFER_IP_COMMA` 与 `BK_MONITORV3_IP_COMMA` 都未设置,但其它三个角色 IP 已配置
- **THEN** 报告中 `Services["bkmonitorv3-transfer"]` 不出现,其它三个角色按各自 IP 正常采集

#### Scenario: ingester 不采集

- **WHEN** 部署清单中存在 `monitorv3(ingester)` 角色
- **THEN** 巡检工具不为 ingester 注册 sub-module,不出现在 `ModuleRegistry`,不出现在报告中,不报错
