## Why

`CollectBKMonitorV3Deps` 当前两处与生产环境实际不符,导致依赖体检"假阳性":

1. **MySQL 探测** 分别从 `BK_PAAS_MYSQL_*` 与 `BK_MONITOR_MYSQL_*` 读 host/port,但蓝鲸生产实际上只跑一套 MySQL,通过 Consul 服务发现暴露为 `mysql.service.consul:3306`。两套 env 大部分情况下其实指向同一个端点,且部分新部署里 `BK_*_MYSQL_HOST` 干脆没有,导致 probe 直接 `skip`。
2. **ZooKeeper 探测** 用 `ruok` 4lw 命令,但蓝鲸 ZK 默认 `4lw.commands.whitelist` 只放 `srvr`,`ruok` 被禁,probe 一律返回 `fail: no ruok response (4lw disabled?)`,但 ZK 本身是健康的。

## What Changes

- **MySQL**:把 `mysql-paas` / `mysql-monitor` 两条合并为单条 `mysql`。host 默认 `mysql.service.consul`、port 默认 `3306`,允许 env 覆盖。
  - 用户名密码仍走 env(优先 `BK_MONITOR_MYSQL_USER/PASSWORD`,回退 `BK_PAAS_MYSQL_USER/PASSWORD`)。
  - `Config.BKMonitorV3` 中 `PaaSMySQL*` / `MonitorMySQL*` 字段合并为单组 `MySQLHost/Port/User/Password`。
- **ZooKeeper**:去掉 `ruok` 协议握手,改为纯 TCP `DialTimeout` 即视为可达。`Detail` 写明"TCP-only probe; 4lw whitelist may exclude ruok"。
- **报告字段**:`DependencyResult.Item` 集合从 `{redis, mysql-paas, mysql-monitor, rabbitmq, zookeeper, es7, influxdb}` 变为 `{redis, mysql, rabbitmq, zookeeper, es7, influxdb}`。**BREAKING(报告语义)**:消费 `Dependencies[*].Item` 的下游需调整。

## Capabilities

### New Capabilities
- (无)

### Modified Capabilities
- `bkmonitorv3-collection`: 依赖采集 MySQL 合并、ZK 探测方式改纯 TCP。
- `bk-config-loading`: `BKMonitorV3DepConfig` 字段调整(MySQL 合并)。

## Impact

- **代码**
  - [collector/bkmonitor_dep.go](collector/bkmonitor_dep.go) — `probeMySQLLogin` 调用减少为 1 处;`probeZookeeper` 实现简化。
  - [config/config.go](config/config.go) — `BKMonitorV3DepConfig` 结构变化,`Load()` 解析逻辑调整。
- **行为**
  - 依赖结果数从 7 → 6。
  - ZK 项不再因 4lw whitelist 误报。
- **依赖 / 接口**:报告 JSON `BKMonitorV3.Dependencies[*].Item` 取值集合变化。
