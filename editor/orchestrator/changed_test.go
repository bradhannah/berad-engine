package orchestrator_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/orchestrator"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
)

// A document's last change is saved with it and listed in the project
// index, which windows hear of once a day rather than at every edit.
func TestChangesAreDatedAndListedOnceADay(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	monday := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	var now atomic.Int64
	now.Store(monday.UnixNano())
	clock := func() time.Time { return time.Unix(0, now.Load()).UTC() }
	h := start(t, dir, func(cfg *orchestrator.Config) { cfg.Clock = clock })
	browser := h.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	plan := createText(t, browser, "Plan", "")
	if entry, _ := indexEntry(browser, plan); !entry.Changed.Equal(monday) {
		t.Fatalf("a new document changed when it was made: %v", entry.Changed)
	}
	now.Store(monday.Add(2 * time.Hour).UnixNano())
	mustCommand(t, browser, plan, textdoc.SetBodyEdit, "", textdoc.SetBodyArgs{Body: "Later that morning"})
	if entry, _ := indexEntry(browser, plan); !entry.Changed.Equal(monday) {
		t.Fatalf("the index says so once a day: %v", entry.Changed)
	}
	tuesday := monday.Add(25 * time.Hour)
	now.Store(tuesday.UnixNano())
	mustCommand(t, browser, plan, textdoc.SetBodyEdit, "", textdoc.SetBodyArgs{Body: "The next day"})
	waitFor(t, "the index to say it changed on Tuesday", func() bool {
		entry, _ := indexEntry(browser, plan)

		return entry.Changed.Equal(tuesday)
	})
	later := tuesday.Add(time.Hour)
	now.Store(later.UnixNano())
	mustCommand(t, browser, plan, textdoc.SetBodyEdit, "", textdoc.SetBodyArgs{Body: "An hour on"})
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}
	if saved := readOne(t, dir, "texts/plan--*.json"); !saved.Changed.Equal(later) {
		t.Fatalf("the file keeps the time of the last change: %v", saved.Changed)
	}
	reopened := start(t, dir, func(cfg *orchestrator.Config) { cfg.Clock = clock })
	view := reopened.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	if entry, _ := indexEntry(view, plan); !entry.Changed.Equal(later) {
		t.Fatalf("reopened, the index lists the saved time: %v", entry.Changed)
	}
}
