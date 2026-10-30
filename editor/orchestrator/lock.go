package orchestrator

import (
	"errors"
)

const maxLockSize = 4096

// ErrProjectOpen reports a project already open in another orchestrator.
var ErrProjectOpen = errors.New("project is already open")

// lockInfo is written into the lock file so a second launch can say which
// process has the project open.
type lockInfo struct {
	PID int `json:"pid"`
}
