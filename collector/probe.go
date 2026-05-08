package collector

import (
	"context"
	"errors"
	"log"
	"net"
	"regexp"
	"strings"
	"time"
)

// ErrorClass 把每次探测错误归到固定枚举,供报告渲染统一判级。
type ErrorClass string

const (
	ErrNone     ErrorClass = ""
	ErrNetwork  ErrorClass = "network"
	ErrAuth     ErrorClass = "auth"
	ErrProtocol ErrorClass = "protocol"
	ErrTimeout  ErrorClass = "timeout"
	ErrUnknown  ErrorClass = "unknown"
)

// ProbeResult 是 Probe.Run 的统一返回结构。
type ProbeResult struct {
	Target   string
	Latency  time.Duration
	Err      error
	ErrClass ErrorClass
}

// Probe 是所有组件 collector 共享的探针接口。
type Probe interface {
	Name() string
	Run(ctx context.Context) ProbeResult
}

// Logger 用于把每次探测的结构化结果输出到调用方指定的实现。
type Logger interface {
	Probe(name, target string, latency time.Duration, errClass ErrorClass, err error)
}

type nopLogger struct{}

func (nopLogger) Probe(string, string, time.Duration, ErrorClass, error) {}

type stdLogger struct{}

func (stdLogger) Probe(name, target string, latency time.Duration, errClass ErrorClass, err error) {
	if err != nil {
		log.Printf("probe name=%s target=%s latency_ms=%d err_class=%s err=%v",
			name, target, latency.Milliseconds(), errClass, err)
	}
}

var probeLogger Logger = stdLogger{}

// SetProbeLogger 注入自定义 Logger;传 nil 则使用 nopLogger。
func SetProbeLogger(l Logger) {
	if l == nil {
		probeLogger = nopLogger{}
		return
	}
	probeLogger = l
}

// DefaultProbeTimeout 是单次探测默认超时(框架内常量,不暴露到配置)。
const DefaultProbeTimeout = 5 * time.Second

// RunProbe 给一次 probe 套上 ctx 超时、计时、日志钩子。
func RunProbe(ctx context.Context, p Probe) ProbeResult {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultProbeTimeout)
		defer cancel()
	}
	start := time.Now()
	r := p.Run(ctx)
	if r.Latency == 0 {
		r.Latency = time.Since(start)
	}
	if r.Err != nil && r.ErrClass == "" {
		r.ErrClass = Classify(r.Err)
	}
	probeLogger.Probe(p.Name(), r.Target, r.Latency, r.ErrClass, r.Err)
	return r
}

// Classify 返回 err 对应的通用 ErrorClass。组件专属错误(如 MySQL 1045 → auth)
// 由各 collector 在 Classify 之前自行判定后再传入,或调用方覆写 ProbeResult.ErrClass。
func Classify(err error) ErrorClass {
	if err == nil {
		return ErrNone
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return ErrTimeout
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ErrTimeout
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return ErrNetwork
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return ErrNetwork
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "i/o timeout"), strings.Contains(msg, "deadline exceeded"):
		return ErrTimeout
	case strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "no route to host"),
		strings.Contains(msg, "network is unreachable"),
		strings.Contains(msg, "no such host"):
		return ErrNetwork
	case strings.Contains(msg, "access denied"),
		strings.Contains(msg, "authentication failed"),
		strings.Contains(msg, "noauth"),
		strings.Contains(msg, "wrongpass"):
		return ErrAuth
	}
	return ErrUnknown
}

// dsnPwdRe 匹配 user:pass@ 形式 DSN/URI 中的密码段。
var dsnPwdRe = regexp.MustCompile(`(://)?([^:/\s@]+):([^@/\s]+)@`)

// RedactDSN 把字符串里 user:pass@ 形式的密码替换成 ***。
// 同时覆盖 mongodb://user:pass@host 与 user:pass@tcp(host)/ 两类常见形式。
func RedactDSN(s string) string {
	if s == "" {
		return s
	}
	return dsnPwdRe.ReplaceAllString(s, "${1}${2}:***@")
}

// WrapErr 在错误传播路径上脱敏,避免 DSN/URI 中的明文密码漏到日志或报告。
func WrapErr(err error) error {
	if err == nil {
		return nil
	}
	redacted := RedactDSN(err.Error())
	if redacted == err.Error() {
		return err
	}
	return errors.New(redacted)
}
