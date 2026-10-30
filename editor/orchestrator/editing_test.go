package orchestrator_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	"github.com/bradhannah/berad-engine/editor/protocol"
)

// D1: strict updates. A window's copy changes only from the orchestrator's
// event, and every subscribed window receives it.
func TestPaintReachesEveryView(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	first := h.dial(t, "map:shared")
	second := h.dial(t, "map:shared")

	result := mustPaint(t, first, h.floor, 3, 4, "wall", "")
	if result.Revision != 1 {
		t.Fatalf("revision = %d", result.Revision)
	}
	if got := tileAt(t, first, h.floor, 3, 4); got != "wall" {
		t.Fatalf("origin sees %q", got)
	}
	waitFor(t, "second view", func() bool { return revision(second, h.floor) == 1 })
	if got := tileAt(t, second, h.floor, 3, 4); got != "wall" {
		t.Fatalf("second view sees %q", got)
	}
}

// D14: a refused edit leaves everything unchanged and explains why.
func TestRefusedEditExplainsWhy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	view := h.dial(t, "")
	result, err := paint(t, view, h.floor, 99, 0, "wall", "")
	if !errors.Is(err, client.ErrRejected) || result.OK || !strings.Contains(result.Reason, "refused") {
		t.Fatalf("result %+v err %v", result, err)
	}
	if revision(view, h.floor) != 0 {
		t.Fatal("refused edit changed the revision")
	}
}

func TestUndoAndRedo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	view := h.dial(t, "")
	mustPaint(t, view, h.floor, 1, 1, "wall", "")
	mustPaint(t, view, h.floor, 1, 1, "road", "")

	if _, err := undo(t, view); err != nil {
		t.Fatal(err)
	}
	if got := tileAt(t, view, h.floor, 1, 1); got != "wall" {
		t.Fatalf("after one undo %q", got)
	}
	if _, err := undo(t, view); err != nil {
		t.Fatal(err)
	}
	if got := tileAt(t, view, h.floor, 1, 1); got != "grass" {
		t.Fatalf("after two undos %q", got)
	}
	if _, err := undo(t, view); err == nil {
		t.Fatal("undo with nothing to undo succeeded")
	}
	if _, err := redo(t, view); err != nil {
		t.Fatal(err)
	}
	if got := tileAt(t, view, h.floor, 1, 1); got != "wall" {
		t.Fatalf("after redo %q", got)
	}
	mustPaint(t, view, h.floor, 2, 2, "wall", "")
	if _, err := redo(t, view); err == nil {
		t.Fatal("a new edit should clear redo")
	}
}

// D1 note: a whole paint stroke (one gesture) is one undo step.
func TestGestureIsOneUndoStep(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	view := h.dial(t, "")
	for col := range 6 {
		mustPaint(t, view, h.floor, col, 0, "wall", "stroke-1")
	}
	if _, err := undo(t, view); err != nil {
		t.Fatal(err)
	}
	for col := range 6 {
		if got := tileAt(t, view, h.floor, col, 0); got != "grass" {
			t.Fatalf("col %d = %q after undoing the stroke", col, got)
		}
	}
	if _, err := redo(t, view); err != nil {
		t.Fatal(err)
	}
	for col := range 6 {
		if got := tileAt(t, view, h.floor, col, 0); got != "wall" {
			t.Fatalf("col %d = %q after redoing the stroke", col, got)
		}
	}
}

// D3: views of the same map share one undo list.
func TestViewsOfOneMapShareUndo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	viewA := h.dial(t, "map:floor")
	viewB := h.dial(t, "map:floor")
	mustPaint(t, viewA, h.floor, 0, 0, "brick", "")
	mustPaint(t, viewB, h.floor, 0, 0, "concrete", "")

	if _, err := undo(t, viewA); err != nil {
		t.Fatal(err)
	}
	if got := tileAt(t, viewA, h.floor, 0, 0); got != "brick" {
		t.Fatalf("undo in A should take back B's latest change, got %q", got)
	}
}

// D3: separate windows each undo only their own edits, and an undo that would
// overwrite a later change from another window is refused with a message.
func TestUndoClashIsRefused(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	mapWindow := h.dial(t, "")
	other := h.dial(t, "")
	mustPaint(t, mapWindow, h.floor, 5, 5, "brick", "")
	mustPaint(t, other, h.floor, 5, 5, "concrete", "")

	result, err := undo(t, mapWindow)
	if err == nil || !strings.Contains(result.Reason, "another window") {
		t.Fatalf("expected a clash, got %+v %v", result, err)
	}
	waitFor(t, "B's paint", func() bool { return revision(mapWindow, h.floor) == 2 })
	if got := tileAt(t, mapWindow, h.floor, 5, 5); got != "concrete" {
		t.Fatalf("clash must leave B's change, got %q", got)
	}
}

func TestUndoWithoutClashOnOtherCells(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	first := h.dial(t, "")
	second := h.dial(t, "")
	mustPaint(t, first, h.floor, 1, 0, "brick", "")
	mustPaint(t, second, h.floor, 2, 0, "concrete", "")
	if _, err := undo(t, first); err != nil {
		t.Fatalf("undo of an untouched cell: %v", err)
	}
	if got := tileAt(t, first, h.floor, 1, 0); got != "grass" {
		t.Fatalf("got %q", got)
	}
}

// D3: undo keeps the configured number of steps (default 100).
func TestUndoLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, func(cfg *orchestrator.Config) { cfg.UndoLimit = 3 })
	view := h.dial(t, "")
	for col := range 5 {
		mustPaint(t, view, h.floor, col, 0, "wall", "")
	}
	for range 3 {
		if _, err := undo(t, view); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := undo(t, view); err == nil {
		t.Fatal("undo beyond the limit succeeded")
	}
}

// D3: closing a window ends its undo list, unless another view shares it.
func TestClosingWindowEndsItsUndo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	solo := h.dial(t, "map:solo")
	mustPaint(t, solo, h.floor, 0, 0, "wall", "")
	if err := solo.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := h.dial(t, "map:solo")
	waitFor(t, "previous window gone", func() bool {
		_, err := undo(t, reopened)
		return err != nil
	})

	keeper := h.dial(t, "map:kept")
	leaver := h.dial(t, "map:kept")
	mustPaint(t, leaver, h.floor, 1, 1, "wall", "")
	if err := leaver.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := undo(t, keeper); err != nil {
		t.Fatalf("shared view should still undo: %v", err)
	}
}

func TestHandshakeRefusesOtherProtocol(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.Dial(ctx, h.socket, protocol.Hello{Protocol: protocol.Version - 1, View: "map"}, newRegistry(t))
	if !errors.Is(err, client.ErrRefused) || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("err = %v", err)
	}
}

func TestSessionStateReachesOtherWindows(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	sender := h.dial(t, "")
	receiver := h.dial(t, "")
	if err := sender.Publish("clock", []byte(`"09:12"`)); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(5 * time.Second)
	for {
		select {
		case update := <-receiver.Updates():
			if update.Session != nil && update.Session.Topic == "clock" {
				if string(update.Session.Value) != `"09:12"` {
					t.Fatalf("value %s", update.Session.Value)
				}
				return
			}
		case <-deadline:
			t.Fatal("session update not received")
		}
	}
}
