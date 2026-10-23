//go:build !unix

package orchestrator

import (
	"errors"
	"fmt"
	"os"
)

// projectLock keeps one orchestrator per project. Locking is not yet
// implemented on this platform.
type projectLock struct{}

func acquireLock(*os.Root, lockInfo) (*projectLock, error) {
	return nil, fmt.Errorf("project lock: %w", errors.ErrUnsupported)
}

func (*projectLock) release() error { return nil }
