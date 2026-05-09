package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"weops-inspect/checker"
	"weops-inspect/collector"
	"weops-inspect/config"
	"weops-inspect/lock"
	"weops-inspect/model"
	"weops-inspect/notify"
	"weops-inspect/output"
	sshclient "weops-inspect/ssh"
)

var version = "dev"

func main() {
	outputDir := flag.String("o", ".", "输出目录")
	showVersionShort := flag.Bool("v", false, "打印版本号并退出")
	showVersionLong := flag.Bool("version", false, "打印版本号并退出")
	flag.Parse()

	if *showVersionShort || *showVersionLong {
		fmt.Println(version)
		return
	}

	// Load config from BK_* environment variables
	cfg, err := config.Load(*outputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "配置加载失败: %v\n", err)
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "配置校验失败: %v\n", err)
		os.Exit(1)
	}

	// Single-instance guard. Lock lives next to notify state.json so it
	// shares the user-private config dir. If another instance holds the
	// lock, exit code 0 — overlapping cron triggers are protective skips,
	// not failures (a non-zero exit would make cron mail the operator).
	if notifyPath, err := notify.ConfigPath(); err == nil {
		lockPath := filepath.Join(filepath.Dir(notifyPath), "inspect.lock")
		release, err := lock.Acquire(lockPath)
		switch {
		case err == nil:
			defer release()
		case errors.Is(err, lock.ErrBusy):
			fmt.Fprintln(os.Stderr, "weops-inspect: another instance is running, exiting")
			return
		default:
			fmt.Fprintf(os.Stderr, "weops-inspect: lock unavailable, continuing without it: %v\n", err)
		}
	}

	report := &model.InspectReport{
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Services:  make(map[string][]model.ServiceStatus),
	}

	// Initialize SSH client
	sshClient, err := sshclient.New(cfg.SSHUser, cfg.SSHPort, cfg.SSHKeyPath, cfg.SSHUseSudo,
		30*time.Second, 60*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SSH 客户端初始化失败: %v\n", err)
		os.Exit(1)
	}

	// Phase 1: Collect host metrics
	fmt.Fprintf(os.Stderr, "[1/3] 采集主机指标 (%d 台主机)...\n", len(cfg.AllHosts))
	hostMetrics := collector.CollectAllHosts(sshClient, cfg.AllHosts, cfg.CheckMountPath, cfg.DiskIncludeNFS)

	// Apply rules and build host check results
	var allChecks []model.CheckResult
	for _, hm := range hostMetrics {
		checks := checker.CheckHost(hm, cfg.Thresholds)
		allChecks = append(allChecks, checks...)
		report.Hosts = append(report.Hosts, model.HostCheckResult{
			Metrics: hm,
			Checks:  checks,
		})
	}

	// Phase 2: Collect service status
	fmt.Fprintf(os.Stderr, "[2/3] 采集蓝鲸模块状态...\n")
	serviceResults := collector.CollectAllServices(sshClient, cfg)
	report.Services = serviceResults

	// Check service status rules. Iterate by index so that backfilled
	// RenderStatus / HealthzRenderStatus / ExitedRenderStatus persist for
	// template rendering.
	for moduleKey, statuses := range serviceResults {
		for i := range statuses {
			s := &statuses[i]
			allChecks = append(allChecks, checker.CheckServiceCollectError(s)...)
			for j := range s.Services {
				allChecks = append(allChecks, checker.CheckService(&s.Services[j], s.HostIP, moduleKey)...)
			}
			allChecks = append(allChecks, checker.CheckServiceContainers(s, cfg.Thresholds)...)
		}
	}

	// Phase 3: Collect open source components
	fmt.Fprintf(os.Stderr, "[3/3] 采集开源组件状态...\n")
	ctx := context.Background()
	report.ES = collector.CollectES(ctx, cfg)
	report.MySQL = collector.CollectMySQL(ctx, cfg)
	report.RedisStandalone = collector.CollectRedisStandalone(ctx, cfg)
	report.RedisSentinel = collector.CollectRedisSentinel(ctx, cfg)
	collector.CrossCheckSentinelMaster(report.RedisSentinel, cfg.RedisMasterIPs)
	report.MongoDB = collector.CollectMongo(ctx, cfg)
	report.RabbitMQ = collector.CollectRabbitMQ(ctx, cfg)
	report.Replication = collector.CollectReplication(ctx, cfg)
	if deps := collector.CollectBKMonitorV3Deps(cfg); deps != nil {
		report.BKMonitorV3 = &model.BKMonitorV3Section{Dependencies: deps}
	}

	// Component-level checks (each Check* mutates the report struct in place to
	// backfill render statuses, then returns CheckResults for Summary/notify).
	allChecks = append(allChecks, checker.CheckES(report.ES, cfg.Thresholds)...)
	allChecks = append(allChecks, checker.CheckRedis(report.RedisStandalone, cfg.Thresholds)...)
	allChecks = append(allChecks, checker.CheckRedisSentinel(report.RedisSentinel)...)
	allChecks = append(allChecks, checker.CheckMongo(report.MongoDB)...)
	allChecks = append(allChecks, checker.CheckRabbitMQ(report.RabbitMQ, cfg.Thresholds)...)
	allChecks = append(allChecks, checker.CheckBKDeps(report.BKMonitorV3)...)
	allChecks = append(allChecks, checker.CheckReplication(report.Replication, cfg.Thresholds)...)

	// Summary
	report.Summary = checker.Summarize(allChecks)
	report.AllChecks = allChecks

	// Optional notify config: when enabled, persistence confirmation runs
	// BEFORE rendering so the on-disk HTML/JSON match what we notify on.
	// Pending warns are demoted to Notice (excluded from Summary).
	notifyCfg, err := notify.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "notify: 配置加载失败: %v\n", err)
		notifyCfg = nil
	}
	prep := notify.Prepare(notifyCfg, report)

	// Output (after persistence demotion so HTML, JSON, and Summary agree)
	fmt.Fprintf(os.Stderr, "\n生成报告...\n")
	htmlPath, err := output.Write(report, cfg.OutputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "报告生成失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "\n巡检完成! 共 %d 项检查, %d 正常, %d 告警, %d 未知\n",
		report.Summary.Total, report.Summary.OK, report.Summary.Warn, report.Summary.Unknown)

	notify.Dispatch(prep, notifyCfg, report, htmlPath)
}
