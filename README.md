# weops-inspect

蓝鲸 (BlueKing) 运维平台一次性巡检 CLI。运行后对所有蓝鲸模块所在主机、蓝鲸服务
进程、以及周边开源中间件做一次全量健康采集，输出 `weops_inspection.html` 与
`weops_inspection.json` 报告。

```
weops-inspect -o /var/log/weops
```

配置以 `BK_*` / `INSPECT_*` 环境变量为准（与 `bk.env` 对齐）。

## 磁盘采集配置

主机磁盘 / inode 使用率通过远端 `df -ThP` / `df -iPT` 采集，按以下规则筛选要纳入
告警判断的挂载点：

| `CHECK_MOUNT_PATH`     | 行为                                                                 |
|-----------------------|----------------------------------------------------------------------|
| 留空（默认）          | 自动按文件系统类型采集所有"真实"磁盘（xfs/ext4/btrfs/zfs 等）       |
| `/data:/var:/home`    | 仅采集冒号分隔列表中**完全相等**的挂载点（不做前缀匹配）            |

无论哪种模式，伪文件系统（`tmpfs` / `devtmpfs` / `overlay` / `squashfs` / `shm` /
`proc` / `sysfs` / `cgroup` / `autofs` 等）始终排除，避免容器宿主机被淹没。

NFS / SMB（`nfs` / `nfs4` / `cifs` / `smbfs` / `smb3`）默认不采，需要时显式启用：

```sh
INSPECT_DISK_INCLUDE_NFS=true
```

> **Breaking change（v? 起）**：旧版本默认 `CHECK_MOUNT_PATH=/data`，未配置 `/data`
> 挂载的主机会静默漏采（最典型：LVM 把数据合并到 `/`）。新默认改为"采所有真实磁盘"，
> 升级后阈值告警面会扩大；想保持原行为请显式 `CHECK_MOUNT_PATH=/data`。
>
> 当 `df` 输出非空但筛选结果为 0 条时，主机条目的 `error` 字段会追加
> `disk: configured mount paths [...] did not match any of [...]` 形式的 warning，
> 便于发现配置和实际挂载不匹配的情况。

## bkmonitorv3 角色 IP 配置

bkmonitorv3 的 4 个子角色(monitor / influxdb-proxy / transfer / unify-query)在
生产中常被分散到不同主机部署。巡检会按角色分别在对应主机上探测 systemctl 与
healthz,避免在没跑该角色的主机上误报 `not-found` / `unreachable`。

| 环境变量                                      | 含义                                               |
|-----------------------------------------------|----------------------------------------------------|
| `BK_MONITORV3_MONITOR_IP_COMMA`               | bk-monitor (supervisord) 部署主机                  |
| `BK_MONITORV3_INFLUXDB_PROXY_IP_COMMA`        | bk-influxdb-proxy 部署主机                         |
| `BK_MONITORV3_TRANSFER_IP_COMMA`              | bk-transfer 部署主机                               |
| `BK_MONITORV3_UNIFY_QUERY_IP_COMMA`           | bk-unify-query 部署主机                            |
| `BK_MONITORV3_IP_COMMA`(legacy / 兼容)       | 任一角色变量为空时回退到此 IP 列表                 |

`ingester` 角色不采集。仅设 `BK_MONITORV3_IP_COMMA` 时,4 个角色都按它的 IP 列表
采集,等价于本特性上线之前的行为。

## 巡检判定流程

每个原始数据字段都流经一条统一的判定流水线，三个出口（Summary / 告警邮件 /
HTML 着色）共享同一份 `CheckResult.Status`，避免"红字 ≠ 计数 ≠ 邮件"的不一致。

```
原始数据                统一判定                     出口
───────                ───────                     ────

HostMetrics ─────► CheckHost          ┐
ServiceModule ───► CheckService       │            ┌─► Summary
ESCluster ──────► CheckES             │            │   (Total / OK / Warn / Unknown)
RedisNode ──────► CheckRedis          ├─► CheckResult
SentinelStatus ─► CheckRedisSentinel  │   { Field, ├─► 邮件 (仅 Status==Warn)
MongoCluster ───► CheckMongo          │     Value, │   ↓ 签名 + 冷却窗口决策
RabbitMQStatus ─► CheckRabbitMQ       │     Status }│
DependencyResult► CheckBKDeps         │            └─► HTML 渲染结构 Status 字段
ReplicationReport►CheckReplication    ┘                ↓ 模板按 Status 着色
```

四档状态语义：

| 状态    | 计入 Total | 桶       | 进邮件 | HTML       | 典型场景                                     |
|---------|------------|----------|--------|------------|----------------------------------------------|
| OK      | ✓          | OK       | —      | 绿         | active 服务、HeapPercent < 阈值              |
| Warn    | ✓          | Warn     | ✓      | 红         | 节点不可达、RabbitMQ 积压、active 失败        |
| Unknown | ✓          | Unknown  | —      | 灰         | 子服务在主机上未注册（`Status=""`）           |
| Notice  | —          | （不计） | —      | 红         | ES RAM > 95%、Docker 退出 > 0、bkdeps fail   |

> **Warn 与 Notice 同色**（红），但语义不同：Warn 会让人睡醒，Notice 仅着色提示。
> 详细判定规则（每组件哪些字段进哪档、签名计算与通知决策状态机）见
> [docs/checks.md](docs/checks.md)。

Notice 阈值速查（仅以下 6 项触发 Notice，env 覆盖）：

