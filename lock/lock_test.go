package lock

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestAcquire_SerialReuse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")

	rel1, err := Acquire(path)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	rel1()

	rel2, err := Acquire(path)
	if err != nil {
		t.Fatalf("second acquire after release: %v", err)
	}
	rel2()
}

func TestAcquire_ConcurrentReturnsBusy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")

	rel1, err := Acquire(path)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer rel1()

	rel2, err := Acquire(path)
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("second acquire: want ErrBusy, got %v", err)
	}
	if rel2 != nil {
		t.Fatalf("second acquire: release fn must be nil on contention")
	}
}

func TestAcquire_MissingDirCreated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "subdir", "test.lock")
	rel, err := Acquire(path)
	if err != nil {
		t.Fatalf("acquire created nested path: %v", err)
	}
	rel()
}
