## 1. 模型扩展

- [x] 1.1 在 [model/types.go](model/types.go) 新增 `ESNodeReach struct { IP, Status, Detail string }`
- [x] 1.2 在 `ESCluster` 结构追加 `NodeReachability []ESNodeReach` 字段(JSON tag 命名一致)

## 2. 采集器并发探测

- [x] 2.1 在 [collector/es.go](collector/es.go) 中 `CollectES` 增加 fan-out:对 `cfg.ES7IPs` 每个 IP 起 goroutine 调 `curlGet("http://<ip>:9200/")`,5s 上限,结果按 index 写入预分配的 `[]ESNodeReach` 切片
- [x] 2.2 等待全部完成后,从顺序中挑首个 `Status="ok"` 的 IP 作为 `host` 与 `Instance`
- [x] 2.3 全部 unreachable 时,设置 `cluster.Error="all nodes unreachable"`、`cluster.ErrorClass=ErrNetwork`,并 `return`(不再调 `_cluster/health`)
- [x] 2.4 把现有 `_cluster/health` 与 `_cat/nodes` 的探测改为基于选中 IP 执行,逻辑保持

## 3. 渲染层

- [x] 3.1 在 ES 章节模板追加 reachability 子表(IP / Status / Detail),仅当 `len(NodeReachability) > 0` 时渲染
- [x] 3.2 视觉上把 `unreachable` 行标红或加 badge,与现有错误行风格一致

## 4. 验证

- [x] 4.1 `go build ./...`、`go vet ./...`
- [ ] 4.2 单元/集成验证:模拟 `[unreachable, ok, ok]` 三节点,确认报告:
  - `ESCluster.Error` 为空
  - `Instance` 是第二个 IP
  - `NodeReachability` 三条全在,首条 unreachable
- [x] 4.3 模拟全部 unreachable,确认 `Error / ErrorClass` 正确(`TestCollectES_AllUnreachable`)
- [ ] 4.4 旧场景回归:单节点 IP 健康时报告与本 change 之前等价(只多出一条 `NodeReachability`)
