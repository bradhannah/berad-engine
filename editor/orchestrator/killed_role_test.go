package orchestrator_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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

// The test binary re-runs itself in this role to act as an editor process
// that is killed.
const (
	envRole   = "BERAD_TEST_ROLE"
	envDir    = "BERAD_TEST_DIR"
	envFloor  = "BERAD_TEST_FLOOR"
	envMarker = "BERAD_TEST_MARKER"

	roleEditor = "editor"
	// killedPaints is how many cells the killed editor paints.
	killedPaints = 50
)

func runChildRole(role string) int {
	var err error
	switch role {
	case roleEditor:
		err = childEditor()
	default:
		err = fmt.Errorf("unknown role %q", role)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// childEditor runs an orchestrator and a pane in one process, as the editor
// does, never flushing, so only the journal holds its edits. The pane paints
// killedPaints cells, each acknowledged, leaves the marker, and waits to be
// killed.
func childEditor() error {
	registry, err := plugin.NewRegistry(project.Plugin(), tilefloor.Plugin(), zone.Plugin(), textdoc.Plugin())
	if err != nil {
		return err
	}
	floor, err := ids.Parse(os.Getenv(envFloor))
	if err != nil {
		return err
	}
	orch, err := orchestrator.New(orchestrator.Config{
		ProjectDir:    os.Getenv(envDir),
		Registry:      registry,
		FlushInterval: time.Hour,
		Logger:        quietLogger(),
	})
	if err != nil {
		return err
	}
	go func() { _ = orch.Run(context.Background()) }()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pane, err := client.Dial(ctx, orch.Connect, protocol.Hello{View: "map"}, registry)
	if err != nil {
		return err
	}
	if err := pane.Subscribe(ctx, floor); err != nil {
		return err
	}
	for index := range killedPaints {
		cell := tilefloor.Cell{Col: index % 16, Row: index / 16, Layer: tilefloor.LayerGround, Tile: "wall"}
		args, err := json.Marshal(tilefloor.PaintArgs{Cells: []tilefloor.Cell{cell}})
		if err != nil {
			return err
		}
		result, err := pane.Command(ctx, protocol.Command{Document: floor, Edit: tilefloor.PaintEdit, Args: args})
		if err != nil || !result.OK {
			return fmt.Errorf("paint %d: %+v %w", index, result, err)
		}
	}
	if err := os.WriteFile(os.Getenv(envMarker), nil, 0o600); err != nil {
		return err
	}
	<-ctx.Done()
	return ctx.Err()
}
