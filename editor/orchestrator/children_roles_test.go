package orchestrator_test

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// The test binary re-runs itself in these roles to act as real processes.
const (
	envRole      = "BERAD_TEST_ROLE"
	envDir       = "BERAD_TEST_DIR"
	envSocket    = "BERAD_TEST_SOCKET"
	envMarkerDir = "BERAD_TEST_MARKERS"

	roleOrchestrator = "orchestrator"
	roleCrashWindow  = "crash-window"
	roleIdleWindow   = "idle-window"
	rolePickWindow   = "pick-window"
	envPickFloor     = "BERAD_TEST_PICK_FLOOR"
	envPickItem      = "BERAD_TEST_PICK_ITEM"
)

func runChildRole(role string) int {
	var err error
	switch role {
	case roleOrchestrator:
		err = childOrchestrator()
	case roleCrashWindow:
		err = childWindow(true)
	case roleIdleWindow:
		err = childWindow(false)
	case rolePickWindow:
		err = childPickWindow()
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
		Build:         testBuild,
		SocketPath:    os.Getenv(envSocket),
		FlushInterval: time.Hour,
		Logger:        quietLogger(),
	})
	if err != nil {
		return err
	}
	return orch.Run(ctx)
}

type windowArgs struct {
	socket, view, build string
	window, target      ids.ULID
}

func parseWindowArgs(args []string) (windowArgs, error) {
	var parsed windowArgs
	for index := 0; index+1 < len(args); index++ {
		value := args[index+1]
		var err error
		switch args[index] {
		case "--socket":
			parsed.socket = value
		case "--view":
			parsed.view = value
		case "--build":
			parsed.build = value
		case "--window":
			parsed.window, err = ids.Parse(value)
		case "--target":
			parsed.target, err = ids.Parse(value)
		}
		if err != nil {
			return parsed, err
		}
	}
	return parsed, nil
}

// childWindow connects like a real window. A crashing window panics after
// subscribing; an idle window waits for shutdown and records a clean exit.
func childWindow(crash bool) error {
	args, err := parseWindowArgs(os.Args[1:])
	if err != nil {
		return err
	}
	registry, err := plugin.NewRegistry(project.Plugin(), tilefloor.Plugin(), zone.Plugin(), textdoc.Plugin())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	connection, err := client.Dial(ctx, args.socket, protocol.Hello{
		Build: args.build, Window: args.window, View: args.view, Target: args.target,
	}, registry)
	if err != nil {
		return err
	}
	if err := connection.Subscribe(ctx, args.target); err != nil {
		return err
	}
	if crash {
		panic("simulated window crash")
	}
	for update := range connection.Updates() {
		if update.Shutdown != nil {
			break
		}
	}
	marker := filepath.Join(os.Getenv(envMarkerDir), args.window.String()+".clean")
	return os.WriteFile(marker, []byte("closed cleanly\n"), 0o600)
}

// childPickWindow is a Map window that answers the first pick it is given
// with the floor and item from its environment, then waits for shutdown.
func childPickWindow() error {
	args, err := parseWindowArgs(os.Args[1:])
	if err != nil {
		return err
	}
	registry, err := plugin.NewRegistry(project.Plugin(), tilefloor.Plugin(), zone.Plugin(), textdoc.Plugin())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	connection, err := client.Dial(ctx, args.socket, protocol.Hello{
		Build: args.build, Window: args.window, View: args.view, Target: args.target,
	}, registry)
	if err != nil {
		return err
	}
	floor, err := ids.Parse(os.Getenv(envPickFloor))
	if err != nil {
		return err
	}
	for update := range connection.Updates() {
		if update.Shutdown != nil {
			return nil
		}
		if update.PickStart != nil {
			answer := protocol.PickAnswer{Pick: update.PickStart.Pick, Floor: floor, Item: os.Getenv(envPickItem)}
			if err := connection.PickAnswer(answer); err != nil {
				return err
			}
		}
	}
	return nil
}
