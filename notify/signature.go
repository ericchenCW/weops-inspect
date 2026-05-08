package notify

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
)

// rabbitmqQueueFieldPattern matches per-queue RabbitMQ alert Fields of the form
// rabbitmq.<vhost>.<queue>.<no_consumer|backlog>. Celery-derived queue names
// (celery_api_cron, celery_alert_builder, celery_service_*, etc.) rotate
// between worker generations, so per-queue Field identity is unstable. The
// signature collapses these to a vhost-level key to keep dedup stable.
var rabbitmqQueueFieldPattern = regexp.MustCompile(`^rabbitmq\.([^.]+)\.(.+)\.(no_consumer|backlog)$`)

func normalizeFieldForSignature(field string) string {
	if m := rabbitmqQueueFieldPattern.FindStringSubmatch(field); m != nil {
		return "rabbitmq." + m[1] + "." + m[3]
	}
	return field
}

// Signature returns a stable hash over the set of alerts. Only Host and Field
// participate (Value drift, e.g. CPU 76% → 78%, must not change the signature).
// Returns "" when there are no alerts so the empty case is distinguishable.
func Signature(items []AlertItem) string {
	if len(items) == 0 {
		return ""
	}
	seen := make(map[string]struct{}, len(items))
	for _, a := range items {
		seen[a.Host+"|"+normalizeFieldForSignature(a.Field)] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
