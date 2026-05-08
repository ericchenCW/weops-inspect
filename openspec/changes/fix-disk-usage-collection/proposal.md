## Why

当前主机磁盘使用率采集存在严重的"静默漏采"缺陷:

- 默认 `CHECK_MOUNT_PATH=/data` 仅采集**完全相等**的挂载点列表; 现实中很多机器(如 LVM 把数据合并到 `/`)根本没有 `/data` 这个挂载, 导致 `DiskUsage` 为空数组、checker 无任何告警。
- 真实案例: 一台 BUILD60 根分区已 98%, 但因为 `/` 不在白名单内, 巡检报告里完全看不到该机器的磁盘信息, 漏掉了最该告警的目标。
- `df -h` / `df -i` 在长设备名(LVM、NFS)下会输出折行, 现有 parser 直接跳过, 进一步加剧漏采。
- 配置 mismatch 时没有任何提示, 运维难以发现自己漏配。

## What Changes

- **BREAKING**: `CHECK_MOUNT_PATH` 语义变更: 留空(默认)时改为按文件系统类型自动筛选真实磁盘, 不再是空集; 显式配置时仍然按完全相等匹配, 但由"无配置→只采 `/data`"改为"无配置→采所有真实磁盘"。
- 远端命令由 `df -h` / `df -i` 改为 `df -ThP` / `df -iPT`(`-P` 防折行, `-T` 输出文件系统类型)。
- 引入文件系统白名单常量: `xfs, ext2, ext3, ext4, btrfs, zfs, f2fs, ufs, jfs, reiserfs`; 黑名单类型(`tmpfs, devtmpfs, overlay, squashfs, shm, proc, sysfs, cgroup, fuse.*, autofs, binfmt_misc, mqueue, pstore, debugfs, tracefs`)始终排除。
- NFS 默认排除, 通过新增 `INSPECT_DISK_INCLUDE_NFS=true` 显式启用。
- 当 `df` 输出非空但筛选结果为 0 条时, 在该主机的 `Error` 字段追加 warning(不阻塞采集), 让运维能看到"配置和实际不匹配"。
- inode 采集走相同的筛选逻辑, 与 disk usage 保持一致。

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `host-metrics-collection`: 新增对磁盘/inode 采集的筛选语义、命令格式与 NFS 处理规则; 现有"采集字段集合保持不变"的描述继续有效, 但选取哪些挂载点的逻辑被重新定义。

## Impact

- 受影响代码: [collector/host.go](collector/host.go) 的 `hostBatchCmd`、`parseDiskUsage`、`parseHostOutput`; [config/config.go](config/config.go) 的 `CheckMountPath` 语义与新增 `DiskIncludeNFS`。
- 受影响数据结构: [model/types.go](model/types.go) `DiskUsage` 增加 `FsType` 字段(向后兼容, JSON 输出多一列)。
- 受影响下游: [checker/rules.go](checker/rules.go) 遍历 `DiskUsage` 的循环不变, 但实际遍历到的条目会变多/变化, 阈值告警范围相应扩大。
- 配置兼容性: 显式设置过 `CHECK_MOUNT_PATH=/data` 的部署在升级后行为不变; 未设置或留空的部署会从"几乎不采"切换到"采所有真实磁盘", 是预期的修复方向。
- 远端命令依赖: `df` 必须支持 `-T -P` 选项(GNU coreutils 与 busybox 均支持, 影响面极小)。
