## 1. 状态枚举与 Summary 重构

- [x] 1.1 在 `model/types.go` 增加 `StatusUnknown` 与 `StatusNotice` 常量
- [x] 1.2 在 `model.CheckSummary` 增加 `Unknown int` 字段（JSON tag `"unknown"`）
- [x] 1.3 改写 `checker.Summarize`：Total 计 OK+Warn+Unknown，Notice 不计入任何桶
- [x] 1.4 更新 `main.go` 收尾打印为"共 N 项检查, X 正常, Y 告警, Z 未知"
- [x] 1.5 更新 `summary.html.tmpl` 增加 Unknown / Notice 卡片显示

## 2. 渲染结构补 Status 字段

- [x] 2.1 给 `model.ESCluster`、`model.ESNode`、`model.ESNodeReach` 增加 `Status string` 字段
- [x] 2.2 给 `model.RedisStandaloneNode` 增加 `Status string` 字段（区分顶层节点级 Status；Celery/Monitor 队列单元另加 `CeleryQueueStatus` `MonitorQueueStatus`）
- [x] 2.3 给 `model.SentinelNode` 增加 `Status string` 字段；`RedisSentinelStatus.Status` 复用现有字段，但取值范围扩展到 `unknown/notice`（如需）
- [x] 2.4 给 `model.MongoMember` 增加 `Status string` 字段
- [x] 2.5 给 `model.RabbitMQQueue`、`model.RabbitMQAlarm`、`model.RabbitMQVHostSummary` 增加 `Status string` 字段
- [x] 2.6 给 `model.DependencyResult` 增加 `RenderStatus string`（避免与现有 `Status` 业务字段冲突）
- [x] 2.7 给 `model.ServiceModule` 增加 `RenderStatus / HealthzRenderStatus string`；给 `model.DockerSummary` 增加 `ExitedStatus string`
- [x] 2.8 review 所有 collector，确认这些字段在 collector 阶段一律留空（grep 验证）

## 3. 阈值搬迁与 env 解析

- [x] 3.1 在 `config.Thresholds` 增加 `ESHeapPercent / ESRAMPercent / ESUnassignedShards / RedisCeleryQueue / RedisMonitorQueue / ServiceContainersExited`
- [x] 3.2 写默认值（85 / 95 / 0 / 1000 / 10000 / 0）
- [x] 3.3 在 `config.Load()` 增加 6 个 env 变量解析（参照现有 `INSPECT_MYSQL_REPL_LAG_THRESHOLD` 的非法数字硬退出处理）
- [x] 3.4 更新 `config_test.go`：默认值用例 + env 覆盖用例 + 非法数字用例

## 4. CheckService 修改与 Service 段补检

- [x] 4.1 改写 `checker.CheckService`：`Status==""` → 产出 Unknown 而不是跳过
- [x] 4.2 在 `checker.CheckService` 增加 Docker `ContainersExited > Threshold` → Notice
- [x] 4.3 在 `main.go` 服务循环中遇到 `ServiceStatuses.Error` 非空时，append 一条 Notice CheckResult（field 形如 `service.{module}.collect_error`）
- [x] 4.4 更新 `checker/rules_test.go`：空 Status 用例改断言 Unknown；新增 Docker 退出 Notice 用例

## 5. 新增 CheckES

- [x] 5.1 创建 `checker/es.go`，实现 `CheckES(clusters []model.ESCluster, t Thresholds) []model.CheckResult`
- [x] 5.2 处理 `cluster.Error` → Warn；同时回填 `cluster.Status = "warn"`，并跳过该集群下属节点 reachability 检查
- [x] 5.3 处理节点 `Status == "unreachable"` → Warn；回填节点 Status
- [x] 5.4 处理 `UnassignedShards > Threshold` → Notice；回填 cluster Status 单元
- [x] 5.5 处理 `HeapPercent > Threshold` / `RAMPercent > Threshold` → Notice；回填节点 Status
- [x] 5.6 处理 `cluster.status != "green"`（Error 为空时）→ Notice
- [x] 5.7 处理 `pending_tasks > 0` → Notice
- [x] 5.8 写 `checker/es_test.go` 覆盖以上 7 个分支

## 6. 新增 CheckRedis（单点 + 哨兵）

- [x] 6.1 创建 `checker/redis.go`，实现 `CheckRedis(...)` 处理 RedisStandalone：节点 Error → Warn；CeleryQueue/MonitorQueue 超阈值 → Notice
- [x] 6.2 实现 Sentinel 检查：`Sentinel.Error` 非空、`MasterReachable==false`、`MasterEnvMatch=="warn"`、`Status != "ok"`、sentinel 节点 `Reachable==false` → 全部 Warn
- [x] 6.3 注意复用现有 `RedisSentinelStatus.Status` 字段而不新增（避免冲突）
- [x] 6.4 写 `checker/redis_test.go`

## 7. 新增 CheckMongo

- [x] 7.1 创建 `checker/mongo.go`，实现 `CheckMongo(m *model.MongoCluster) []model.CheckResult`
- [x] 7.2 `MongoDB.Error` 非空 → Warn
- [x] 7.3 成员 `Health != 1` → Notice
- [x] 7.4 写 `checker/mongo_test.go`

