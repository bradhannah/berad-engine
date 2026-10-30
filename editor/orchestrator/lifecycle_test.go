package orchestrator_test

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// dialView connects a window of any view and subscribes to the documents.
func (h *harness) dialView(t testing.TB, view string, target ids.ULID, group string, subscribe ...ids.ULID) *client.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, err := client.Dial(ctx, h.socket, protocol.Hello{
		View: view, Target: target, UndoGroup: group,
	}, newRegistry(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	for _, document := range subscribe {
		if err := connection.Subscribe(ctx, document); err != nil {
			t.Fatal(err)
		}
	}
	return connection
}

func command(t testing.TB, c *client.Client, document ids.ULID, edit, gesture string, args any) (protocol.Result, error) {
	t.Helper()
	data, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.Command(ctx, protocol.Command{Document: document, Edit: edit, Gesture: gesture, Args: data})
}

func mustCommand(t testing.TB, c *client.Client, document ids.ULID, edit, gesture string, args any) protocol.Result {
	t.Helper()
	result, err := command(t, c, document, edit, gesture, args)
	if err != nil {
		t.Fatalf("%s: %v", edit, err)
	}
	return result
}

func encodeDocument(t testing.TB, document interface {
	Encode() (json.RawMessage, error)
}) json.RawMessage {
	t.Helper()
	data, err := document.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// createDocument creates a document through the project index and returns its ID.
func createDocument(t testing.TB, c *client.Client, format, name string, scope ids.ULID, data json.RawMessage, gesture string) ids.ULID {
	t.Helper()
	result := mustCommand(t, c, project.IndexID(), project.CreateEdit, gesture, project.DocumentArgs{
		ID: ids.ULID{}, Format: format, Name: name, Scope: scope, Data: data,
	})
	if result.Created.IsZero() {
		t.Fatal("no ID for the new document")
	}
	return result.Created
}

func newFloorData(t testing.TB) json.RawMessage {
	t.Helper()
	document, err := tilefloor.NewDocument(8, 8, "grass")
	if err != nil {
		t.Fatal(err)
	}
	return encodeDocument(t, document)
}

func indexEntry(c *client.Client, id ids.ULID) (project.Entry, bool) {
	var entry project.Entry
	var found bool
	c.View(project.IndexID(), func(document client.Document) {
		entry, found = document.Model.(*project.Index).Entry(id)
	})
	return entry, found
}

func zoneFloors(c *client.Client, zoneID ids.ULID) []ids.ULID {
	var floors []ids.ULID
	c.View(zoneID, func(document client.Document) { floors = document.Model.(*zone.Document).Zone.Floors() })
	return floors
}

func files(t testing.TB, dir, pattern string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		t.Fatal(err)
	}
	return matches
}

// D4: creating a zone and a floor in one gesture is one undo step; undo
// removes both documents for every window, and redo brings them back with the
// same permanent IDs.
func TestCreateIsOneUndoableStep(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	browser := h.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	watcher := h.dialView(t, "browser", project.IndexID(), "", project.IndexID())

	town := createDocument(t, browser, "zone", "Town", ids.ULID{}, encodeDocument(t, zone.NewDocument()), "new")
	ground := createDocument(t, browser, floor.FormatName, "Ground", town, newFloorData(t), "new")
	mustCommand(t, browser, town, zone.InsertFloorEdit, "new", zone.FloorArgs{Floor: ground, Index: 0})
	waitFor(t, "the watcher to see both documents", func() bool {
		_, hasTown := indexEntry(watcher, town)
		_, hasGround := indexEntry(watcher, ground)
		return hasTown && hasGround
	})

	if _, err := undo(t, browser); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "both documents to go", func() bool {
		_, hasTown := indexEntry(watcher, town)
		_, hasGround := indexEntry(watcher, ground)
		return !hasTown && !hasGround
	})
	if _, err := redo(t, browser); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "both documents to return", func() bool {
		entry, hasTown := indexEntry(watcher, town)
		_, hasGround := indexEntry(watcher, ground)
		return hasTown && hasGround && entry.Name == "Town"
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := watcher.Subscribe(ctx, town); err != nil {
		t.Fatal(err)
	}
	if floors := zoneFloors(watcher, town); len(floors) != 1 || floors[0] != ground {
		t.Fatalf("zone floors after redo %v", floors)
	}
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}
	if len(files(t, dir, "zones/town--*.json")) != 1 || len(files(t, dir, "tilefloors/ground--*.json")) != 1 {
		t.Fatal("new documents were not saved")
	}
}

