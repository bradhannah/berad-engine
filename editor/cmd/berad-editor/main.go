// Command berad-editor is the shared berad-engine editor. The same program runs
// as the orchestrator ("open") and as each editor window ("window").
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bradhannah/berad-engine/editor/orchestrator"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/window"
	"github.com/bradhannah/berad-engine/engine/ids"
)

const (
	exitUsage   = 2
	buildIDHex  = 16
	sampleWidth = 32
	sampleTall  = 24
	dirPerm     = 0o750
	tapPerm     = 0o600
)

// ErrNoDocuments reports a project with nothing to edit.
var ErrNoDocuments = errors.New("the project has no documents; run with --sample to create one")

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)

		return exitUsage
	}
	var err error
	switch args[0] {
	case "open":
		err = runOpen(args[1:], stderr)
	case "window":
		err = runWindow(args[1:])
	case "help", "-h", "--help":
		usage(stderr)

		return 0
	default:
		usage(stderr)

		return exitUsage
	}
	if err != nil {
		report(stderr, "berad-editor: "+err.Error()+"\n")

		return 1
	}

	return 0
}

func usage(stderr io.Writer) {
	report(stderr, `usage:
  berad-editor open [--project DIR] [--sample] [--tap FILE] [--no-window] [--exit-after DURATION]
      Open a project: start its orchestrator and a Map window.
  berad-editor window --socket PATH --view map --target ID --window ID
      Run one editor window (started by the orchestrator).
`)
}

// plugins is the explicit plug-in list for this program.
func plugins() (*plugin.Registry, error) {
	registry, err := plugin.NewRegistry(tilefloor.Plugin())
	if err != nil {
		return nil, fmt.Errorf("plug-ins: %w", err)
	}

	return registry, nil
}

// buildID identifies this exact executable, so windows from a different build
// are refused by the orchestrator.
func buildID() string {
	path, err := os.Executable()
	if err != nil {
		return "unknown"
	}
	raw, err := os.ReadFile(path) //nolint:gosec // Reads this program's own executable.
	if err != nil {
		return "unknown"
	}
	sum := sha256.Sum256(raw)

	return hex.EncodeToString(sum[:])[:buildIDHex]
}

type openOptions struct {
	project   string
	sample    bool
	tap       string
	noWindow  bool
	exitAfter time.Duration
}

func parseOpen(args []string) (openOptions, error) {
	flags := flag.NewFlagSet("open", flag.ContinueOnError)
	var options openOptions
	flags.StringVar(&options.project, "project", ".", "project folder")
	flags.BoolVar(&options.sample, "sample", false, "create a sample floor if the project is empty")
	flags.StringVar(&options.tap, "tap", "", "write every protocol message to this file")
	flags.BoolVar(&options.noWindow, "no-window", false, "run without opening a window")
	flags.DurationVar(&options.exitAfter, "exit-after", 0, "stop after this long (for smoke tests)")
	if err := flags.Parse(args); err != nil {
		return options, fmt.Errorf("open: %w", err)
	}

	return options, nil
}

func runOpen(args []string, stderr io.Writer) error {
	options, err := parseOpen(args)
	if err != nil {
		return err
	}
	if options.sample {
		if err := os.MkdirAll(options.project, dirPerm); err != nil {
			return fmt.Errorf("create project: %w", err)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if options.exitAfter > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.exitAfter)
		defer cancel()
	}
	cfg, closeTap, err := openConfig(options, stderr)
	if err != nil {
		return err
	}
	defer closeTap()
	orch, err := orchestrator.New(ctx, cfg)
	if errors.Is(err, orchestrator.ErrProjectOpen) {
		return fmt.Errorf("%w: use Cmd+N in one of its Map windows to open another view", err)
	}
	if err != nil {
		return fmt.Errorf("open project: %w", err)
	}
	first, err := firstDocument(orch, options.sample)
	if err != nil {
		return errors.Join(err, orch.Close())
	}
	if !options.noWindow {
		if _, err := orch.LaunchWindow("map", first); err != nil {
			return errors.Join(err, orch.Close())
		}
	}
	report(stderr, "Editing "+options.project+". Close every window or press Ctrl+C to quit.\n")

	if err := orch.Run(ctx); err != nil {
		return fmt.Errorf("editor: %w", err)
	}

	return nil
}

