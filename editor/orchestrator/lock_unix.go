//go:build unix

package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
)

// projectLock keeps one orchestrator per project.
type projectLock struct {
	file *os.File
}

func acquireLock(root *os.Root, info lockInfo) (*projectLock, error) {
	file, err := root.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, filePerm)
	if err != nil {
		return nil, fmt.Errorf("open lock: %w", err)
	}
	descriptor := int(file.Fd())
	if err := syscall.Flock(descriptor, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		existing := readLockInfo(file)
		closeErr := file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errors.Join(fmt.Errorf("%w by process %d", ErrProjectOpen, existing.PID), closeErr)
		}

		return nil, errors.Join(fmt.Errorf("lock project: %w", err), closeErr)
	}
	encoded, err := json.Marshal(info)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("encode lock: %w", err), file.Close())
	}
	if err := writeLockInfo(file, encoded); err != nil {
		return nil, errors.Join(err, file.Close())
	}

	return &projectLock{file: file}, nil
}

func writeLockInfo(file *os.File, encoded []byte) error {
	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("write lock: %w", err)
	}
	if _, err := file.WriteAt(encoded, 0); err != nil {
		return fmt.Errorf("write lock: %w", err)
	}

	return nil
}

func readLockInfo(file *os.File) lockInfo {
	var info lockInfo
	raw, err := io.ReadAll(io.NewSectionReader(file, 0, maxLockSize))
	if err == nil {
		_ = json.Unmarshal(raw, &info) //nolint:errcheck // An unreadable lock only loses the hint.
	}

	return info
}

func (l *projectLock) release() error {
	descriptor := int(l.file.Fd())
	unlockErr := syscall.Flock(descriptor, syscall.LOCK_UN)
	closeErr := l.file.Close()
	if err := errors.Join(unlockErr, closeErr); err != nil {
		return fmt.Errorf("release lock: %w", err)
	}

	return nil
}
