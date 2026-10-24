package orchestrator_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
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
		Format: textdoc.FormatName, Name: name, Folder: folder, Data: encodeDocument(t, textdoc.NewDocument()),
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
	makeFolder(t, browser, textdoc.FormatName, "Town")
	makeFolder(t, browser, textdoc.FormatName, "Town/Notes")
	makeFolder(t, browser, textdoc.FormatName, "Wilds")
	plan := createText(t, browser, "Plan", "Town")
	diary := createText(t, browser, "Diary", "")
	mustCommand(t, browser, project.IndexID(), project.MoveEdit, "", project.MoveArgs{ID: diary, Folder: "town/notes"})
	mustCommand(t, browser, project.IndexID(), project.TagsEdit, "", project.TagsArgs{ID: plan, Tags: []string{"quest", "#town"}})
	mustCommand(t, browser, project.IndexID(), project.FolderEdit, "",
		project.FolderArgs{Format: textdoc.FormatName, From: "Town", To: "Village"})
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

	saved := readOne(t, dir, "texts/Village/plan--*.json")
	if !slices.Equal(saved.Tags, []string{"quest", "town"}) {
		t.Fatalf("tags saved with the document: %q", saved.Tags)
	}
	readOne(t, dir, "texts/Village/Notes/diary--*.json")
	if info, err := os.Stat(filepath.Join(dir, "texts", "Wilds")); err != nil || !info.IsDir() {
		t.Fatalf("the empty folder is kept: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "texts", "Town")); !os.IsNotExist(err) {
		t.Fatalf("the renamed folder is gone: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "texts", "Wilds", ".DS_Store"), []byte("finder"), 0o600); err != nil {
		t.Fatal(err)
	}

	reopened := start(t, dir, nil)
	view := reopened.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	if got := folders(view, textdoc.FormatName); !slices.Equal(got, []string{"Village", "Village/Notes", "Wilds"}) {
		t.Fatalf("folders read back: %v", got)
	}
	if entry, _ := indexEntry(view, plan); entry.Folder != "Village" || !slices.Equal(entry.Tags, []string{"quest", "town"}) {
		t.Fatalf("the plan read back: %+v", entry)
	}
	mustCommand(t, view, project.IndexID(), project.FolderEdit, "", project.FolderArgs{Format: textdoc.FormatName, From: "Wilds"})
	if err := reopened.stop(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "texts", "Wilds")); !os.IsNotExist(err) {
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
	makeFolder(t, browser, textdoc.FormatName, "Town")
	plan := createText(t, browser, "Plan", "Town")
	mustCommand(t, browser, project.IndexID(), project.TagsEdit, "", project.TagsArgs{ID: plan, Tags: []string{"quest"}})
	mustCommand(t, browser, project.IndexID(), project.FolderEdit, "",
		project.FolderArgs{Format: textdoc.FormatName, From: "Town", To: "Village"})
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
	makeFolder(t, browser, textdoc.FormatName, "Town")
	makeFolder(t, browser, textdoc.FormatName, "Empty")
	plan := createText(t, browser, "Plan", "")
	mustCommand(t, browser, project.IndexID(), project.MoveEdit, "", project.MoveArgs{ID: plan, Folder: "Town"})
	mustCommand(t, browser, project.IndexID(), project.TagsEdit, "", project.TagsArgs{ID: plan, Tags: []string{"quest"}})
	mustCommand(t, browser, project.IndexID(), project.FolderEdit, "",
		project.FolderArgs{Format: textdoc.FormatName, From: "Town", To: "Village"})

	crashed := t.TempDir()
	copyTree(t, dir, crashed)
	reopened := start(t, crashed, nil)
	view := reopened.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	if got := folders(view, textdoc.FormatName); !slices.Equal(got, []string{"Empty", "Village"}) {
		t.Fatalf("folders after the crash: %v", got)
	}
	if entry, _ := indexEntry(view, plan); entry.Folder != "Village" || !slices.Equal(entry.Tags, []string{"quest"}) {
		t.Fatalf("the plan after the crash: %+v", entry)
	}
	if err := reopened.stop(); err != nil {
		t.Fatal(err)
	}
	readOne(t, crashed, "texts/Village/plan--*.json")
}
