## Context

`notify/email.go::Send()` 当前用 `msg.SetBodyString(mail.TypeTextPlain, body)` 设置纯文本正文，并把 HTML 巡检报告通过 `msg.AttachFile(attachmentPath)` 作为附件。收件人需先下载附件、再用浏览器打开才能看到带样式的报告。

`weops_inspection.html` 由项目模板渲染，结构稳定：标准 `<!DOCTYPE>` + `<html>` + `<head>`（含一个 `<style>`）+ `<body>`。文件无 `<script>`、无外链资源、无内联图片，纯静态 HTML + 内联 CSS，天然适合嵌入邮件。

`go-mail` 库（已是项目依赖）原生支持 multipart 结构：`SetBodyString` 设置主正文后，`AddAlternativeString` 可添加备选格式，库会自动包成 `multipart/alternative`；再叠加 `AttachFile` 时会进一步嵌套为 `multipart/mixed`。

## Goals / Non-Goals

**Goals:**
- 收件人在邮件客户端中直接看到带样式的巡检报告，无需下载附件
- 不支持 HTML 的客户端仍能看到与现状一致的纯文本正文
- HTML 报告作为附件继续保留，便于存档与离线查看
- 读取/解析 HTML 失败时优雅降级为现有行为，不影响巡检主流程

**Non-Goals:**
- 不优化/重写 HTML 模板本身（grid/gradient 等在 Outlook 下的兼容性问题不在本次处理）
- 不做内联 CSS 转换（不引入 premailer 类工具，依赖客户端自身对 `<style>` 块的支持）
- 不调整附件命名、生成逻辑或冷却/状态判定策略
- 不修改 SMTP/TLS/超时等传输层行为

## Decisions

### Decision 1: 只截取 `<body>` 内部内容嵌入，不嵌入整文件

**选择**: 提取 `<body ...>` 与 `</body>` 之间的内容（含原 `<head>` 中的 `<style>`），拼接为 `<style>...</style> + body 内容` 后作为 HTML alternative。

**为什么**:
- 邮件客户端会把整封 HTML 装进自己的 sandbox iframe，外层 `<html>/<head>` 多被剥离或忽略，但保留意义不大
- 直接截 `<body>` 内部内容 + 复制 `<style>` 块，更接近邮件 HTML 的标准做法
- Gmail/Outlook 对完整 `<!DOCTYPE>` 文档的处理不一致，单纯片段反而更稳

**备选**:
- (A) 整文件原样作为 alternative —— 客户端兼容性参差
- (B) 先内联 CSS 再嵌入 —— 引入额外依赖，实现复杂度跳一档，本次不需要

### Decision 2: HTML 提取放在 `notify.Process()`，`Send()` 接收 HTML 字符串

**选择**: `Process()` 读 `htmlPath` 文件并提取片段；`Send()` 签名增加 `htmlBody string` 参数（空字符串表示无 HTML 备选）。

**为什么**:
- `Send()` 当前已经把"路径"作为参数语义弱化（仅做附件），再混入"读文件 + 解析"的职责会让单元测试更难
- `Process()` 是 IO 密集层，集中处理文件读取与降级逻辑（warning + 退化为纯文本）天然合适
- 失败路径单一：`Process()` 读不到/解析不出 → 传空串 → `Send()` 跳过 alternative

**备选**:
- 在 `Send()` 内部读文件 —— 把降级日志也压进去，错误传播链复杂

### Decision 3: 用字符串切片而非 HTML parser

**选择**: 在 `notify` 包内新增小工具 `extractBodyFragment(html string) (fragment, style string, ok bool)`，使用 `strings.Index` 定位 `<style>...</style>` 与 `<body ...>...</body>`。

**为什么**:
- HTML 由项目模板生成，结构稳定可控；不存在嵌套 `<body>` / 转义等病态情况
- `golang.org/x/net/html` 是新依赖，对此体量过重
- 失败时返回 `ok=false`，由调用方决定降级，不抛 panic

**备选**:
- 引入 `golang.org/x/net/html` —— 严谨但杀鸡用牛刀
- 正则 —— 比字符串索引更慢且更难读

### Decision 4: 附件保留，邮件体积约翻倍可接受

**选择**: 不去重，HTML 同时作为正文和附件。

**为什么**:
- 用户明确要求保留附件
- 生产环境 HTML < 500 KB，邮件体积约 1 MB 量级，远低于主流邮箱 25 MB 限制
- 附件提供"完整保真"备份，正文提供"即时可读"

## Risks / Trade-offs

- **[邮件体积翻倍]** → 缓解: 生产环境 HTML 体积可控；如未来出现 RabbitMQ 巡检环境下的 9 MB 异常报告，由 HTML 模板侧治理（不属本次范围）
- **[Outlook 渲染降级]** → 缓解: 当前 HTML 用到 grid / gradient / `:nth-child` 等 Outlook 不支持特性，邮件正文在 Outlook 下会塌成"朴素版"但内容完整；附件仍可在浏览器打开。本次不引入 inline-CSS 转换
- **[模板结构变化打断片段提取]** → 缓解: `extractBodyFragment` 失败时降级为纯文本 + 附件并打 warning，不影响主流程；新增单元测试覆盖典型片段以提前发现回归
- **[纯文本与 HTML 内容不一致]** → 缓解: 两者都基于同一 `report` 与 `items`，源头一致；纯文本仍由 `BuildAlertBody` 生成，HTML 直接复用磁盘上的报告文件
