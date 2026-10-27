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
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/views/browser"
	"github.com/bradhannah/berad-engine/editor/views/mapview"
	"github.com/bradhannah/berad-engine/editor/window"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/zone"
)

const (
	exitUsage   = 2
	buildIDHex  = 16
	sampleWidth = 32
	sampleTall  = 24
	dirPerm     = 0o750
	filePerm    = 0o600
	tapPerm     = 0o600
)

// The sample building, in cells from the zone origin, which sits in the
// middle of its room: its walls, door, stairs and a desk inside.
const (
	buildingLeft   = -6
	buildingTop    = -4
	buildingRight  = 6
	buildingBottom = 4
	doorCol        = 0
	deskCol        = -3
	deskRow        = -1
	mediumOffset   = 4
	stairsCol      = 4
	stairsRow      = -2
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
  berad-editor open [--project DIR] [--sample] [--shell] [--tap FILE] [--no-window] [--exit-after DURATION]
      Open a project: start its orchestrator, the project browser and a Map
      window on the first zone. With --shell, open one window instead: the
      project browser, with every window opened after it as one of its tabs.
  berad-editor window --socket PATH --view VIEW --target ID --window ID [--shell]
      Run one editor window (started by the orchestrator).
`)
}

// plugins is the explicit plug-in list for this program.
func plugins() (*plugin.Registry, error) {
	registry, err := plugin.NewRegistry(
		project.Plugin(), tilefloor.Plugin(), zoneplugin.Plugin(), textdoc.Plugin(), workflowplugin.Plugin(),
		npcplugin.Plugin(),
	)
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
	shell     bool
	tap       string
	noWindow  bool
	exitAfter time.Duration
}

func parseOpen(args []string) (openOptions, error) {
	flags := flag.NewFlagSet("open", flag.ContinueOnError)
	var options openOptions
	flags.StringVar(&options.project, "project", ".", "project folder")
	flags.BoolVar(&options.sample, "sample", false, "create a sample zone if the project is empty")
	flags.BoolVar(&options.shell, "shell", false, "open one window with a tab for each window (increment 10)")
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
		return fmt.Errorf("%w: use its project browser to open more windows", err)
	}
	if err != nil {
		return fmt.Errorf("open project: %w", err)
	}
	firstZone, err := ensureDocuments(orch, options.sample, options.project)
	if err != nil {
		return errors.Join(err, orch.Close())
	}
	if !options.noWindow {
		if err := launchFirstWindows(orch, firstZone, options.shell); err != nil {
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

// launchFirstWindows opens the project browser and, when the project has a
// zone, a Map window on it. A shell opens the project browser alone.
func launchFirstWindows(orch *orchestrator.Orchestrator, firstZone ids.ULID, shell bool) error {
	if shell {
		if _, err := orch.LaunchShell(browser.ViewName, project.IndexID()); err != nil {
			return fmt.Errorf("open the project browser: %w", err)
		}

		return nil
	}
	if _, err := orch.LaunchWindow(browser.ViewName, project.IndexID()); err != nil {
		return fmt.Errorf("open the project browser: %w", err)
	}
	if firstZone.IsZero() {
		return nil
	}
	if _, err := orch.LaunchWindow(mapview.ViewName, firstZone); err != nil {
		return fmt.Errorf("open a map: %w", err)
	}

	return nil
}

// ensureDocuments returns the first zone by name, creating the sample project,
// with its audio, in an empty project when asked to.
func ensureDocuments(orch *orchestrator.Orchestrator, sample bool, projectDir string) (ids.ULID, error) {
	documents, err := orch.Documents()
	if err != nil {
		return ids.ULID{}, fmt.Errorf("list documents: %w", err)
	}
	if len(documents) == 0 {
		if !sample {
			return ids.ULID{}, ErrNoDocuments
		}
		if err := sampleAudio(projectDir); err != nil {
			return ids.ULID{}, err
		}

		return createSample(orch)
	}
	for _, document := range documents {
		if document.Format == zone.FormatName {
			return document.ID, nil
		}
	}

	return ids.ULID{}, nil
}

// createSample creates placeholder content: a zone with a walled building on
// the ground floor and its upper floor, an anchor at a desk, and a text
// document. It holds no game content.
func createSample(orch *orchestrator.Orchestrator) (ids.ULID, error) {
	desk, err := orch.NewID()
	if err != nil {
		return ids.ULID{}, fmt.Errorf("create sample: %w", err)
	}
	ground, err := sampleFloor("grass", "wall.a", true, desk)
	if err != nil {
		return ids.ULID{}, err
	}
	upper, err := sampleFloor("", "wall.b", false, ids.ULID{})
	if err != nil {
		return ids.ULID{}, err
	}
	groundID, err := orch.CreateDocument("Ground floor", ground)
	if err != nil {
		return ids.ULID{}, fmt.Errorf("create sample: %w", err)
	}
	upperID, err := orch.CreateDocument("Upper floor", upper)
	if err != nil {
		return ids.ULID{}, fmt.Errorf("create sample: %w", err)
	}
	sampleZone := zoneplugin.NewDocument()
	for index, floorID := range []ids.ULID{groundID, upperID} {
		if err := sampleZone.Zone.Insert(index, floorID); err != nil {
			return ids.ULID{}, fmt.Errorf("create sample: %w", err)
		}
	}
	zoneID, err := orch.CreateDocument("Sample zone", sampleZone)
	if err != nil {
		return ids.ULID{}, fmt.Errorf("create sample: %w", err)
	}
	notes := textdoc.NewDocument()
	notes.Body = "Write here. Enter saves; Cmd+Enter or Ctrl+J starts a new line."
	if _, err := orch.CreateDocument("Notes", notes); err != nil {
		return ids.ULID{}, fmt.Errorf("create sample: %w", err)
	}

	return zoneID, nil
}

// sampleFloor is one floor of the sample building: ground everywhere, walls of
// one family around the building, plain floor inside and stairs joining the
// two floors. The ground floor has a door, two pickups and an anchor at the
// desk.
func sampleFloor(outside, family string, groundFloor bool, desk ids.ULID) (*tilefloor.Document, error) {
	document, err := tilefloor.NewDocumentAt(floor.Centered(sampleWidth, sampleTall), outside)
	if err != nil {
		return nil, fmt.Errorf("sample floor: %w", err)
	}
	if err := sampleBuilding(document.Floor, family, groundFloor); err != nil {
		return nil, err
	}
	if !groundFloor {
		return document, nil
	}
	if err := sampleFurnishing(document.Floor, desk); err != nil {
		return nil, err
	}

	return document, nil
}

// sampleBuilding puts plain floor inside the building and walls around it,
// with a door on the ground floor and stairs up from it to the same cell of
// the floor above.
func sampleBuilding(floorData *floor.Floor, family string, groundFloor bool) error {
	for col := buildingLeft; col <= buildingRight; col++ {
		for row := buildingTop; row <= buildingBottom; row++ {
			if _, err := floorData.SetGround(col, row, "floor"); err != nil {
				return fmt.Errorf("sample floor: %w", err)
			}
			edge := col == buildingLeft || col == buildingRight || row == buildingTop || row == buildingBottom
			if !edge {
				continue
			}
			top := family
			if groundFloor && row == buildingBottom && col == doorCol {
				top = "door"
			}
			if _, err := floorData.SetTop(col, row, top); err != nil {
				return fmt.Errorf("sample floor: %w", err)
			}
		}
	}
	stairs := "stairs.down"
	if groundFloor {
		stairs = "stairs.up"
	}
	if _, err := floorData.SetTop(stairsCol, stairsRow, stairs); err != nil {
		return fmt.Errorf("sample floor: %w", err)
	}

	return nil
}

// sampleFurnishing adds two pickups and an anchor at the desk.
func sampleFurnishing(floorData *floor.Floor, desk ids.ULID) error {
	for _, item := range []struct {
		col, row int
		tile     string
	}{{deskCol + 1, deskRow, "pickup.small"}, {deskCol + mediumOffset, deskRow + 2, "pickup.medium"}} {
		if _, err := floorData.SetTop(item.col, item.row, item.tile); err != nil {
			return fmt.Errorf("sample floor: %w", err)
		}
	}
	if err := floorData.InsertAnchor(0, floor.Anchor{ID: desk, Name: "Desk", Col: deskCol, Row: deskRow}); err != nil {
		return fmt.Errorf("sample floor: %w", err)
	}

	return nil
}

func runWindow(args []string) error {
	flags := flag.NewFlagSet("window", flag.ContinueOnError)
	socket := flags.String("socket", "", "orchestrator socket")
	view := flags.String("view", mapview.ViewName, "window view")
	target := flags.String("target", "", "document ID")
	windowID := flags.String("window", "", "window ID")
	flags.String("build", "", "orchestrator build (informational)")
	shell := flags.Bool("shell", false, "run as a shell, showing other windows as panes")
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
		Socket: *socket, View: *view, Target: targetID, Window: parsedWindow, Build: buildID(), Shell: *shell,
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
