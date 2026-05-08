## Why

当前告警邮件正文是纯文本，HTML 巡检报告仅作为附件随邮件发送。收件人需要先下载附件再用浏览器打开才能看到完整结果，多了两步操作。直接将 HTML 报告嵌入邮件正文，可让收件人在邮件客户端中即刻看到带样式的巡检结果。

## What Changes

- 告警/恢复邮件 SHALL 改为 `multipart/alternative` 结构，同时携带纯文本和 HTML 两个版本
- HTML 版本 SHALL 取自本次生成的 `weops_inspection.html`，仅截取 `<body>` 内部内容嵌入邮件正文
- 原 HTML 附件 SHALL 保留（命名与位置不变），用于存档与不便阅读邮件正文时下载查看
- 纯文本版本 SHALL 维持现状，作为不支持 HTML 的客户端的 fallback
- HTML 读取或解析失败时 SHALL 退化为仅纯文本 + 附件，并在 stderr 打印 warning，不影响主流程

## Capabilities

### New Capabilities
<!-- 无新能力 -->

### Modified Capabilities
- `alert-notification`: 邮件发送结构由 "纯文本正文 + HTML 附件" 升级为 "纯文本 + 内嵌 HTML 正文 + HTML 附件"。"告警邮件结构"与"恢复邮件结构" 两个 Scenario 的正文部分需要更新，并新增 HTML 内嵌相关的 Scenario 与失败降级行为

## Impact

- 代码:
  - `notify/email.go`: `Send()` 签名增加 HTML 正文参数；构造消息时使用 `AddAlternativeString`
  - `notify/notify.go`: `Process()` 中读取 `htmlPath` 文件并提取 `<body>` 内容传给 `Send()`
  - 新增 body 提取小工具（无需引入新依赖，标准库 `strings` 即可，因为模板生成的 HTML 结构稳定）
- 依赖: 无新增（`go-mail` 已支持 alternative parts）
- 邮件体积: 单封邮件体积约翻倍（HTML 正文 + HTML 附件同源）。生产环境报告通常 < 500 KB，可接受
- 兼容性: 现有 SMTP 服务器与所有主流邮件客户端均原生支持 multipart/alternative；不构成 breaking change