| 环境变量                                  | 默认值 | 含义                                           |
|-------------------------------------------|--------|------------------------------------------------|
| `INSPECT_ES_HEAP_THRESHOLD`               | 85     | ES 节点 `heap_percent` 上限                    |
| `INSPECT_ES_RAM_THRESHOLD`                | 95     | ES 节点 `ram_percent` 上限                     |
| `INSPECT_ES_UNASSIGNED_SHARDS_THRESHOLD`  | 0      | ES 集群未分配分片上限                          |
| `INSPECT_REDIS_CELERY_QUEUE_THRESHOLD`    | 1000   | Redis `celery_queue` 长度上限                  |
| `INSPECT_REDIS_MONITOR_QUEUE_THRESHOLD`   | 10000  | Redis `monitor_queue` 长度上限                 |
| `INSPECT_DOCKER_EXITED_THRESHOLD`         | 0      | Service 段 Docker `containers_exited` 上限     |

> **升级注意**：本特性首跑时此前漏报的项会一次性进入告警邮件（触发"签名变化→
> 立即发送"）。建议先把 `email.enabled` 设为 false 跑一次确认 Warn 列表后再开启
> 通知。同时控制台输出新增"Z 未知"项，HTML 报告新增"未知项"卡片。

## 邮件告警通知

巡检结束后可选地把告警情况通过邮件推送给运维。配置文件位置：

```
~/.config/weops/config.json
```

可由环境变量 `WEOPS_CONFIG=/path/to/config.json` 覆盖。

### 配置示例

```json
{
  "email": {
    "enabled": true,
    "smtp_host": "smtp.example.com",
    "smtp_port": 465,
    "smtp_use_tls": true,
    "username": "alert@example.com",
    "password": "your-smtp-password",
    "from": "WeOps 巡检 <alert@example.com>",
    "to": ["ops@example.com", "dba@example.com"]
  },
  "trigger": {
    "min_interval_minutes": 120,
    "send_recovery": true
  }
}
```

字段说明：

- `email.smtp_use_tls`：`true` 走隐式 SSL/TLS（常见 465 端口），`false` 走明文/
  STARTTLS（常见 25/587 端口）。
- `trigger.min_interval_minutes`：同一组告警的去重窗口（默认 120）。窗口内同签名
  告警会被抑制；告警集合发生变化（新增/消失某项）时立即重发，不受窗口限制。
- `trigger.send_recovery`：上次发过告警邮件、本次恢复正常时是否发送恢复通知。

**安全提醒**：配置文件含明文密码，必须收紧权限：

```
chmod 600 ~/.config/weops/config.json
```

启动时若文件权限不是 0600，工具会在 stderr 打印 warning 提示。

### 通知行为

| 上次状态  | 本次告警数 | 条件                           | 动作       |
|-----------|-----------|--------------------------------|-----------|
| 空 / ok   | 0         | —                              | 不发      |
| 空 / ok   | >0        | —                              | 发告警    |
| alert     | 0         | —                              | 发恢复    |
| alert     | >0        | 签名相同 且 距上次 < 冷却窗口  | 抑制      |
| alert     | >0        | 签名变化（告警集合变了）        | 立即重发  |
| alert     | >0        | 签名相同 且 距上次 ≥ 冷却窗口  | 重发      |

发送状态保存在 `~/.config/weops/state.json`（与 config 同目录）。**仅在发送成功
后**写入；发送失败保留旧基线，下次按旧状态判定，避免一次 SMTP 抖动让告警长期被
误抑制。

通知子系统的任何错误都只在 stderr 输出，**不影响巡检退出码**——与 cron 共存友好。

## Crontab 周期巡检部署

工具本身不会修改 crontab，请手工添加。

### 加载 bk.env 的正确方式（生产环境必看）

蓝鲸生产环境的环境变量定义在 `/data/install/utils.fc` 中，且其中有大量赋值
**没有 `export`**。直接 `source utils.fc` 在子 shell 里这些变量不会被导出，
工具读不到。正确写法是先开启自动 export：

```bash
set -a
source /data/install/utils.fc
./weops-inspect-linux-amd64
```

`set -a` 让此后所有赋值自动 `export`，之后 `source` 进来的变量都可见。手动跑
也按这个套路。

### 推荐 cron 行（每 5 分钟）

```cron
# crontab -e
*/5 * * * * /bin/bash -c 'set -a; source /data/install/utils.fc; /usr/local/bin/weops-inspect -o /var/log/weops' >>/var/log/weops/weops-inspect.log 2>&1
```

要点：

1. **`set -a` 必须在 `source` 之前**：原因见上一节，缺它工具会因为读不到 `BK_*`
   而把 hosts 列表跑成空。
2. **不要用 `bash -lc`**：登录 shell 模式会再加载 `~/.bash_profile` 等，对
   crontab 反而不必要。`/bin/bash -c` 单独 spawn 即可。
3. **stderr 重定向**：工具所有日志走 stderr，全量重定向到日志文件便于排查。
4. **`WEOPS_CONFIG` 覆盖**（可选）：使用非默认路径的邮件配置时：
   ```cron
   */5 * * * * WEOPS_CONFIG=/etc/weops/config.json /bin/bash -c 'set -a; source /data/install/utils.fc; /usr/local/bin/weops-inspect -o /var/log/weops' >>/var/log/weops/weops-inspect.log 2>&1
   ```
5. **冷却窗口与执行频率**：`min_interval_minutes` 应大于等于 cron 间隔的 1 倍，
   否则去重失效。每 5 分钟跑一次配合默认 120 分钟窗口是常见组合。
