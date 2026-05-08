## 1. 版本变量与 CLI flag

- [x] 1.1 在 `main.go` 中新增 `var version = "dev"` 包级变量
- [x] 1.2 在 `main()` 内 `flag.Parse()` 之前注册 `-v` 与 `--version` 两个 `flag.Bool`
- [x] 1.3 解析后若任一为 `true`，打印 `version` 并以退出码 0 返回，跳过巡检逻辑
- [x] 1.4 本地验证：`go build -ldflags "-X main.version=v0.0.0-test"` 后运行 `./weops-inspect -v` 输出 `v0.0.0-test`
- [x] 1.5 本地验证：不带 flag 运行二进制，行为与改动前一致

## 2. GitHub Actions 工作流

- [x] 2.1 创建目录 `.github/workflows/`
- [x] 2.2 新建 `.github/workflows/release.yml`，配置 `on: workflow_dispatch` 与必填输入 `tag`
- [x] 2.3 顶层声明 `permissions: contents: write`
- [x] 2.4 添加 `concurrency: { group: release-${{ inputs.tag }}, cancel-in-progress: false }`
- [x] 2.5 定义 `build` job：`runs-on: ubuntu-latest`，`strategy.matrix.goarch: [amd64, arm64]`
- [x] 2.6 build steps：`actions/checkout@v4`（`ref: ${{ inputs.tag }}`）→ `actions/setup-go@v5`（读取 `go.mod`）→ 交叉编译命令
- [x] 2.7 编译命令：`CGO_ENABLED=0 GOOS=linux GOARCH=${{ matrix.goarch }} go build -trimpath -ldflags "-s -w -X main.version=${{ inputs.tag }}" -o weops-inspect-linux-${{ matrix.goarch }} .`
- [x] 2.8 用 `actions/upload-artifact@v4` 上传二进制，artifact 名带 goarch 后缀
- [x] 2.9 定义 `release` job：`needs: build`，`runs-on: ubuntu-latest`
- [x] 2.10 release steps：`actions/download-artifact@v4`（merge-multiple 或两次下载到同一目录）
- [x] 2.11 探测 Release：`gh release view "$TAG" || gh release create "$TAG" --title "$TAG" --notes ""`
- [x] 2.12 上传资产：`gh release upload "$TAG" weops-inspect-linux-amd64 weops-inspect-linux-arm64 --clobber`
- [x] 2.13 `gh` 步骤通过 `env: GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}` 注入令牌

## 3. 验证

- [ ] 3.1 在 fork 或测试 tag 上手动跑一次 workflow，确认两份二进制成功上传到 Release
- [ ] 3.2 用同一 tag 重跑一次，确认资产被覆盖、无重复
- [ ] 3.3 下载 amd64 二进制，`file weops-inspect-linux-amd64` 应显示 `ELF 64-bit LSB ... x86-64`
- [ ] 3.4 下载 arm64 二进制，`file` 应显示 `ELF 64-bit LSB ... ARM aarch64`
- [ ] 3.5 在对应架构机器上运行 `./weops-inspect -v`，输出与触发的 tag 一致