// D4: deleting a document keeps everything needed to undo it. Windows showing
// it close, and its file moves to the backups instead of being destroyed.
func TestDeleteCanBeUndoneAndClosesItsWindows(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	floorID := createProject(t, dir)
	h := start(t, dir, nil)
	browser := h.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	shown := h.dialView(t, "map", floorID, "", floorID)
	mustPaint(t, shown, floorID, 4, 4, "road", "")

	mustCommand(t, browser, project.IndexID(), project.DeleteEdit, "", project.DocumentArgs{ID: floorID})
	if _, found := indexEntry(browser, floorID); found {
		t.Fatal("deleted document still listed")
	}
	waitForShutdown(t, shown)

	if _, err := undo(t, browser); err != nil {
		t.Fatal(err)
	}
	again := h.dialView(t, "map", floorID, "", floorID)
	if got := tileAt(t, again, floorID, 4, 4); got != "road" || revision(again, floorID) != 1 {
		t.Fatalf("restored floor lost its content: %q revision %d", got, revision(again, floorID))
	}

	mustCommand(t, browser, project.IndexID(), project.DeleteEdit, "", project.DocumentArgs{ID: floorID})
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}
	if len(files(t, dir, "tilefloors/*.json")) != 0 || len(files(t, dir, ".berad/backups/removed/*.bak")) != 1 {
		t.Fatal("deleted file was not moved to the backups")
	}
}

func waitForShutdown(t testing.TB, c *client.Client) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case update, open := <-c.Updates():
			if !open || update.Shutdown != nil {
				return
			}
		case <-deadline:
			t.Fatal("window was not told to close")
		}
	}
}

// D4 and D11: renaming moves the file to the new name; names are unique per
// format, and floor names only within the zone that lists them.
func TestRenameAndNameScopes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	floorID := createProject(t, dir)
	h := start(t, dir, nil)
	browser := h.dialView(t, "browser", project.IndexID(), "", project.IndexID())

	north := createDocument(t, browser, "zone", "North", ids.ULID{}, encodeDocument(t, zone.NewDocument()), "")
	south := createDocument(t, browser, "zone", "South", ids.ULID{}, encodeDocument(t, zone.NewDocument()), "")
	upper := createDocument(t, browser, floor.FormatName, "Upper", north, newFloorData(t), "")
	mustCommand(t, browser, north, zone.InsertFloorEdit, "", zone.FloorArgs{Floor: upper, Index: 0})
	southUpper := createDocument(t, browser, floor.FormatName, "Upper", south, newFloorData(t), "")
	mustCommand(t, browser, south, zone.InsertFloorEdit, "", zone.FloorArgs{Floor: southUpper, Index: 0})

	refused := map[string]project.DocumentArgs{
		"floor name used in the same zone": {Format: floor.FormatName, Name: "upper", Scope: north, Data: newFloorData(t)},
		"zone name used":                   {Format: "zone", Name: "NORTH", Data: encodeDocument(t, zone.NewDocument())},
		"unknown format":                   {Format: "spaceship", Name: "X", Data: json.RawMessage(`{}`)},
		"data that is not a floor":         {Format: floor.FormatName, Name: "Bad", Data: json.RawMessage(`{"width":0}`)},
	}
	for why, args := range refused {
		if _, err := command(t, browser, project.IndexID(), project.CreateEdit, "", args); !errors.Is(err, client.ErrRejected) {
			t.Fatalf("%s: %v", why, err)
		}
	}
	if _, err := command(t, browser, project.IndexID(), project.RenameEdit, "", project.RenameArgs{ID: south, Name: "north"}); err == nil {
		t.Fatal("renamed a zone to a used name")
	}

	mustCommand(t, browser, project.IndexID(), project.RenameEdit, "", project.RenameArgs{ID: floorID, Name: "Upper"})
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}
	if len(files(t, dir, "tilefloors/floor-1--*.json")) != 0 || len(files(t, dir, "tilefloors/upper--"+floorID.Short(8)+".json")) != 1 {
		t.Fatal("renamed file not moved to its new name")
	}
}

// D3: undoing a creation is refused once another window has changed the new
// document, rather than deleting that work.
func TestUndoCreateClashesWithOtherWindows(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	browser := h.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	created := createDocument(t, browser, floor.FormatName, "Fresh", ids.ULID{}, newFloorData(t), "")
	painter := h.dialView(t, "map", created, "", created)
	mustPaint(t, painter, created, 0, 0, "road", "")
	if _, err := undo(t, browser); err == nil || !strings.Contains(err.Error(), "another window") {
		t.Fatalf("undo create after another window's edit: %v", err)
	}
}

