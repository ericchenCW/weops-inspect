package collector

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"weops-inspect/config"
	"weops-inspect/model"
)

// CollectRedisStandalone probes each standalone Redis node in cfg.RedisIPs
// individually using INFO via the native go-redis client.
func CollectRedisStandalone(ctx context.Context, cfg *config.Config) []model.RedisNode {
	if len(cfg.RedisIPs) == 0 {
		return nil
	}

	var nodes []model.RedisNode
	for _, ip := range cfg.RedisIPs {
		p := &redisNodeProbe{ip: ip, port: cfg.RedisPort, password: cfg.Creds.RedisPassword}
		RunProbe(ctx, p)
		nodes = append(nodes, p.node)
	}
	return nodes
}

type redisNodeProbe struct {
	ip       string
	port     string
	password string
	node     model.RedisNode
}

func (p *redisNodeProbe) Name() string { return "redis" }

func (p *redisNodeProbe) Run(ctx context.Context) ProbeResult {
	target := net.JoinHostPort(p.ip, p.port)
	p.node = model.RedisNode{IP: p.ip}

	rdb := newRedisClient(target, p.password, 0)
	defer rdb.Close()

	infoOut, err := rdb.Info(ctx).Result()
	if err != nil {
		p.node.Error = fmt.Sprintf("redis info error: %v", err)
		p.node.ErrorClass = string(classifyRedis(err))
		return ProbeResult{Target: target, Err: err, ErrClass: classifyRedis(err)}
	}

	info := parseRedisInfo(infoOut)
	p.node.Version = info["redis_version"]
	p.node.Role = info["role"]
	p.node.ClusterEnabled = info["cluster_enabled"]
	p.node.UsedMemory = info["used_memory"]
	p.node.MaxMemory = info["maxmemory"]
	p.node.UptimeDays = info["uptime_in_days"]
	p.node.ConnectedClients = info["connected_clients"]
	p.node.BlockedClients = info["blocked_clients"]

	// celery / monitor 队列长度位于 db 11。
	rdb11 := newRedisClient(target, p.password, 11)
	defer rdb11.Close()
	if v, err := rdb11.LLen(ctx, "celery").Result(); err == nil {
		p.node.CeleryQueue = int(v)
	}
	if v, err := rdb11.LLen(ctx, "monitor").Result(); err == nil {
		p.node.MonitorQueue = int(v)
	}

	return ProbeResult{Target: target}
}

// CollectRedisSentinel probes each sentinel node in cfg.RedisSentinelIPs and
// derives cluster-level health using the native sentinel client.
func CollectRedisSentinel(ctx context.Context, cfg *config.Config) *model.SentinelClusterStatus {
	if len(cfg.RedisSentinelIPs) == 0 {
		return nil
	}

	masterName := envOrDefault("BK_APIGW_REDIS_SENTINEL_MASTER_NAME", "mymaster")
	sentinelPort := envOrDefault("INSPECT_REDIS_SENTINEL_PORT", "26379")

	st := &model.SentinelClusterStatus{
		MasterName: masterName,
	}

	reachableCount := 0
	for _, ip := range cfg.RedisSentinelIPs {
		s := pingSentinel(ctx, ip, sentinelPort)
		st.Sentinels = append(st.Sentinels, s)
		if s.Reachable {
			reachableCount++
		}
	}

	if reachableCount == 0 {
		st.Status = "critical"
		st.Error = "no sentinel reachable"
		st.ErrorClass = string(ErrNetwork)
		return st
	}

	// 通过任一可达 sentinel 发现 master,然后对 master 做单独可达性探测。
	for _, s := range st.Sentinels {
		if !s.Reachable {
			continue
		}
		masterIP, masterPort, err := sentinelGetMaster(ctx, s.IP, s.Port, masterName)
		if err == nil && masterIP != "" {
			st.DiscoveredMaster = net.JoinHostPort(masterIP, masterPort)
			st.MasterReachable = redisPing(ctx, masterIP, masterPort, cfg.Creds.RedisPassword)
			break
		}
	}

	switch {
	case st.DiscoveredMaster == "" || !st.MasterReachable:
		st.Status = "critical"
	case reachableCount < len(cfg.RedisSentinelIPs):
		st.Status = "warn"
	default:
		st.Status = "ok"
	}
	return st
}

func pingSentinel(ctx context.Context, ip, port string) model.SentinelNodeStatus {
	s := model.SentinelNodeStatus{IP: ip, Port: port}
	addr := net.JoinHostPort(ip, port)
	sc := redis.NewSentinelClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		MaxRetries:   -1,
	})
	defer sc.Close()

	pong, err := sc.Ping(ctx).Result()
	if err != nil {
		s.Error = fmt.Sprintf("ping error: %v", err)
		s.ErrorClass = string(classifyRedis(err))
		return s
	}
	if pong == "PONG" {
		s.Reachable = true
	} else {
		s.Error = "unexpected response: " + pong
		s.ErrorClass = string(ErrProtocol)
	}
	return s
}

func sentinelGetMaster(ctx context.Context, ip, port, masterName string) (string, string, error) {
	addr := net.JoinHostPort(ip, port)
	sc := redis.NewSentinelClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		MaxRetries:   -1,
	})
	defer sc.Close()

	pair, err := sc.GetMasterAddrByName(ctx, masterName).Result()
	if err != nil {
		return "", "", err
	}
	if len(pair) < 2 {
		return "", "", errors.New("sentinel returned incomplete master addr")
	}
	return pair[0], pair[1], nil
}

func redisPing(ctx context.Context, ip, port, password string) bool {
	addr := net.JoinHostPort(ip, port)
	rdb := newRedisClient(addr, password, 0)
	defer rdb.Close()
	pong, err := rdb.Ping(ctx).Result()
	return err == nil && pong == "PONG"
}

func newRedisClient(addr, password string, db int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		MaxRetries:   -1,
	})
}

func parseRedisInfo(output string) map[string]string {
	info := make(map[string]string)
	for _, line := range strings.Split(output, "\r\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.Index(line, ":"); idx > 0 {
			info[line[:idx]] = line[idx+1:]
		}
	}
	// fallback to \n if INFO body uses LF only.
	if len(info) == 0 {
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if idx := strings.Index(line, ":"); idx > 0 {
				info[line[:idx]] = line[idx+1:]
			}
		}
	}
	return info
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// classifyRedis 在通用 Classify 之前优先识别 Redis 协议错误。
// go-redis 把服务端错误映射成普通 error,这里按文本前缀判定。
func classifyRedis(err error) ErrorClass {
	if err == nil {
		return ErrNone
	}
	msg := err.Error()
	upper := strings.ToUpper(msg)
	switch {
	case strings.Contains(upper, "NOAUTH"),
		strings.Contains(upper, "WRONGPASS"),
		strings.Contains(upper, "INVALID PASSWORD"):
		return ErrAuth
	}
	// strconv 之类的解析失败保留意为 protocol。
	if errors.Is(err, strconv.ErrSyntax) || errors.Is(err, strconv.ErrRange) {
		return ErrProtocol
	}
	return Classify(err)
}