## 8. 新增 CheckRabbitMQ

- [x] 8.1 创建 `checker/rabbitmq.go`，实现 `CheckRabbitMQ(r *model.RabbitMQStatus) []model.CheckResult`
- [x] 8.2 顶层 `Error` / `ClusterPartition` / `AbnormalConnections>0` / `QueuesError` → Warn
- [x] 8.3 节点 `MemAlarm` / `DiskFreeAlarm` → Warn
- [x] 8.4 `ExceedingQueues` 每条 → Warn（field 形如 `rabbitmq.{vhost}.{queue}.backlog`）
- [x] 8.5 `NoConsumerQueues` 每条 → Warn（field 形如 `rabbitmq.{vhost}.{queue}.no_consumer`）
- [x] 8.6 写 `checker/rabbitmq_test.go`

## 9. 新增 CheckBKDeps

- [x] 9.1 创建 `checker/bkdeps.go`，实现 `CheckBKDeps(s *model.BKMonitorV3Section) []model.CheckResult`
- [x] 9.2 `Status == "ok"` → OK；`Status == "skip"` → 不产生；其他 → Notice
- [x] 9.3 写 `checker/bkdeps_test.go`

## 10. main.go 接入新 Checker

- [x] 10.1 在 main.go 第 102 行附近（CheckReplication 之前/之后）依次 append 新 Checker 的产出
- [x] 10.2 确认所有 Status 字段在 Summarize 之前都已回填（顺序：Check 先于 Summarize 先于模板渲染）

## 11. notify.ExtractAlerts 改写

- [x] 11.1 改 `notify/alerts.go`：循环 `report.Hosts[].Checks`、遍历 service 段 / replication 段时，统一只读 `Status == StatusWarn`
- [x] 11.2 删除对 `sm.Status != "active"` / `m.Status != "ok"` 等的二次判定
- [x] 11.3 同步遍历 `report.ES / RedisStandalone / RedisSentinel / MongoDB / RabbitMQ / BKMonitorV3` 中的 CheckResult 列表（如果 checker 把它们放进 allChecks 并通过另一个字段返回；否则需要在 main.go 把 allChecks 整体也存入 report 供 notify 读取）
- [x] 11.4 决定 allChecks 暴露方式：建议在 `model.InspectReport` 增加一个未导出的 `AllChecks []CheckResult` 字段（JSON ignore），ExtractAlerts 直接读它
- [x] 11.5 更新 `notify/signature.go`：签名输入改为 AllChecks 中 Status==Warn 的项；Unknown/Notice 不参与签名
- [x] 11.6 更新 `notify/email.go`：邮件正文 Summary 行展示 Unknown 数；Warn 明细不变；不展示 Unknown/Notice 明细
- [x] 11.7 更新 `notify/signature_test.go`、`notify/trigger_test.go` 适配新结构

## 12. 模板改造（删除内联判断）

- [x] 12.1 在 `render/render.go` 注册 template func `statusClass(string) string`：ok→"status-ok"，warn|notice→"status-warn"，unknown→"status-na"，空→""
- [x] 12.2 重写 `services.html.tmpl`：删除 `{{if eq .Status "active"}}...{{end}}` 等所有内联判断，改为 `{{statusClass .RenderStatus}}` / `{{statusClass .HealthzRenderStatus}}` / `{{statusClass .ExitedStatus}}`
- [x] 12.3 重写 `opensources.html.tmpl`：删除 ES heap/RAM/UnassignedShards、Redis Celery/Monitor、Mongo Health、RabbitMQ AbnormalConnections / 节点告警、MySQL Replication、Redis Role/Link 等所有内联条件着色，改为读 Status 字段
- [x] 12.4 grep 验证：`grep -RnE "{{if (gt|eq) \." render/templates/` 命中数为 0
- [ ] 12.5 跑一次完整巡检，对比升级前后 HTML 视觉差异（除新出现的 Notice 红字外应当一致）

## 13. 文档与 CI 守卫

- [x] 13.1 README 增加"升级注意"章节：首次升级当次会发送较大邮件（含此前漏报项）；列出 6 个新 env 变量与默认值
- [x] 13.2 README 阈值表格增加 6 行新阈值
- [x] 13.3 在 Makefile / CI 增加一条检查：`grep -RnE "{{if (gt|eq) \." render/templates/` 命中即失败（防止后续模板回归内联判断）

## 14. 端到端验证

- [x] 14.1 准备一份固定的样本 `weops_inspection.json`（含 RabbitMQ 积压、ES RAM>95、MongoDB Health 异常、空 Service Status 等多种状态）
- [x] 14.2 跑全链路：collector→checker→render→notify(dryrun)，断言 Summary、HTML 着色、邮件草稿三处 Warn 集合一致
- [ ] 14.3 在 staging 环境实跑一次（关通知），人工 review HTML 与本次案例（job-analysis 空状态、RabbitMQ 360k）是否被正确归类
- [ ] 14.4 开启通知，确认升级当次邮件可读（不超过 200 行；超过的话考虑 Warn 项摘要+附件）