// D9: creating, renaming and deleting survive a crash. The project folder is
// copied while the editor runs, as if it had been killed, and reopened.
func TestLifecycleSurvivesACrash(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	original := createProject(t, dir)
	h := start(t, dir, nil)
	browser := h.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	town := createDocument(t, browser, "zone", "Town", ids.ULID{}, encodeDocument(t, zone.NewDocument()), "")
	cellar := createDocument(t, browser, floor.FormatName, "Cellar", town, newFloorData(t), "")
	mustCommand(t, browser, town, zone.InsertFloorEdit, "", zone.FloorArgs{Floor: cellar, Index: 0})
	painter := h.dialView(t, "map", cellar, "", cellar)
	mustPaint(t, painter, cellar, 2, 3, "water", "")
	mustCommand(t, browser, project.IndexID(), project.RenameEdit, "", project.RenameArgs{ID: town, Name: "Village"})
	mustCommand(t, browser, project.IndexID(), project.DeleteEdit, "", project.DocumentArgs{ID: original})

	crashed := t.TempDir()
	copyTree(t, dir, crashed)
	reopened := start(t, crashed, nil)
	view := reopened.dialView(t, "browser", project.IndexID(), "", project.IndexID(), cellar)
	if entry, ok := indexEntry(view, town); !ok || entry.Name != "Village" {
		t.Fatalf("rename lost: %+v %v", entry, ok)
	}
	if _, ok := indexEntry(view, original); ok {
		t.Fatal("deletion lost")
	}
	if got := tileAt(t, view, cellar, 2, 3); got != "water" {
		t.Fatalf("edit to a new document lost: %q", got)
	}
}

func copyTree(t testing.TB, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		if entry.Name() == "orchestrator.lock" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// A crash during a rename can leave two files holding one document; loading
// keeps the newer and moves the other to the backups.
func TestDuplicateFilesResolveToTheNewer(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	id := createProject(t, dir)
	path := files(t, dir, "tilefloors/*.json")[0]
	envelope := readFloorFile(t, dir)
	envelope.Revision = 7
	envelope.Name = "Renamed"
	newer, err := content.Encode(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tilefloors", "renamed--"+id.Short(8)+".json"), newer, 0o600); err != nil {
		t.Fatal(err)
	}
	orch, err := orchestrator.New(context.Background(), baseConfig(t, dir, shortSocket(t)))
	if err != nil {
		t.Fatal(err)
	}
	documents, err := orch.Documents()
	if err != nil || len(documents) != 1 || documents[0].Name != "Renamed" || documents[0].Revision != 7 {
		t.Fatalf("documents %+v %v", documents, err)
	}
	if err := orch.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) || len(files(t, dir, ".berad/backups/removed/*")) != 1 {
		t.Fatal("the older copy was not moved to the backups")
	}
}

// D4: where-used lists the documents that refer to a floor, or to one anchor
// on it. Adding an anchor returns the ID the orchestrator gave it.
func TestWhereUsed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	floorID := createProject(t, dir)
	h := start(t, dir, nil)
	browser := h.dialView(t, "browser", project.IndexID(), "", project.IndexID(), floorID)
	town := createDocument(t, browser, "zone", "Town", ids.ULID{}, encodeDocument(t, zone.NewDocument()), "")
	mustCommand(t, browser, town, zone.InsertFloorEdit, "", zone.FloorArgs{Floor: floorID, Index: 0})
	added := mustCommand(t, browser, floorID, tilefloor.AddAnchorEdit, "", tilefloor.AddAnchorArgs{
		Anchor: floor.Anchor{ID: ids.ULID{}, Name: "Desk", Col: 1, Row: 1}, Index: 0,
	})
	if added.Created.IsZero() {
		t.Fatal("anchor got no ID")
	}
	waitFor(t, "the anchor event", func() bool {
		var count int
		browser.View(floorID, func(document client.Document) {
			count = len(document.Model.(*tilefloor.Document).Floor.Anchors())
		})
		return count == 1
	})
	var anchors []floor.Anchor
	browser.View(floorID, func(document client.Document) { anchors = document.Model.(*tilefloor.Document).Floor.Anchors() })
	if anchors[0].ID != added.Created {
		t.Fatal("the event and the result name different anchors")
	}
	note := createDocument(t, browser, textdoc.FormatName, "Note", ids.ULID{}, encodeDocument(t, textdoc.NewDocument()), "")
	mustCommand(t, browser, note, textdoc.SetLocationEdit, "", textdoc.SetLocationArgs{
		Location: floor.AnchorRef{Floor: floorID, Anchor: added.Created},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	uses, err := browser.WhereUsed(ctx, floorID, "")
	if err != nil || len(uses) != 2 {
		t.Fatalf("uses of the floor %+v %v", uses, err)
	}
	uses, err = browser.WhereUsed(ctx, floorID, added.Created.String())
	if err != nil || len(uses) != 1 || uses[0].Document != note || uses[0].Label != "location" {
		t.Fatalf("uses of the anchor %+v %v", uses, err)
	}
	if uses, err := browser.WhereUsed(ctx, note, ""); err != nil || len(uses) != 0 {
		t.Fatalf("uses of the note %+v %v", uses, err)
	}
}
