## 1. 配置层

- [x] 1.1 在 `Config` 结构新增 `MonitorV3MonitorIPs / MonitorV3InfluxDBProxyIPs / MonitorV3TransferIPs / MonitorV3UnifyQueryIPs []string` 字段
- [x] 1.2 在 `Load()` 中解析对应 4 个 `BK_MONITORV3_*_IP_COMMA` env
- [x] 1.3 在 `buildAllHosts()` 把 4 个新切片加入去重列表
- [x] 1.4 在 `GetModuleHosts()` 把 `bkmonitorv3` 单条记录拆为 4 条 `bkmonitorv3-monitor / -influxdb-proxy / -transfer / -unify-query`,IP 来源带回退到 `MonitorV3IPs`

## 2. 服务注册表

- [x] 2.1 删除 `ModuleRegistry["bkmonitorv3"]` 整组 entry
- [x] 2.2 新增 `bkmonitorv3-monitor / -influxdb-proxy / -transfer / -unify-query` 四个 key,每个包含原对应的单一 SubModule(端口/路径/HealthzType 不变)

## 3. 依赖采集开关

- [x] 3.1 `CollectBKMonitorV3Deps` 的 early-return 条件改为"4 个角色 IP 切片 + `MonitorV3IPs` 全空"才跳过

## 4. 报告渲染

- [x] 4.1 检视 [render/templates/](render/templates/) 中是否有硬编码 `bkmonitorv3` 模块标题/分支(services 模板纯 map-key 驱动,无硬编码;无需改动)
- [x] 4.2 调整模板:对前缀 `bkmonitorv3-` 的 module key 聚合为 "bkmonitorv3" 大节,下分 4 个角色子表;或保留各 key 独立成段(选保留各 key 独立成段)

## 5. 验证

- [x] 5.1 `go build ./...`、`go vet ./...`
- [x] 5.2 单元测试:补充 `config` 包测试覆盖回退路径(只设旧变量、只设新变量、混合)
- [ ] 5.3 集成验证:在分布式部署样例(.235 / .236)上跑一次,确认每台只采自己跑的角色,无 `not-found` / `unreachable` 误报
- [x] 5.4 在 `openspec/project.md` 或 README 添加 4 个新 env 变量的使用说明
