package orchestrator_test

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bradhannah/berad-engine/editor/orchestrator"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/plugins/zone"
)

// The test binary re-runs itself in this role to act as an editor process
// that is killed.
const (
	envRole   = "BERAD_TEST_ROLE"
	envDir    = "BERAD_TEST_DIR"
	envSocket = "BERAD_TEST_SOCKET"

	roleOrchestrator = "orchestrator"
)

func runChildRole(role string) int {
	var err error
	switch role {
	case roleOrchestrator:
		err = childOrchestrator()
	default:
		err = fmt.Errorf("unknown role %q", role)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// childOrchestrator runs until killed, never flushing, so only the journal
// holds its edits.
func childOrchestrator() error {
	registry, err := plugin.NewRegistry(project.Plugin(), tilefloor.Plugin(), zone.Plugin(), textdoc.Plugin())
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer stop()
	orch, err := orchestrator.New(ctx, orchestrator.Config{
		ProjectDir:    os.Getenv(envDir),
		Registry:      registry,
		SocketPath:    os.Getenv(envSocket),
		FlushInterval: time.Hour,
		Logger:        quietLogger(),
	})
	if err != nil {
		return err
	}
	return orch.Run(ctx)
}
