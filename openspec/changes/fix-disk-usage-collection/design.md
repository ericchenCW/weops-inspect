## Context

巡检工具通过 SSH 在远端执行 `df -h` / `df -i`, 由 [collector/host.go](collector/host.go) 的 `parseDiskUsage()` 解析。当前实现:

1. 配置项 `CHECK_MOUNT_PATH`(默认 `/data`)用冒号分隔, 在 parser 内部以 `pathSet[mountPoint] == true` 做完全相等匹配。
2. 不区分文件系统类型, 但因为只看完全相等的挂载点, tmpfs/overlay 一般不会误命中。
3. 对长设备名导致的 `df` 折行没有处理, 直接被 `len(fields) < 6` 跳过。

实际部署中暴露三类问题:
- 部署主机的实际挂载布局不是 `/data`(典型: LVM 把数据合并到 `/`), 默认配置下 disk_usage 为空。
- 用户没意识到自己漏配, 漏采是静默的, 报告里看不到该主机磁盘信息但也不报错。
- BUILD60 这种容器宿主机有几十条 overlay/tmpfs 行, 一旦未来想"全采", 必须有过滤机制配套, 否则告警会被淹没。

约束:
- 必须保留对存量部署的兼容: 已显式设置 `CHECK_MOUNT_PATH=xxx` 的环境行为不变。
- 远端 shell 仅依赖 `df` 与 coreutils, 不引入 Python/脚本依赖。
- 现有 [model/types.go](model/types.go) 的 `DiskUsage` JSON 字段需保持向后兼容(只新增字段, 不改名/删字段)。

## Goals / Non-Goals

**Goals:**
- 默认配置下能正确采集**所有真实物理/逻辑磁盘**的使用率, 不再漏掉根分区。
- 自动排除 tmpfs/overlay/shm 等噪音文件系统。
- `df` 长设备名折行不再造成漏采。
- 配置不匹配实际挂载时, 在主机条目里给出可见的 warning, 杜绝静默漏采。
- inode 采集与 disk usage 走同一套筛选逻辑, 保持一致性。

**Non-Goals:**
- 不重构整个 host collector 的批处理框架(===TAG=== 分段方案保留)。
- 不引入 IO 吞吐、磁盘延迟等新指标(只解决"哪些挂载点应该被采")。
- 不为单条 mount 配置每挂载点独立阈值(沿用全局 `INSPECT_DISK_THRESHOLD`)。
- 不处理 Windows / 非类 Unix 主机(项目本身只跑 Linux 主机)。

## Decisions

### Decision 1: 默认采集策略 = "文件系统类型白名单 + 黑名单兜底"

`CHECK_MOUNT_PATH` 留空(默认)时, 按 `df -ThP` 的 Type 列筛选:
- 白名单: `xfs, ext2, ext3, ext4, btrfs, zfs, f2fs, ufs, jfs, reiserfs`
- 黑名单(显式排除): `tmpfs, devtmpfs, overlay, squashfs, shm, proc, sysfs, cgroup, fuse.lxcfs, fuse.gvfsd, autofs, binfmt_misc, mqueue, pstore, debugfs, tracefs, ramfs, rpc_pipefs`
- NFS(`nfs`, `nfs4`, `cifs`, `smbfs`)默认排除, 由 `INSPECT_DISK_INCLUDE_NFS=true` 启用。

判定优先级: **黑名单 > NFS 开关 > 白名单**。即使白名单未来扩充, 黑名单的 tmpfs/overlay 永远不会被采。

**为什么不只用白名单?** 防御性: 未来出现的新文件系统类型(如 bcachefs)在白名单更新前应该被采集到, 而不是因为不认识就静默跳过。但黑名单中的"伪文件系统"性质明确, 永远不该被监控。

**为什么不只用黑名单?** 黑名单容易遗漏新出现的伪文件系统类型, 把没见过的东西都当真盘采集会引入噪音。两者结合是常见做法(参见 node_exporter `--collector.filesystem.fs-types-exclude` 默认值)。

**Alternatives considered:**
- 让用户自行写正则: 心智成本高, 默认体验差。
- 依赖 `findmnt --real`: 不是所有发行版都有 / 输出格式有差异。
- 按设备名前缀(`/dev/`)过滤: 漏掉 zfs/btrfs subvolume, 也漏掉 bind mount, 不可靠。

### Decision 2: 显式配置 `CHECK_MOUNT_PATH` 时维持完全相等匹配

