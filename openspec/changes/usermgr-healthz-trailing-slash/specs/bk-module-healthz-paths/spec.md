## ADDED Requirements

### Requirement: usermgr healthz 路径必须带尾斜杠

`ModuleRegistry["usermgr"]` 的 `HealthzPath` SHALL 为 `/healthz/`(带尾部斜杠),`HealthzType` SHALL 为 `http_status`,期望 HTTP 200。

理由:usermgr 后端基于 Django + gunicorn,无尾斜杠的 `/healthz` 返回 301 重定向,无法被 `http_status` 判定为 ok。

#### Scenario: 健康响应

- **WHEN** usermgr 进程在 `:8009` 监听并对 `/healthz/` 返回 200
- **THEN** 巡检报告中 usermgr 主机的 `HealthzAPI` 字段为 `ok`

#### Scenario: 服务停止

- **WHEN** usermgr 进程未启动,8009 端口不监听
- **THEN** 巡检报告中 usermgr 主机的 `HealthzAPI` 字段为 `unreachable`

### Requirement: BK 模块 healthz 路径以 ModuleRegistry 为唯一来源

任何 BK 业务模块(paas / cmdb / iam / usermgr / nodeman / job / bkmonitorv3-* 等)的 healthz 探测 URL SHALL 仅由 `collector/service_registry.go` 中 `ModuleRegistry` 的 `HealthzPath` 字段决定,巡检流水线 SHALL NOT 在 `service.go` 内对特定模块写死特例 URL。

#### Scenario: 新增模块走 registry

- **WHEN** 团队需要为某 BK 模块加 healthz 探测
- **THEN** 仅在 `ModuleRegistry` 中追加 SubModule 条目即可,无需修改 `service.go`
