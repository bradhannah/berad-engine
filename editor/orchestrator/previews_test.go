package orchestrator_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Previews only route by zone, so these tests name zones without creating
// them.
var (
	previewZone = ids.MustParse("01J9XK4M7QF2ZB8T5N3R6W1VZA")
	otherZone   = ids.MustParse("01J9XK4M7QF2ZB8T5N3R6W1VZB")
)

func playerAt(run ids.ULID, tick uint64, x float64) protocol.PreviewState {
	return protocol.PreviewState{Run: run, Tick: tick, Entities: []protocol.PreviewEntity{
		{ID: protocol.EntityPlayer, Floor: previewZone, X: x, Y: 0, Face: [2]int{1, 0}},
	}}
}

func startPreview(t *testing.T, c *client.Client, zoneID ids.ULID) (ids.ULID, error) {
	t.Helper()
	result, err := c.PreviewStart(pickContext(t), zoneID)

	return result.Created, err
}

// D7, D13, D15: one preview runs at a time. Its window streams the live state
// through the orchestrator to the zone's other Map windows, a view opened
// later is shown the latest state, and stopping tells the other views.
func TestPreviewStreamsToTheZonesOtherViews(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	group := "map:" + previewZone.String()
	owner := h.dialView(t, orchestrator.MapView, previewZone, group)
	viewer := h.dialView(t, orchestrator.MapView, previewZone, group)
	elsewhere := h.dialView(t, orchestrator.MapView, otherZone, "map:"+otherZone.String())
	text := h.dialView(t, "text", ids.ULID{}, "")

	if _, err := startPreview(t, text, previewZone); err == nil {
		t.Fatal("only a Map window of the zone can preview it")
	}
	run, err := startPreview(t, owner, previewZone)
	if err != nil || run.IsZero() {
		t.Fatalf("start: %v %v", run, err)
	}
	started := nextUpdate(t, viewer, "the preview start", func(u client.Update) bool { return u.PreviewStart != nil })
	if started.PreviewStart.Run != run || started.PreviewStart.Window != owner.Window() {
		t.Fatalf("start %+v", started.PreviewStart)
	}
	if _, err := startPreview(t, elsewhere, otherZone); err == nil {
		t.Fatal("a second preview must be refused")
	}

	if err := owner.PreviewState(playerAt(run, 1, 2.5)); err != nil {
		t.Fatal(err)
	}
	state := nextUpdate(t, viewer, "the state", func(u client.Update) bool { return u.PreviewState != nil })
	if state.PreviewState.Tick != 1 || state.PreviewState.Entities[0].X != 2.5 {
		t.Fatalf("state %+v", state.PreviewState)
	}
	if err := viewer.PreviewState(playerAt(run, 99, 9)); err != nil {
		t.Fatal(err)
	}
	joined := h.dialView(t, orchestrator.MapView, previewZone, group)
	nextUpdate(t, joined, "the start for a new view", func(u client.Update) bool { return u.PreviewStart != nil })
	latest := nextUpdate(t, joined, "the latest state", func(u client.Update) bool { return u.PreviewState != nil })
	if latest.PreviewState.Tick != 1 {
		t.Fatalf("only the running window's states count: %+v", latest.PreviewState)
	}

	if err := owner.PreviewStop(protocol.PreviewStop{Run: run, Reason: "stopped"}); err != nil {
		t.Fatal(err)
	}
	stopped := nextUpdate(t, viewer, "the stop", func(u client.Update) bool { return u.PreviewStop != nil })
	if stopped.PreviewStop.Run != run || stopped.PreviewStop.Reason != "stopped" {
		t.Fatalf("stop %+v", stopped.PreviewStop)
	}
	if _, err := startPreview(t, viewer, previewZone); err != nil {
		t.Fatalf("a new preview after the stop: %v", err)
	}
	for {
		select {
		case update := <-elsewhere.Updates():
			if update.PreviewStart != nil || update.PreviewState != nil || update.PreviewStop != nil {
				t.Fatalf("a map of another zone was sent %+v", update)
			}
		default:
			return
		}
	}
}

// D10, D15: a preview ends when the window running it closes, and the other
// views are told.
func TestPreviewEndsWhenItsWindowCloses(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	group := "map:" + previewZone.String()
	owner := h.dialView(t, orchestrator.MapView, previewZone, group)
	viewer := h.dialView(t, orchestrator.MapView, previewZone, group)
	run, err := startPreview(t, owner, previewZone)
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	stopped := nextUpdate(t, viewer, "the stop", func(u client.Update) bool { return u.PreviewStop != nil })
	if stopped.PreviewStop.Run != run || stopped.PreviewStop.Reason == "" {
		t.Fatalf("stop %+v", stopped.PreviewStop)
	}
	if _, err := startPreview(t, viewer, previewZone); err != nil {
		t.Fatalf("a new preview after the window closed: %v", err)
	}
}
