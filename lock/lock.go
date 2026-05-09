// Package lock provides a single-instance file lock so two concurrent
// weops-inspect runs cannot corrupt the notify state.json (read/decide/write
// has a TOCTOU window otherwise). Implemented via flock(2): acquired with
// LOCK_EX | LOCK_NB on a sentinel file. The kernel releases the lock
// automatically when the process exits, so SIGKILL leaves no zombie.
package lock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// ErrBusy is returned by Acquire when another process already holds the lock.
var ErrBusy = errors.New("another instance is running")

// Acquire takes an exclusive non-blocking flock on path. The parent directory
// is created if missing. On success it returns a release function the caller
// MUST call (typically via defer) on normal exit; the kernel handles abnormal
// exit. On contention it returns ErrBusy. Other errors (e.g. parent dir
// unwritable) are surfaced for the caller to decide whether to bail or
// degrade.
func Acquire(path string) (release func(), err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("ensure lock dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, fmt.Errorf("flock: %w", err)
	}

	release = func() {
		// Best-effort: ignore errors. Kernel will release on process exit.
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
	}
	return release, nil
}
