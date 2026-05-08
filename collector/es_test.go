package collector

import (
	"context"
	"testing"
	"time"

	"weops-inspect/config"
)

// 用 RFC5737 TEST-NET-1 段(192.0.2.0/24)的 IP 触发 curl 立即失败 — 这些地址保留给文档示例,
// 不路由,curl 会报 "Couldn't connect to server"。结合 --max-time 5 上限,单次失败远低于超时。

func TestCollectES_AllUnreachable(t *testing.T) {
	cfg := &config.Config{
		ES7IPs: []string{"192.0.2.1", "192.0.2.2"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clusters := CollectES(ctx, cfg)
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	c := clusters[0]
	if c.Error != "all nodes unreachable" {
		t.Errorf("Error = %q, want 'all nodes unreachable'", c.Error)
	}
	if c.ErrorClass != string(ErrNetwork) {
		t.Errorf("ErrorClass = %q, want %q", c.ErrorClass, ErrNetwork)
	}
	if len(c.NodeReachability) != 2 {
		t.Fatalf("NodeReachability len = %d, want 2", len(c.NodeReachability))
	}
	for i, r := range c.NodeReachability {
		if r.Status != "unreachable" {
			t.Errorf("NodeReachability[%d].Status = %q, want unreachable", i, r.Status)
		}
		if r.IP != cfg.ES7IPs[i] {
			t.Errorf("NodeReachability[%d].IP = %q, want %q (顺序应与输入一致)", i, r.IP, cfg.ES7IPs[i])
		}
	}
}

func TestCollectES_EmptyIPs(t *testing.T) {
	cfg := &config.Config{}
	clusters := CollectES(context.Background(), cfg)
	if clusters != nil {
		t.Errorf("expected nil for empty ES7IPs, got %v", clusters)
	}
}
