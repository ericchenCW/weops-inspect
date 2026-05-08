package notify

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

// Signature returns a stable hash over the set of alerts. Only Host and Field
// participate (Value drift, e.g. CPU 76% → 78%, must not change the signature).
// Returns "" when there are no alerts so the empty case is distinguishable.
func Signature(items []AlertItem) string {
	if len(items) == 0 {
		return ""
	}
	keys := make([]string, len(items))
	for i, a := range items {
		keys[i] = a.Host + "|" + a.Field
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
