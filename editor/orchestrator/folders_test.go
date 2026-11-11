package orchestrator_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

func folders(c *client.Client, format string) []string {
	var paths []string
	c.View(project.IndexID(), func(document client.Document) { paths = document.Model.(*project.Index).Folders(format) })
	return paths
}

func makeFolder(t *testing.T, c *client.Client, format, path string) {
	t.Helper()
	mustCommand(t, c, project.IndexID(), project.FolderEdit, "", project.FolderArgs{Format: format, To: path})
}

func createText(t *testing.T, c *client.Client, name, folder string) ids.ULID {
	t.Helper()
	result := mustCommand(t, c, project.IndexID(), project.CreateEdit, "", project.DocumentArgs{
		Format: noteFormat, Name: name, Folder: folder, Data: encodeDocument(t, newNote()),
	})
	return result.Created
}

func readOne(t *testing.T, dir, pattern string) content.Envelope {
	t.Helper()
	matches := files(t, dir, pattern)
	if len(matches) != 1 {
		t.Fatalf("%s: %v", pattern, matches)
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := content.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return envelope
}

// Folders are real folders on disk: documents made in a folder or moved into
// one are saved inside it, a renamed folder moves its documents and
// folders, an empty folder is kept, a deleted one is removed, tags are saved
// with their document, and all of it is read back when the project opens. A
// floor stays with its zone.
func TestFoldersAndTagsAreSavedAsFolders(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	browser := h.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	makeFolder(t, browser, noteFormat, "Town")
	makeFolder(t, browser, noteFormat, "Town/Notes")
	makeFolder(t, browser, noteFormat, "Wilds")
	plan := createText(t, browser, "Plan", "Town")
	diary := createText(t, browser, "Diary", "")
	mustCommand(t, browser, project.IndexID(), project.MoveEdit, "", project.MoveArgs{ID: diary, Folder: "town/notes"})
	mustCommand(t, browser, project.IndexID(), project.TagsEdit, "", project.TagsArgs{ID: plan, Tags: []string{"quest", "#town"}})
	mustCommand(t, browser, project.IndexID(), project.FolderEdit, "",
		project.FolderArgs{Format: noteFormat, From: "Town", To: "Village"})
	if entry, _ := indexEntry(browser, diary); entry.Folder != "Village/Notes" {
		t.Fatalf("the diary moved with its folder: %q", entry.Folder)
	}
	town := createDocument(t, browser, "zone", "Harbour", ids.ULID{}, encodeDocument(t, zone.NewDocument()), "")
	quay := createDocument(t, browser, floor.FormatName, "Quay", town, newFloorData(t), "")
	mustCommand(t, browser, town, zone.InsertFloorEdit, "", zone.FloorArgs{Floor: quay, Index: 0})
	makeFolder(t, browser, floor.FormatName, "Docks")
	if _, err := command(t, browser, project.IndexID(), project.MoveEdit, "", project.MoveArgs{ID: quay, Folder: "Docks"}); err == nil {
		t.Fatal("a floor stays with its zone")
	}
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}

	saved := readOne(t, dir, "notes/Village/plan--*.json")
	if !slices.Equal(saved.Tags, []string{"quest", "town"}) {
		t.Fatalf("tags saved with the document: %q", saved.Tags)
	}
	readOne(t, dir, "notes/Village/Notes/diary--*.json")
	if info, err := os.Stat(filepath.Join(dir, "notes", "Wilds")); err != nil || !info.IsDir() {
		t.Fatalf("the empty folder is kept: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "notes", "Town")); !os.IsNotExist(err) {
		t.Fatalf("the renamed folder is gone: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes", "Wilds", ".DS_Store"), []byte("finder"), 0o600); err != nil {
		t.Fatal(err)
	}

	reopened := start(t, dir, nil)
	view := reopened.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	if got := folders(view, noteFormat); !slices.Equal(got, []string{"Village", "Village/Notes", "Wilds"}) {
		t.Fatalf("folders read back: %v", got)
	}
	if entry, _ := indexEntry(view, plan); entry.Folder != "Village" || !slices.Equal(entry.Tags, []string{"quest", "town"}) {
		t.Fatalf("the plan read back: %+v", entry)
	}
	mustCommand(t, view, project.IndexID(), project.FolderEdit, "", project.FolderArgs{Format: noteFormat, From: "Wilds"})
	if err := reopened.stop(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "notes", "Wilds")); !os.IsNotExist(err) {
		t.Fatalf("the deleted folder is removed, with only Finder's file in it: %v", err)
	}
}

