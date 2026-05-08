## ADDED Requirements

### Requirement: 磁盘使用率采集默认覆盖所有真实文件系统

当 `CHECK_MOUNT_PATH` 未配置或为空时, 系统 SHALL 通过 `df -ThP` 采集所有"真实"文件系统的挂载点使用率, 并按以下规则筛选:

- **黑名单类型**(始终排除, 不可关闭): `tmpfs, devtmpfs, overlay, squashfs, shm, proc, sysfs, cgroup, cgroup2, autofs, binfmt_misc, mqueue, pstore, debugfs, tracefs, ramfs, rpc_pipefs, fusectl, configfs, securityfs, hugetlbfs, fuse.lxcfs, fuse.gvfsd-fuse`。
- **白名单类型**(显式接受): `xfs, ext2, ext3, ext4, btrfs, zfs, f2fs, ufs, jfs, reiserfs`。
- **NFS/SMB 类型**(`nfs, nfs4, cifs, smbfs, smb3`): 默认排除, `INSPECT_DISK_INCLUDE_NFS=true` 时纳入。
- 其他未知类型: 默认排除(避免新出现的伪文件系统造成噪音), 但通过 warning 告知运维。

判定优先级: 黑名单 > NFS 开关 > 白名单。

#### Scenario: 默认配置下采集 LVM 根分区

- **WHEN** 主机 BUILD60 的 `df -ThP` 中 `/` 是 `xfs` 类型, 使用率 98%, 且 `CHECK_MOUNT_PATH` 留空
- **THEN** 该主机 `DiskUsage` 至少包含一条 `MountPoint="/", FsType="xfs", UsageFloat=98` 的记录
- **AND** checker 据此对 `disk_usage(/)` 触发告警

#### Scenario: 默认配置下排除容器宿主机的 overlay/tmpfs 噪音

- **WHEN** 主机 `df -ThP` 输出包含 12 条 `overlay` 类型的容器分层挂载和若干 `tmpfs` 挂载, 且 `CHECK_MOUNT_PATH` 留空
- **THEN** `DiskUsage` 中不出现任何 `FsType="overlay"` 或 `FsType="tmpfs"` 的记录

#### Scenario: NFS 默认排除

- **WHEN** 主机存在 `nfs4` 挂载 `/data/share`, 且 `INSPECT_DISK_INCLUDE_NFS` 未设置
- **THEN** `DiskUsage` 不包含 `/data/share`

#### Scenario: 显式启用后采集 NFS

- **WHEN** `INSPECT_DISK_INCLUDE_NFS=true` 且主机存在 `nfs4` 挂载 `/data/share`
- **THEN** `DiskUsage` 包含 `MountPoint="/data/share", FsType="nfs4"` 的记录

### Requirement: 显式配置 `CHECK_MOUNT_PATH` 时按完全相等匹配

当 `CHECK_MOUNT_PATH` 为非空字符串时, 系统 SHALL 按冒号分隔解析挂载点列表, 仅采集与列表中字符串**完全相等**的挂载点, 不做前缀匹配。即使如此, 黑名单类型仍然排除。

#### Scenario: 显式列表精确命中

- **WHEN** `CHECK_MOUNT_PATH=/var:/data` 且主机 `df` 输出包含 `/var`(ext4) 和 `/data`(xfs) 两个挂载
- **THEN** `DiskUsage` 恰好包含 `/var` 和 `/data` 两条记录

#### Scenario: 显式列表不做前缀匹配

- **WHEN** `CHECK_MOUNT_PATH=/data` 且主机 `df` 输出包含 `/data/share`(nfs4)、`/data/bkee/.../merged`(overlay), 不存在 `/data` 自身挂载
- **THEN** `DiskUsage` 为空(不前缀匹配子目录)

#### Scenario: 显式列表中包含黑名单类型时仍排除

- **WHEN** `CHECK_MOUNT_PATH=/run` 且 `/run` 是 `tmpfs` 类型
- **THEN** `DiskUsage` 不包含 `/run`(黑名单优先级高于显式列表)

### Requirement: `df` 输出解析使用 POSIX 格式且按列定位

系统 SHALL 远端执行 `df -ThP`(disk usage)和 `df -iPT`(inode usage), 利用 `-P` 强制每条记录单行输出, 利用 `-T` 拿到文件系统类型列。Parser SHALL 跳过表头行后按列索引(Filesystem, Type, Size, Used, Avail, Use%, Mounted on)定位字段, 不再依赖"最后两列"的启发式。

#### Scenario: 长 LVM 设备名不再造成漏采

- **WHEN** 主机有 `/dev/mapper/very-long-volume-group-name-root` 挂载到 `/`
- **THEN** `df -ThP` 单行输出该 entry, parser 正确解析出 `MountPoint="/"`

#### Scenario: 表头行被跳过

- **WHEN** `df -ThP` 第一行是 `Filesystem Type Size Used Avail Use% Mounted on`
- **THEN** parser 不把该行当作 mount entry

### Requirement: 筛选结果为空时输出可见 warning

系统 SHALL 在 `df` 命令成功执行但筛选后 `DiskUsage` 为空数组时, 在该主机的 `Error` 字段追加 warning 信息(格式以 `disk:` 前缀开头, 不阻塞 inode 等其他指标采集)。SSH 失败导致整个 batch 不可用的情况下, `Error` 字段继续以 `SSH error:` 前缀报告, 不与本 warning 混淆。

#### Scenario: 显式配置不命中任何挂载点

- **WHEN** `CHECK_MOUNT_PATH=/data` 且 BUILD60 主机不存在 `/data` 挂载
- **THEN** 该主机 `Error` 字段包含 `disk: configured mount paths [/data] did not match any of [/, /boot, /boot/efi]`(或同等可读信息)
- **AND** 其他指标(CPU、内存、inode 等)正常采集

#### Scenario: SSH 失败优先级高于 disk warning

- **WHEN** 主机 SSH 不可达
- **THEN** `Error` 字段以 `SSH error:` 开头, 不再输出 disk warning(整个采集流程已经中断)

### Requirement: `DiskUsage` 数据结构包含文件系统类型

系统 SHALL 在 `model.DiskUsage` 结构中新增 `FsType` 字段(JSON 字段名 `fs_type`), 由 parser 从 `df -ThP` 的 Type 列填充。该字段同时用于 disk usage 和 inode usage(共用结构)。原有 `MountPoint`, `Usage`, `UsageFloat` 字段语义不变, 旧的下游消费方不受影响。

#### Scenario: JSON 输出包含 fs_type

- **WHEN** 主机有 `/` 挂载点(xfs, 98% 使用)被采集
- **THEN** 报告 JSON 中该条目包含 `"mount_point": "/"`, `"fs_type": "xfs"`, `"usage": "98%"`, `"usage_float": 98`

#### Scenario: inode 记录也带 fs_type

- **WHEN** `/` 的 inode 使用率被采集
- **THEN** `InodeUsage` 中对应条目同样包含 `"fs_type": "xfs"`
