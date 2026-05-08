package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"weops-inspect/checker"
	"weops-inspect/collector"
	"weops-inspect/config"
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
	hostMetrics := collector.CollectAllHosts(sshClient, cfg.AllHosts, cfg.CheckMountPath)

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

	// Check service status rules
	for _, statuses := range serviceResults {
		for _, s := range statuses {
			for _, sm := range s.Services {
				checks := checker.CheckService(sm)
				allChecks = append(allChecks, checks...)
			}
		}
	}

	// Phase 3: Collect open source components
	fmt.Fprintf(os.Stderr, "[3/3] 采集开源组件状态...\n")
	report.ES = collector.CollectES(cfg)
	report.MySQL = collector.CollectMySQL(context.Background(), cfg)
	report.RedisStandalone = collector.CollectRedisStandalone(cfg)
	report.RedisSentinel = collector.CollectRedisSentinel(cfg)
	collector.CrossCheckSentinelMaster(report.RedisSentinel, cfg.RedisMasterIPs)
	report.MongoDB = collector.CollectMongo(cfg)
	report.RabbitMQ = collector.CollectRabbitMQ(cfg)
	report.Replication = collector.CollectReplication(cfg)
	if deps := collector.CollectBKMonitorV3Deps(cfg); deps != nil {
		report.BKMonitorV3 = &model.BKMonitorV3Section{Dependencies: deps}
	}

	// Replication checks (no-op when Replication is nil)
	allChecks = append(allChecks, checker.CheckReplication(report.Replication)...)

	// Summary
	report.Summary = checker.Summarize(allChecks)

	// Output
	fmt.Fprintf(os.Stderr, "\n生成报告...\n")
	htmlPath, err := output.Write(report, cfg.OutputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "报告生成失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "\n巡检完成! 共 %d 项检查, %d 正常, %d 告警\n",
		report.Summary.Total, report.Summary.OK, report.Summary.Warn)

	// Optional alert notification (skipped silently when no config or disabled).
	if notifyCfg, err := notify.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "notify: 配置加载失败: %v\n", err)
	} else if notifyCfg != nil {
		notify.Process(notifyCfg, report, htmlPath)
	}
}
