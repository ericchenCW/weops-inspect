## 1. Config 结构调整

- [x] 1.1 在 `BKMonitorV3DepConfig` 中删除 `PaaSMySQLHost/Port/User/Password` 与 `MonitorMySQLHost/Port/User/Password` 字段
- [x] 1.2 新增 `MySQLHost / MySQLPort / MySQLUser / MySQLPassword` 单组字段
- [x] 1.3 在 `Load()` 中按优先级 `BK_MONITOR_MYSQL_*` → `BK_PAAS_MYSQL_*` → 默认 `mysql.service.consul:3306` 解析 host/port,账号亦按此顺序回退

## 2. 依赖采集器调整

- [x] 2.1 [collector/bkmonitor_dep.go](collector/bkmonitor_dep.go):删除 `mysql-paas` / `mysql-monitor` 两条 `probeMySQLLogin` 调用,改为单条 `probeMySQLLogin("mysql", dep.MySQLHost, dep.MySQLPort, dep.MySQLUser, dep.MySQLPassword)`
- [x] 2.2 重写 `probeZookeeper`:仅做 `net.DialTimeout("tcp", ..., 5s)`,成功立即关闭并返回 `Status="ok"`,`Detail="TCP-only probe; 4lw whitelist may exclude ruok"`;失败按现状报 `unreachable`
- [x] 2.3 删除 `probeZookeeper` 中已不再使用的 `Read`/`ruok` 相关代码

## 3. 验证

- [x] 3.1 `go build ./...`、`go vet ./...`
- [x] 3.2 单元测试:补充 config 包测试覆盖"MySQL host 默认 / Monitor 覆盖 / PaaS 回退"三种场景
- [ ] 3.3 在样例环境跑一次依赖巡检,确认报告 `BKMonitorV3.Dependencies` 共 6 条,ZK 项为 ok
- [ ] 3.4 在发布说明中标注 `Item` 集合的 BREAKING 变化(7 → 6,删除 `mysql-paas` / `mysql-monitor`,新增 `mysql`)
