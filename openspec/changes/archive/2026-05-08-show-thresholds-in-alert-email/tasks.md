## 1. model 层加字段

- [x] 1.1 在 `model.CheckResult` 新增 `Threshold string` 字段（与 `Value` / `Status` 同级），
      JSON tag 加 `,omitempty` 保证向后兼容
- [x] 1.2 更新 godoc：注明 "Threshold 为人类可读的阈值或期望值描述；空字符串表示该规则无
      单一阈值（如关系型规则）"

## 2. checker/rules.go 填充阈值

- [x] 2.1 把现有 `add(field, value, status)` helper 扩展为 `add(field, value, status, threshold)`，
      或新增 `addWithThreshold` 二号 helper（择一，依赖最少改动量为准）
- [x] 2.2 数值类阈值填充（用 `fmt.Sprintf` 拼接 `thresholds.X` 真实数值，避免硬编码）：
      `cpu_usage`, `mem_usage`, `disk_usage`, `inode_usage`, `max_open_files`
- [x] 2.3 状态期望类填充：`selinux`, `firewalld`, `chronyd`，统一格式 `期望 <X>`
- [x] 2.4 服务类填充：`service.*.status` (`期望 active`)、`service.*.healthz` (`期望 ok`)、
      `service.*.docker.exited` (`> <ServiceContainersExited>`)
- [x] 2.5 复制类填充：`mysql_slave.replication` (`lag > <MySQLReplLagSec>s`)、
      `redis.link` (`io > <RedisReplIOSec>s`)
- [x] 2.6 RabbitMQ 队列 backlog 填充：`> <RabbitMQQueueBacklog>`（在产生该结果的 collector
      或 rules 路径上找到落点；no_consumer 因黑名单非单一阈值，保持空）
- [x] 2.7 关系/复杂规则项确认 `Threshold = ""`：`load_average`, `mysql_master.read_only`,
      `redis.role`, `rabbitmq.<vhost>.<queue>.no_consumer`, RabbitMQ 集群级告警
- [x] 2.8 单元测试 `checker/rules_test.go`：每类至少一个 case 断言 Warn 时 Threshold 非空且
      格式预期；OK 路径的 Threshold 不做断言（OK 不进邮件）

## 3. notify 层透传与渲染

- [x] 3.1 `notify.AlertItem` 新增 `Threshold string`
- [x] 3.2 `ExtractAlerts` 在构造 `AlertItem` 时拷贝 `c.Threshold`
- [x] 3.3 `BuildAlertBody` 渲染：当 `it.Threshold != ""` 时在行尾追加 `  (阈值 %s)`；
      为空则保持现状不追加
- [x] 3.4 `signature.go` 增加注释 "Threshold 故意不参与签名计算"；逻辑无改动
- [x] 3.5 单元测试 `notify/email_test.go`（新建或扩展）：覆盖
      "Warn 项带阈值"、"Warn 项无阈值"、"混合"三种场景的正文渲染
- [x] 3.6 单元测试 `notify/signature_test.go`：新增 case，构造两个 AlertItem 集合
      `Field/Host` 一致但 `Threshold` 不同，断言签名相同

## 4. 验证

- [x] 4.1 `go build ./...`
- [x] 4.2 `go test ./...`
- [x] 4.3 手工冒烟：本地构造一份含告警的 InspectReport，触发邮件发送（或打印 BuildAlertBody
      输出），目视确认阈值列展示符合 design 中给的格式样例
- [x] 4.4 `openspec validate show-thresholds-in-alert-email --strict` 通过

## 5. 文档与归档

- [x] 5.1 如 [docs/checks.md](docs/checks.md) 已存在阈值/检查项清单，补充"阈值会展示在告警
      邮件正文"的说明（一句话即可）
- [x] 5.2 `tune-alert-thresholds` 代码任务已交付（默认阈值已是 95% / 65536 等新值），
      本 change 可独立合入，无需等待
