## 1. 阈值配置

- [x] 1.1 在 `config/config.go` 的 `Thresholds` 结构体中新增 `RabbitMQQueueBacklog int` 字段
- [x] 1.2 在 `Config.Load()` 阈值解析处增加对 `INSPECT_RABBITMQ_QUEUE_BACKLOG_THRESHOLD` 的读取,默认 `10000`,非法数字返回错误且错误信息包含变量名
- [x] 1.3 沿现有阈值默认值定义位置,把 `RabbitMQQueueBacklog` 默认值与 `MySQLReplLagSec` / `RedisReplIOSec` 等同位置补齐

## 2. 采集层过滤与阈值接入

- [x] 2.1 修改 `collector/rabbitmq.go` 的 `rmqProbe.Run`,在队列遍历开头新增过滤:`vhost == "bk_usermgr"` 或 `strings.HasPrefix(name, "celeryev")` 时 `continue`
- [x] 2.2 把硬编码的 `msgs > 1000` 替换为 `msgs >= cfg.Thresholds.RabbitMQQueueBacklog`(注意从 `>` 改为 `>=` 以匹配 spec 中"等于阈值时进入"的语义)
- [x] 2.3 由于 `rmqProbe` 当前未持有 `cfg`,在 `CollectRabbitMQ` 构造 `rmqProbe` 时把阈值或整个 `cfg` 作为字段注入,保持与其他 probe 一致
- [x] 2.4 保留 `NoConsumerQueues` 现有判定 `consumers == 0 && msgs > 0` 不变
- [x] 2.5 确认过滤逻辑同时作用于 `ExceedingQueues` 与 `NoConsumerQueues`(即 `continue` 写在两处判定之前)

## 3. HTML 模板更新

- [x] 3.1 在 `render/templates/opensources.html.tmpl` 中,把"消息积压队列 (>1000)"标题改为与新默认阈值一致(例如"消息积压队列 (≥10000)"),或改为不包含具体数值的"消息积压队列"以避免日后阈值变更时误差
- [x] 3.2 在"消息积压队列"表后新增"无消费者队列"表,字段:VHost / Queue / MessageCount / Consumers,使用与其他表一致的样式
- [x] 3.3 当 `RabbitMQ.NoConsumerQueues` 为空时不渲染该表,与现有 `ExceedingQueues` 渲染一致

## 4. spec 文件归档前更新

- [x] 4.1 验证 `openspec/changes/rabbitmq-queue-backlog-filtering/specs/infra-component-collection/spec.md` 中所有 Requirement 与本次实现一致
- [x] 4.2 验证 `openspec/changes/rabbitmq-queue-backlog-filtering/specs/threshold-config/spec.md` 与实现一致

## 5. 自测

- [ ] 5.1 准备本地 mock 或 staging 环境,验证 `bk_usermgr` 与 `celeryev*` 队列不出现在生成的 HTML 报告中
- [ ] 5.2 验证设置 `INSPECT_RABBITMQ_QUEUE_BACKLOG_THRESHOLD=5000` 后,5000+ 条消息的队列被列入积压表
- [ ] 5.3 验证 `INSPECT_RABBITMQ_QUEUE_BACKLOG_THRESHOLD=abc` 时进程启动失败,错误信息含变量名
- [ ] 5.4 验证有 `consumers=0 && messages>0` 的真实业务队列出现在"无消费者队列"表中
- [x] 5.5 `go build ./...` 通过
