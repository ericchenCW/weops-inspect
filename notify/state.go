package notify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LastStatus values stored in State.LastStatus.
const (
	StatusEmpty = ""
	StatusAlert = "alert"
	StatusOK    = "ok"
)

// PendingItem records how many consecutive runs an alert has been observed
// before it is allowed to enter the decision matrix. FirstSeen anchors the
// 24h GC sweep that prunes stale entries even when keys never reappear.
type PendingItem struct {
	Count     int       `json:"count"`
	FirstSeen time.Time `json:"first_seen"`
}

// State is persisted between runs to drive cooldown and recovery decisions.
type State struct {
	LastSentAt     time.Time              `json:"last_sent_at"`
	LastSignature  string                 `json:"last_signature"`
	LastStatus     string                 `json:"last_status"`
	Pending        map[string]PendingItem `json:"pending,omitempty"`
	RecoveryStreak int                    `json:"recovery_streak,omitempty"`
}

// LoadState reads the state file. A missing file returns a zero-value state
// (cold start). A corrupt file is treated as cold start with a warning, so a
// damaged file cannot indefinitely suppress alerts.
func LoadState(path string) *State {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "notify: 读取状态文件失败 (%v)，按冷启动处理\n", err)
		}
		return &State{Pending: map[string]PendingItem{}}
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		fmt.Fprintf(os.Stderr, "notify: 状态文件解析失败 (%v)，按冷启动处理\n", err)
		return &State{Pending: map[string]PendingItem{}}
	}
	if s.Pending == nil {
		s.Pending = map[string]PendingItem{}
	}
	return &s
}

// SaveState writes state atomically (temp file + rename).
func SaveState(path string, s *State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("mkdir state dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "state-*.json")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}
