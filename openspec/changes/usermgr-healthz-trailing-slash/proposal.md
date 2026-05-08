## Why

usermgr 的 `/healthz` 在 gunicorn/Django 后端会返回 `301 Moved Permanently`,`Location: /healthz/`。当前 [collector/service_registry.go](collector/service_registry.go) 把 `usermgr` 的 `HealthzPath` 配为 `/healthz`,而 [collector/service.go](collector/service.go) `http_status` 类型只把 `200` 判为 `ok`,`301` 落入 `else` 分支被记为 fail。

实测:

```
GET /healthz   → 301 (Location: /healthz/)
GET /healthz/  → 200 {"results": [{"system_name": "default-db", "alive": true, "issues": []}]}
```

只需把 `HealthzPath` 改为带尾斜杠的 `/healthz/`,即可拿到稳定的 200。

## What Changes

- `ModuleRegistry["usermgr"][0].HealthzPath` 从 `/healthz` 改为 `/healthz/`。
- `HealthzType` 维持 `http_status`,200 → ok 的判定保持不变。
- 新增 capability spec `bk-module-healthz-paths`,锚定 BK 模块的 healthz URL 路径(目前先列 usermgr 一条),为后续模块路径调整提供 spec 落点。

## Capabilities

### New Capabilities
- `bk-module-healthz-paths`: 定义各 BK 业务模块的 healthz HTTP 路径与判活方式,作为 `ModuleRegistry` 的契约来源。

### Modified Capabilities
- (无)

## Impact

- **代码**
  - [collector/service_registry.go](collector/service_registry.go):一行字符串改动。
- **行为**
  - usermgr 主机巡检 `HealthzAPI` 列从 `301` 变为 `ok`。
  - 不影响其它模块。
- **依赖 / 接口**:无。