如果用户显式设置了 `CHECK_MOUNT_PATH=/var:/data:/home`, 则:
- 仅采集这 3 个挂载点(完全相等, 不做前缀匹配)。
- 文件系统类型黑名单**仍然生效**(防止用户误把 `/run` 之类塞进去)。
- 白名单**不生效**(用户显式列了, 我们尊重选择, 哪怕是 nfs)。

**为什么不引入前缀匹配?** 像 `/data` 在 BUILD60 上会前缀匹配到 12 条 overlay 行, 反而炸开。完全相等是最不会出意外的语义, 与原行为兼容。

**Alternatives considered:**
- 配置项加正则模式开关: 复杂度收益比不划算。
- 引入 `CHECK_MOUNT_PATH_PATTERN` 第二个变量: 双配置项更让人迷惑。

### Decision 3: 远端命令切换到 `df -ThP` 与 `df -iPT`

- `-P`: POSIX 模式, 每个 entry 一行, 不再因长设备名折行。
- `-T`: 输出文件系统类型(白/黑名单判定的依据)。
- `-i`: inode 模式; coreutils 支持 `-iP -T` 组合。

解析时按列定位: POSIX 输出固定为 `Filesystem Type Size Used Avail Use% Mounted-on`(7 列), 跳过表头行, 用列索引取值, 不再依赖"最后两列"的脆弱假设。

### Decision 4: 配置不匹配时追加 warning 到 `Error` 字段

`Error` 字段当前只用于 SSH 失败。新增"采集成功但筛选结果为空"的 warning, 格式:

```
disk: configured mount paths [/data] did not match any of [/, /boot, /boot/efi]
```

**为什么放 `Error` 而不是新字段?** 报告输出/邮件已经会渲染 `Error`, 复用现有渲染路径; 真正的 SSH 失败仍然以 `SSH error:` 前缀区分, checker 不会把"无 disk 数据"和"主机不可达"混淆。后续如果有更多 warning, 再考虑独立的 `Warnings []string` 字段。

**Alternatives considered:**
- 直接 stderr 打印: 跑批量主机时会被噪音淹没。
- 新增 `Warnings` 字段: 改动面更大(报告/邮件渲染都要改), 留作后续。

### Decision 5: `DiskUsage` 结构新增 `FsType` 字段

`model.DiskUsage` 从 `{MountPoint, Usage, UsageFloat}` 扩展为 `{MountPoint, FsType, Usage, UsageFloat}`。

- 新字段, 旧 JSON 消费方不受影响。
- 让报告里能看到挂载点对应的文件系统类型, 排查问题时有用。
- inode 和 disk 共用同一个结构。

## Risks / Trade-offs

- **[行为变更可能产生新的告警]** 升级后, 之前默认配置下"看不见的"根分区使用率会暴露出来, 部分原本就高使用率但没被监控的主机会立刻产生告警 → 这是修复目标, 但需要在 release notes 里明确说明, 让运维有心理预期。
- **[`df -T` 在极少数极简镜像上可能不可用]** busybox 默认支持 `-T`, Alpine/CentOS/Ubuntu 均支持; 极少数定制镜像可能阉割。Mitigation: parser 检测到 Type 列缺失时, 退化为按设备名启发式判断(以 `/dev/` 开头视为真盘), 并把 fs_type 标为 `unknown`。
- **[NFS 默认排除改变行为]** 当前实现里, 如果用户把某 NFS 挂载点加进了 `CHECK_MOUNT_PATH`, 会被采集到; 新方案中**显式配置时仍会采**(决策 2), 不影响这种用法。仅默认全采模式下排除 NFS。Mitigation: 在 README/proposal 中明确说明 `INSPECT_DISK_INCLUDE_NFS` 开关。
- **[overlay/shm 行被排除后, 容器宿主机看不到容器层使用率]** → 这是预期行为。容器层最终落到宿主机根盘, 监控根盘就够了; 如要看单容器用量, 是另一个 capability 的范畴。

## Migration Plan

1. 一次性合并 `collector/host.go` + `config/config.go` + `model/types.go` 修改, 单 PR。
2. 升级前在 release notes 强调:
   - 留空 `CHECK_MOUNT_PATH` 的部署会从"几乎不采"变成"采所有真实磁盘", 阈值告警面会扩大。
   - 想保持原行为, 显式设置 `CHECK_MOUNT_PATH=/data`。
3. 无回滚脚本: 配置回退即可恢复旧行为(显式 `CHECK_MOUNT_PATH` 路径在新代码下行为与旧代码一致, 唯一例外是 fs 黑名单生效——但黑名单只会排除 tmpfs/overlay 这类不该出现在用户配置里的东西)。
4. 不需要 db migration, 不需要协调下游消费方(JSON 只新增字段)。
