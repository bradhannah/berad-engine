package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const maxLockSize = 4096

// ErrProjectOpen reports a project already open in another orchestrator.
var ErrProjectOpen = errors.New("project is already open")

// lockInfo is written into the lock file so a second launch can find the
// running orchestrator.
type lockInfo struct {
	Socket string `json:"socket"`
	PID    int    `json:"pid"`
}

// RunningSocket returns the socket of the orchestrator that last held the
// project's lock. The orchestrator may have since exited.
func RunningSocket(projectDir string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(filepath.Clean(projectDir), lockPath))
	if err != nil {
		return "", fmt.Errorf("read project lock: %w", err)
	}
	var info lockInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return "", fmt.Errorf("decode project lock: %w", err)
	}

	return info.Socket, nil
}
