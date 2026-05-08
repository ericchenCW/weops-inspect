## 1. HTML 片段提取

- [x] 1.1 在 `notify` 包新增 `extractBodyFragment(html string) (body, style string, ok bool)`，使用 `strings.Index` 定位 `<style>...</style>` 和 `<body ...>...</body>`，找不到时返回 `ok=false`
- [x] 1.2 为 `extractBodyFragment` 添加单元测试: 完整 HTML 文档（典型 `weops_inspection.html` 片段）、缺失 `<body>`、缺失 `<style>`、空字符串、`<body>` 带属性等场景

## 2. 邮件发送层接入 HTML alternative

- [x] 2.1 修改 `notify/email.go::Send()` 签名: 增加 `htmlBody string` 参数（位置在 `body` 之后、`attachmentPath` 之前）
- [x] 2.2 在 `Send()` 中: 当 `htmlBody != ""` 时调用 `msg.AddAlternativeString(mail.TypeTextHTML, htmlBody)`；保留原 `SetBodyString(TypeTextPlain, ...)` 与 `AttachFile`
- [x] 2.3 检查 go-mail API: 确认 `AddAlternativeString` 与 `AttachFile` 同时使用时生成 `multipart/mixed > multipart/alternative + attachment` 结构正确

## 3. Process 编排与降级

- [x] 3.1 修改 `notify/notify.go::Process()`: 在 `Send` 调用前读取 `htmlPath` 文件并调用 `extractBodyFragment`
- [x] 3.2 拼接 HTML 正文: `<style>{style}</style>\n{body}`（style 可为空），失败时 `htmlBody = ""` 并 `fmt.Fprintf(os.Stderr, "notify: 解析 HTML 报告失败，退化为纯文本+附件: %v\n", err)`
- [x] 3.3 将 `htmlBody` 传入 `Send()`，告警与恢复两个分支均覆盖
- [x] 3.4 验证现有"已发送告警邮件"等日志路径不变；附件路径不变

## 4. 验证

- [x] 4.1 运行 `go build ./...` 与 `go test ./notify/...`，确保无回归
- [ ] 4.2 手工冒烟: 用本地 SMTP（或 mailtrap/`smtp4dev`）发送一封告警邮件，在 Gmail Web、Apple Mail、Outlook Web 三种客户端中目视确认 HTML 正文显示且附件存在
- [ ] 4.3 故障注入: 临时把 `htmlPath` 改为不存在的路径，确认 stderr 输出 warning 且邮件仍然发出（仅含纯文本 + 无附件，或保留原行为—与现状一致即可）

## 5. 文档与归档

- [x] 5.1 更新 `notify` 包内相关 godoc/注释（`Send` 函数签名变更说明）
- [x] 5.2 实现完成后运行 `openspec validate embed-html-in-alert-email` 通过