func openConfig(options openOptions, stderr io.Writer) (orchestrator.Config, func(), error) {
	registry, err := plugins()
	if err != nil {
		return orchestrator.Config{}, func() {}, err
	}
	executable, err := os.Executable()
	if err != nil {
		return orchestrator.Config{}, func() {}, fmt.Errorf("find editor executable: %w", err)
	}
	var cfg orchestrator.Config
	cfg.ProjectDir = options.project
	cfg.Registry = registry
	cfg.Build = buildID()
	cfg.Logger = slog.New(slog.NewTextHandler(stderr, nil))
	cfg.Launcher = orchestrator.ExecutableLauncher{Path: executable, Env: nil}
	cfg.QuitWhenLastWindowCloses = !options.noWindow
	closeTap := func() {}
	if options.tap != "" {
		tap, err := os.OpenFile(options.tap, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, tapPerm)
		if err != nil {
			return cfg, closeTap, fmt.Errorf("open tap: %w", err)
		}
		cfg.Tap = tap
		closeTap = func() {
			if err := tap.Close(); err != nil {
				report(stderr, "berad-editor: close tap: "+err.Error()+"\n")
			}
		}
	}

	return cfg, closeTap, nil
}

func firstDocument(orch *orchestrator.Orchestrator, sample bool) (ids.ULID, error) {
	documents, err := orch.Documents()
	if err != nil {
		return ids.ULID{}, fmt.Errorf("list documents: %w", err)
	}
	if len(documents) > 0 {
		return documents[0].ID, nil
	}
	if !sample {
		return ids.ULID{}, ErrNoDocuments
	}
	floor, err := sampleFloor()
	if err != nil {
		return ids.ULID{}, err
	}
	created, err := orch.CreateDocument("Floor 1", floor)
	if err != nil {
		return ids.ULID{}, fmt.Errorf("create sample: %w", err)
	}

	return created, nil
}

// sampleFloor is placeholder content: grass with one walled room and a door.
func sampleFloor() (*tilefloor.Document, error) {
	floor, err := tilefloor.NewDocument(sampleWidth, sampleTall, "grass")
	if err != nil {
		return nil, fmt.Errorf("sample floor: %w", err)
	}
	const left, top, right, bottom, door = 6, 5, 18, 13, 12
	for col := left; col <= right; col++ {
		for row := top; row <= bottom; row++ {
			tile := "floor"
			if col == left || col == right || row == top || row == bottom {
				tile = "wall"
			}
			if row == bottom && col == door {
				tile = "floor"
			}
			if _, err := floor.Floor.Set(col, row, tile); err != nil {
				return nil, fmt.Errorf("sample floor: %w", err)
			}
		}
	}

	return floor, nil
}

func runWindow(args []string) error {
	flags := flag.NewFlagSet("window", flag.ContinueOnError)
	socket := flags.String("socket", "", "orchestrator socket")
	view := flags.String("view", "map", "window view")
	target := flags.String("target", "", "document ID")
	windowID := flags.String("window", "", "window ID")
	flags.String("build", "", "orchestrator build (informational)")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("window: %w", err)
	}
	targetID, err := ids.Parse(*target)
	if err != nil {
		return fmt.Errorf("window target: %w", err)
	}
	parsedWindow, err := ids.Parse(*windowID)
	if err != nil {
		return fmt.Errorf("window id: %w", err)
	}
	registry, err := plugins()
	if err != nil {
		return err
	}
	if err := window.Run(window.Args{
		Socket: *socket, View: *view, Target: targetID, Window: parsedWindow, Build: buildID(),
	}, registry); err != nil {
		return fmt.Errorf("window: %w", err)
	}

	return nil
}

// report writes a message for the person running the editor. If standard
// error itself fails there is nowhere left to report to.
func report(stderr io.Writer, message string) {
	if _, err := io.WriteString(stderr, message); err != nil {
		return
	}
}
