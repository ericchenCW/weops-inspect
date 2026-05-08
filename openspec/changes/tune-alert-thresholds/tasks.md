## 1. 调整默认阈值

- [x] 1.1 修改 `config/config.go` 中 `INSPECT_CPU_THRESHOLD` 默认值: 75 → 95
- [x] 1.2 修改 `INSPECT_MEM_THRESHOLD` 默认值: 75 → 95
- [x] 1.3 修改 `INSPECT_DISK_THRESHOLD` 默认值: 75 → 95
- [x] 1.4 修改 `INSPECT_INODE_THRESHOLD` 默认值: 75 → 95
- [x] 1.5 修改 `INSPECT_MAX_OPEN_FILES` 默认值: 102400 → 65536
- [x] 1.6 同步更新 `config/config_test.go` 中默认值断言

## 2. 移除 RunDays 告警

- [x] 2.1 删除 `config/config.go` 中 `INSPECT_RUN_DAYS` 解析与 `Thresholds.RunDays` 字段
- [x] 2.2 删除 `model/Thresholds`(或同等位置)中的 `RunDays` 字段
- [x] 2.3 删除 `checker/rules.go:73-78` 中 RunDays 判定块
- [x] 2.4 检查 `render/`、`output/`、`notify/` 是否引用 `Thresholds.RunDays`, 全部清理
- [x] 2.5 保留 `HostMetrics.RunDays` 采集字段(若已存在), 验证 render 输出仍正常显示运行天数

## 3. ServiceSpec 新增字段

- [x] 3.1 在 `collector/service_registry.go` 的 `ServiceSpec` struct 中新增 `HealthzPort int` 字段(注释说明: 0 = 沿用 Port)
- [x] 3.2 新增 `SkipStatusCheck bool` 与 `SkipHealthzCheck bool` 字段(注释说明默认 false 即原行为)

## 4. job-gateway healthz 端口修复

- [x] 4.1 在 `service_registry` 中 `job-gateway` 的 entry 上配置 `HealthzPort: 19876`
- [x] 4.2 修改 `collector/service.go` 拼 healthz URL 处, 引入 `hzPort := sub.HealthzPort; if hzPort == 0 { hzPort = sub.Port }`, 在 `http_status` / `http_alive` / `json_ok` / `json_up` 各分支中用 `hzPort` 替代 `sub.Port`
- [ ] 4.3 在测试环境验证 job-gateway 的 healthz 检查项变为 `ok`(不再是 fail)

## 5. job-analysis 跳过检查

- [x] 5.1 在 `service_registry` 中 `job-analysis` 的 entry 上配置 `SkipStatusCheck: true` 与 `SkipHealthzCheck: true`
- [x] 5.2 修改 `collector/service.go`: 拼命令前根据 `sub.SkipStatusCheck` 跳过 `===SVC_<name>===` 段; 根据 `sub.SkipHealthzCheck` 跳过 healthz curl 段
- [x] 5.3 修改 `collector/service.go` 解析阶段: skip 的服务对应字段不赋值(或显式置 `""` / `"N/A"`)
- [x] 5.4 修改 `checker/rules.go` `CheckService`: `sm.Status == ""` 时不输出 status 检查项; healthz 已有 `"N/A"` 路径, 沿用即可
- [ ] 5.5 在测试环境验证 job-analysis 不出现在巡检报告的 status / healthz 检查中, 其它 job-* 服务正常

## 6. RabbitMQ 0 消费者 vhost 黑名单

- [x] 6.1 在 `Thresholds` struct 中新增 `RabbitMQNoConsumerVHostBlacklist []string` 字段
- [x] 6.2 在 `config/config.go` 中新增 env 解析: `INSPECT_RABBITMQ_NO_CONSUMER_VHOST_BLACKLIST`(逗号分隔), 默认 `["bk_bknodeman"]`; 实现"完全替换"语义(env 设置即覆盖默认)
- [x] 6.3 定位 RabbitMQ 队列检查代码(0 消费者判定处), 在判定前用 set 检查 `queue.VHost` 是否在黑名单, 命中则跳过 0 消费者告警
- [x] 6.4 确认队列堆积阈值告警逻辑不受黑名单影响
- [x] 6.5 添加 `config/config_test.go` 用例: 默认值, env 覆盖, 空字符串(等于禁用黑名单), 多 vhost

## 7. 文档与发布说明

- [ ] 7.1 更新 README 中阈值默认值说明(若有)
- [ ] 7.2 在 release notes / CHANGELOG 中明确列出: 默认阈值变化、INSPECT_RUN_DAYS 失效、新增 INSPECT_RABBITMQ_NO_CONSUMER_VHOST_BLACKLIST、job-gateway/job-analysis 行为变化
- [ ] 7.3 提示用户: 仍需 75% 阈值的环境必须显式 export 对应 env

## 8. 端到端验收

- [ ] 8.1 在测试环境跑一次完整巡检, 确认: 无 RunDays 告警; CPU/Mem/Disk/Inode 阈值生效在 95%; MaxOpenFiles 65536 阈值生效; job-gateway healthz=ok; job-analysis 无 status/healthz 项; bk_bknodeman vhost 队列即使 0 消费者也不报警, 但堆积超阈值仍报警
- [ ] 8.2 在 SSH 连不通的主机上验证 host 采集 fallback 行为未受影响
- [x] 8.3 `go build ./...` 与现有 `go test ./...` 全部通过
