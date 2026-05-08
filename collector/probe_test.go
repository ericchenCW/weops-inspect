package collector

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want ErrorClass
	}{
		{"nil", nil, ErrNone},
		{"deadline", context.DeadlineExceeded, ErrTimeout},
		{"canceled", context.Canceled, ErrTimeout},
		{"opErr", &net.OpError{Op: "dial", Err: errors.New("connection refused")}, ErrNetwork},
		{"dnsErr", &net.DNSError{Err: "no such host", Name: "x"}, ErrNetwork},
		{"connRefusedText", errors.New("dial tcp 1.2.3.4:3306: connection refused"), ErrNetwork},
		{"noRouteText", errors.New("no route to host"), ErrNetwork},
		{"ioTimeoutText", errors.New("read tcp: i/o timeout"), ErrTimeout},
		{"accessDeniedText", errors.New("Access denied for user 'root'@'localhost'"), ErrAuth},
		{"noauthText", errors.New("NOAUTH Authentication required."), ErrAuth},
		{"wrongpassText", errors.New("WRONGPASS invalid username-password pair"), ErrAuth},
		{"unknown", errors.New("something else"), ErrUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Classify(c.err)
			if got != c.want {
				t.Fatalf("Classify(%v) = %q, want %q", c.err, got, c.want)
			}
		})
	}
}

// fakeNetTimeout 模拟一个 net.Error(Timeout=true)。
type fakeNetTimeout struct{}

func (fakeNetTimeout) Error() string   { return "fake net timeout" }
func (fakeNetTimeout) Timeout() bool   { return true }
func (fakeNetTimeout) Temporary() bool { return true }

func TestClassifyNetTimeout(t *testing.T) {
	if got := Classify(fakeNetTimeout{}); got != ErrTimeout {
		t.Fatalf("net.Error Timeout=true should be ErrTimeout, got %q", got)
	}
}

func TestRedactDSN(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"mysql dsn", "user:s3cret@tcp(127.0.0.1:3306)/", "user:***@tcp(127.0.0.1:3306)/"},
		{
			"mongodb uri",
			"mongodb://admin:p%40ss@host1:27017,host2:27017/?replicaSet=rs0",
			"mongodb://admin:***@host1:27017,host2:27017/?replicaSet=rs0",
		},
		{"no password", "host:3306", "host:3306"},
		{"empty", "", ""},
		{
			"error message wraps mysql dsn",
			`Error 1045 (28000): Access denied for user 'root'@'10.0.0.1' using DSN root:topsecret@tcp(10.0.0.1:3306)/`,
			`Error 1045 (28000): Access denied for user 'root'@'10.0.0.1' using DSN root:***@tcp(10.0.0.1:3306)/`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RedactDSN(c.in)
			if got != c.want {
				t.Fatalf("RedactDSN(%q) = %q, want %q", c.in, got, c.want)
			}
			if c.in != "" && strings.Contains(got, "topsecret") {
				t.Fatalf("password leaked: %q", got)
			}
		})
	}
}

func TestWrapErr(t *testing.T) {
	if WrapErr(nil) != nil {
		t.Fatalf("nil should stay nil")
	}
	err := errors.New("dial root:pass@tcp(1.2.3.4:3306)/: connection refused")
	wrapped := WrapErr(err)
	if strings.Contains(wrapped.Error(), "pass") {
		t.Fatalf("password leaked through WrapErr: %v", wrapped)
	}
	if !strings.Contains(wrapped.Error(), "***") {
		t.Fatalf("WrapErr should redact, got %v", wrapped)
	}
}

// 框架超时验收:Probe 的 Run 不会被调用足够久,RunProbe 会让 ctx 截止把它拉回来。
func TestRunProbeAppliesDefaultTimeout(t *testing.T) {
	old := probeLogger
	probeLogger = nopLogger{}
	defer func() { probeLogger = old }()

	p := slowProbe{}
	start := time.Now()
	r := RunProbe(context.Background(), p)
	if time.Since(start) > 8*time.Second {
		t.Fatalf("default timeout not applied")
	}
	if r.Latency == 0 {
		t.Fatalf("latency should be recorded")
	}
}

type slowProbe struct{}

func (slowProbe) Name() string { return "slow" }
func (slowProbe) Run(ctx context.Context) ProbeResult {
	<-ctx.Done()
	return ProbeResult{Target: "slow:0", Err: ctx.Err()}
}
