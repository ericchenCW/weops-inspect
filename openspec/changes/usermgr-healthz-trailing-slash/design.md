## Context

usermgr 是基于 Django + gunicorn 的服务,Django 默认对未尾斜杠的 URL 返回 301 重定向到带尾斜杠版本(`APPEND_SLASH=True`)。当前巡检 `HealthzPath="/healthz"` 命中第一跳 301,被 `http_status` 判定逻辑视为非 200 → fail。

此 change 是一个一行字符串修复,但顺势引入 `bk-module-healthz-paths` capability spec,让"模块 → healthz 路径"这一隐性契约显式化,后续有类似 redirect/路径不一致问题时有 spec 锚点可改。

## Goals / Non-Goals

**Goals:**
- 让 usermgr 巡检的 `HealthzAPI` 列显示 `ok` 而非 `301`。
- 把"模块 healthz 路径"这一契约写进 spec。

**Non-Goals:**
- 不改 `http_status` 判定语义(不让它接受 3xx)。理由:`http_status` 的语义是"模块自报 200",2xx/3xx 一锅端会掩盖真实异常。
- 不批量审计其它模块是否也有 redirect 问题(留作后续单独 change)。
- 不改 usermgr `HealthzType`(不切到 `json_ok` 解析 `alive` 字段),理由:200 本身已经足够表达健康,JSON 字段格式是 usermgr 内部协议,巡检不应耦合。

## Decisions

### D1: 改路径而非改判定逻辑

两种修法:

| | 改动 | 取舍 |
|---|---|---|
| **A. `HealthzPath: "/healthz/"`**(选定) | 1 行字符串 | 精确,不影响其它模块 |
| B. `http_status` 接受 2xx/3xx | 改 service.go 判定分支 | 可能掩盖其它模块的 redirect 异常 |

选 A。

### D2: 引入新 capability `bk-module-healthz-paths`

OpenSpec 要求 spec 改动有落点。当前没有 spec 描述"模块 → healthz path"的契约。新建一个轻量 capability 用作锚点,内容仅包含 usermgr 一条规则 + 一条通用约束(以 ModuleRegistry 为唯一来源)。后续模块若有类似问题,只需在此 capability 下追加 ADDED 条目。

## Risks / Trade-offs

- [若 usermgr 后端将来去掉 `APPEND_SLASH` 或改路径] → 需要再次同步,但这是一行修改,运维成本低。
- [新建 capability 的"轻量化"定位可能与现有 capability 粒度不一致] → 接受。capability 的存在意义是为变更提供 spec 落点,不是按代码体量切分。
