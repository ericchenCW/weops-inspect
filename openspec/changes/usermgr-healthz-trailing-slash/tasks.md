## 1. 代码修改

- [x] 1.1 在 [collector/service_registry.go](collector/service_registry.go) 把 `usermgr` 子模块的 `HealthzPath` 从 `/healthz` 改为 `/healthz/`

## 2. 验证

- [x] 2.1 `go build ./...`、`go vet ./...`
- [ ] 2.2 在样例环境跑一次巡检,确认 usermgr 主机的 `HealthzAPI` 列从 `301` 变为 `ok`
- [ ] 2.3 复查报告中其它 BK 模块的 healthz 列,无回归
