## 1. 配置加载

- [x] 1.1 在 `config/config.go` 新增 `MonitorV3IPs []string` 字段, 从 `BK_MONITORV3_IP_COMMA` 解析
- [x] 1.2 将 `MonitorV3IPs` 纳入 `AllHosts` 去重集合
- [x] 1.3 新增 bkmonitorv3 依赖凭据字段 (Redis / MySQL×2 / RabbitMQ / GseZK / ES7 / InfluxDB), 从对应 `BK_*` env 读取
- [x] 1.4 在 `Config.GetModuleHosts()` 追加 `{Module: "bkmonitorv3", IPs: c.MonitorV3IPs}`

## 2. 子模块进程巡检

- [x] 2.1 在 `collector/service_registry.go` 注册 `bkmonitorv3` 模块, 包含 4 个 SubModule (influxdb-proxy / transfer / monitor / unify-query) 的 unit/process/port/healthz 配置
- [x] 2.2 验证 service.go 流水线对 bkmonitorv3 无需改动 (静态阅读 + 一次空跑)

## 3. 模块依赖采集器

- [x] 3.1 新建 `collector/bkmonitor_dep.go`, 定义 `DependencyResult` 结构与 `CollectBKMonitorV3Deps(cfg) []DependencyResult` 入口
- [x] 3.2 实现 Redis 单点登录探活 (复用 `redis-cli -h ... PING`)
- [x] 3.3 实现 MySQL 登录探活函数, 分别用于 paas/monitor 两套凭据 (复用 `mysql -h ... -e "SELECT 1"`)
- [x] 3.4 实现 RabbitMQ AMQP 鉴权探活 (curl management API `/api/whoami` 或同等手段)
- [x] 3.5 实现 ZooKeeper `ruok` 探活 (`echo ruok | nc host port`, 超时与 nc 缺失分支处理)
- [x] 3.6 实现 Elasticsearch 7 登录探活 (`curl -u user:pass http://host:9200/_cluster/health`)
- [x] 3.7 实现 InfluxDB `/ping` 探活 (期望 204)
- [x] 3.8 任一 host/凭据缺失时返回 `Status="skip"` 且不阻塞后续项
- [x] 3.9 在 `BK_MONITORV3_IP_COMMA` 为空时整体跳过依赖采集

## 4. 报告与渲染

- [x] 4.1 在 `model/` 新增 `BKMonitorV3Section` 与 `DependencyResult` 类型, 接入 `Report`
- [x] 4.2 `main.go` 串入 `CollectBKMonitorV3Deps`, 写入 `Report.BKMonitorV3`
- [x] 4.3 `render/templates/` HTML 模板新增 "bkmonitorv3 依赖连通性" 段落, 渲染 4 列表格
- [x] 4.4 JSON 输出确认顶层包含 `BKMonitorV3.Dependencies`

## 5. 验证

- [ ] 5.1 用现网 bk.env 跑一次 `weops-inspect`, 检查 HTML/JSON 报告中 bkmonitorv3 进程段与依赖段都正确出现
- [ ] 5.2 模拟 `BK_MONITORV3_IP_COMMA` 为空, 确认整段巡检静默跳过
- [ ] 5.3 模拟 `BK_MONITOR_RABBITMQ_PASSWORD` 错误, 确认仅 RabbitMQ 项 fail, 其它依赖项正常
- [x] 5.4 `go vet ./... && go build ./...` 通过