// Undo puts a renamed folder back, with what it holds, and a deleted
// document comes back in its folder with its tags.
func TestFolderChangesUndo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	browser := h.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	makeFolder(t, browser, noteFormat, "Town")
	plan := createText(t, browser, "Plan", "Town")
	mustCommand(t, browser, project.IndexID(), project.TagsEdit, "", project.TagsArgs{ID: plan, Tags: []string{"quest"}})
	mustCommand(t, browser, project.IndexID(), project.FolderEdit, "",
		project.FolderArgs{Format: noteFormat, From: "Town", To: "Village"})
	if _, err := undo(t, browser); err != nil {
		t.Fatal(err)
	}
	if entry, _ := indexEntry(browser, plan); entry.Folder != "Town" {
		t.Fatalf("undo puts the folder back: %q", entry.Folder)
	}
	mustCommand(t, browser, project.IndexID(), project.DeleteEdit, "", project.DocumentArgs{ID: plan})
	if _, err := undo(t, browser); err != nil {
		t.Fatal(err)
	}
	if entry, _ := indexEntry(browser, plan); entry.Folder != "Town" || !slices.Equal(entry.Tags, []string{"quest"}) {
		t.Fatalf("the deleted plan comes back in its folder with its tags: %+v", entry)
	}
}

// Folder changes not yet saved come back from the journal after a crash.
func TestFoldersSurviveACrash(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	browser := h.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	makeFolder(t, browser, noteFormat, "Town")
	makeFolder(t, browser, noteFormat, "Empty")
	plan := createText(t, browser, "Plan", "")
	mustCommand(t, browser, project.IndexID(), project.MoveEdit, "", project.MoveArgs{ID: plan, Folder: "Town"})
	mustCommand(t, browser, project.IndexID(), project.TagsEdit, "", project.TagsArgs{ID: plan, Tags: []string{"quest"}})
	mustCommand(t, browser, project.IndexID(), project.FolderEdit, "",
		project.FolderArgs{Format: noteFormat, From: "Town", To: "Village"})

	crashed := t.TempDir()
	copyTree(t, dir, crashed)
	reopened := start(t, crashed, nil)
	view := reopened.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	if got := folders(view, noteFormat); !slices.Equal(got, []string{"Empty", "Village"}) {
		t.Fatalf("folders after the crash: %v", got)
	}
	if entry, _ := indexEntry(view, plan); entry.Folder != "Village" || !slices.Equal(entry.Tags, []string{"quest"}) {
		t.Fatalf("the plan after the crash: %+v", entry)
	}
	if err := reopened.stop(); err != nil {
		t.Fatal(err)
	}
	readOne(t, crashed, "notes/Village/plan--*.json")
}

// names lists what a directory holds, by name as spelled on disk.
func names(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, entry := range entries {
		found = append(found, entry.Name())
	}
	return found
}

// A folder renamed only in its capitals keeps what it holds. Where the file
// system ignores case, "Town" and "town" are one directory, so the documents'
// old and new paths name the same files: they are saved, not moved to the
// backups, an empty folder stays, and the directories take the new spelling.
func TestRenamingAFoldersCapitalsKeepsWhatItHolds(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	browser := h.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	makeFolder(t, browser, noteFormat, "Town")
	makeFolder(t, browser, noteFormat, "Town/Notes")
	makeFolder(t, browser, noteFormat, "Empty")
	plan := createText(t, browser, "Plan", "Town")
	diary := createText(t, browser, "Diary", "Town/Notes")
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}
	readOne(t, dir, "notes/Town/plan--*.json")

	renamed := start(t, dir, nil)
	view := renamed.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	mustCommand(t, view, project.IndexID(), project.FolderEdit, "",
		project.FolderArgs{Format: noteFormat, From: "Town", To: "town"})
	mustCommand(t, view, project.IndexID(), project.FolderEdit, "",
		project.FolderArgs{Format: noteFormat, From: "Empty", To: "empty"})
	if err := renamed.stop(); err != nil {
		t.Fatal(err)
	}
	if saved := readOne(t, dir, "notes/*/plan--*.json"); saved.ID != plan {
		t.Fatalf("the plan is still in its folder: %v", saved.ID)
	}
	if saved := readOne(t, dir, "notes/*/*/diary--*.json"); saved.ID != diary {
		t.Fatalf("the diary is still in its folder: %v", saved.ID)
	}
	if got := names(t, filepath.Join(dir, "notes")); !slices.Equal(got, []string{"empty", "town"}) {
		t.Fatalf("the folders on disk, as respelled: %v", got)
	}

	reopened := start(t, dir, nil)
	again := reopened.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	if got := folders(again, noteFormat); !slices.Equal(got, []string{"empty", "town", "town/Notes"}) {
		t.Fatalf("folders read back: %v", got)
	}
	if entry, found := indexEntry(again, plan); !found || entry.Folder != "town" {
		t.Fatalf("the plan read back: %+v %v", entry, found)
	}
	if entry, found := indexEntry(again, diary); !found || entry.Folder != "town/Notes" {
		t.Fatalf("the diary read back: %+v %v", entry, found)
	}
}
