## 1. 实现归一化函数

- [x] 1.1 在 [notify/signature.go](notify/signature.go) 中新增 `normalizeFieldForSignature(field string) string`,使用包级 `regexp.MustCompile` 匹配 `^rabbitmq\.([^.]+)\.(.+)\.(no_consumer|backlog)$`,命中时返回 `rabbitmq.$1.$3`,否则原样返回。
- [x] 1.2 改造 `Signature()`: 构造 keys 时对 Field 调用 `normalizeFieldForSignature`,折叠后用 `map[string]struct{}` 去重,再 `sort.Strings` 后 sha256 求和。
- [x] 1.3 保留空告警列表返回 `""` 的现有行为不变。

## 2. 单元测试

- [x] 2.1 在 [notify/signature_test.go](notify/signature_test.go) 新增"同 vhost 队列名漂移"用例: `bk_bkmonitorv3` 下三组不同队列名(覆盖 22:44/23:02 真实样本)产生相同签名。
- [x] 2.2 新增"同 vhost 队列数量变化"用例: 3 个 → 5 个 `no_consumer` 队列,签名相同。
- [x] 2.3 新增"跨 vhost 告警"用例: 仅 vhost A → vhost A + vhost B,签名不同。
- [x] 2.4 新增"backlog 与 no_consumer 互不合并"用例: 同一 vhost 同一队列名,后缀不同,签名不同。
- [x] 2.5 新增"集群级 Field 不被折叠"用例: `rabbitmq.error`、`rabbitmq.cluster_partition`、`rabbitmq.node.es-1.mem_alarm` 原样进入签名,值不变则签名不变,值变(如新增节点告警)则签名变。
- [x] 2.6 新增"非 rabbitmq Field 不受影响"用例: host CPU、ES heap、redis 等告警在归一化后行为与现状一致。
- [x] 2.7 保留并通过原 `Signature` 全部既有测试,确认无回归。

## 3. 验证与发布

- [x] 3.1 运行 `go test ./notify/...` 全绿。
- [x] 3.2 运行 `go test ./...` 确认其他包(尤其 checker/render)无回归。
- [x] 3.3 在 `docs/` 或 changelog 中记录"告警去重维度收敛"的行为变更与一次性重发说明(运维可见)。
- [x] 3.4 `openspec validate rabbitmq-signature-normalize --strict` 通过。
