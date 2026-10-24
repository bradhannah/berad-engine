package project_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
)

func folderIndex(t *testing.T) (*project.Index, *plugin.Registry) {
	t.Helper()
	registry, err := plugin.NewRegistry(project.Plugin())
	if err != nil {
		t.Fatal(err)
	}
	index := project.NewIndexWithFolders([]project.Entry{
		{ID: id(1), Name: "Guard day", Format: "workflow", Folder: "Town/Guards"},
		{ID: id(2), Name: "Baker day", Format: "workflow", Folder: "Town"},
		{ID: id(3), Name: "Notes", Format: "text"},
	}, []project.Folder{
		{Format: "workflow", Path: "Town"}, {Format: "workflow", Path: "Town/Guards"},
		{Format: "workflow", Path: "Wilds"}, {Format: "text", Path: "Town"},
	})

	return index, registry
}

func entry(t *testing.T, index *project.Index, seed byte) project.Entry {
	t.Helper()
	found, ok := index.Entry(id(seed))
	if !ok {
		t.Fatalf("no entry %d", seed)
	}

	return found
}

// TestFoldersAreMadeMovedAndDeleted makes a folder inside another, refuses
// one whose name is taken (ignoring case) or whose parent is missing, moves
// a folder with what it holds, refuses moving it into itself, and deletes
// only an empty folder. Each edit undoes exactly.
func TestFoldersAreMadeMovedAndDeleted(t *testing.T) {
	t.Parallel()
	index, registry := folderIndex(t)
	change := project.FolderChange{}
	make := encode(t, project.FolderArgs{Format: "workflow", From: "", To: "town/Night watch"})
	if err := model.VerifyUndo(index, change, make, registry.Edit); err != nil {
		t.Fatal(err)
	}
	if _, err := change.Apply(index, make); err != nil {
		t.Fatal(err)
	}
	if got := index.Folders("workflow"); !slices.Equal(got, []string{"Town", "Town/Guards", "Town/Night watch", "Wilds"}) {
		t.Fatalf("made inside Town, spelled as Town was: %v", got)
	}
	for _, bad := range []project.FolderArgs{
		{Format: "workflow", To: "WILDS"},
		{Format: "workflow", To: "Nowhere/Camp"},
		{Format: "workflow", To: "Bad: name"},
		{Format: "workflow", To: ".hidden"},
		{Format: "workflow", From: "Town", To: "Town/Guards/Town"},
		{Format: "workflow", From: "Town", To: "Wilds"},
		{Format: "workflow", From: "Town"},
		{Format: "workflow"},
	} {
		if _, err := change.Apply(index, encode(t, bad)); !errors.Is(err, project.ErrFolder) {
			t.Fatalf("%+v: %v", bad, err)
		}
	}
	move := encode(t, project.FolderArgs{Format: "workflow", From: "Town", To: "Wilds/Old town"})
	if err := model.VerifyUndo(index, change, move, registry.Edit); err != nil {
		t.Fatal(err)
	}
	if _, err := change.Apply(index, move); err != nil {
		t.Fatal(err)
	}
	if guard, baker := entry(t, index, 1), entry(t, index, 2); guard.Folder != "Wilds/Old town/Guards" ||
		baker.Folder != "Wilds/Old town" {
		t.Fatalf("documents move with the folder: %q %q", guard.Folder, baker.Folder)
	}
	if !index.HasFolder("text", "town") || index.HasFolder("workflow", "Town") {
		t.Fatal("folders of another type stay where they are")
	}
	remove := encode(t, project.FolderArgs{Format: "workflow", From: "Wilds/Old town/Night watch"})
	if err := model.VerifyUndo(index, change, remove, registry.Edit); err != nil {
		t.Fatal(err)
	}
	rename := encode(t, project.FolderArgs{Format: "workflow", From: "Wilds", To: "wilds"})
	if _, err := change.Apply(index, rename); err != nil {
		t.Fatalf("a folder can change the case of its own name: %v", err)
	}
}

// TestDocumentsMoveAndCarryTags moves a document into a folder and back to
// the top, refuses a folder that does not exist, and sets tags tidied:
// trimmed, without "#", without repeats.
func TestDocumentsMoveAndCarryTags(t *testing.T) {
	t.Parallel()
	index, registry := folderIndex(t)
	move := encode(t, project.MoveArgs{ID: id(3), Folder: "town"})
	if err := model.VerifyUndo(index, project.Move{}, move, registry.Edit); err != nil {
		t.Fatal(err)
	}
	if _, err := (project.Move{}).Apply(index, move); err != nil || entry(t, index, 3).Folder != "Town" {
		t.Fatalf("moved into Town: %v %q", err, entry(t, index, 3).Folder)
	}
	if _, err := (project.Move{}).Apply(index, encode(t, project.MoveArgs{ID: id(3), Folder: "Wilds"})); err == nil {
		t.Fatal("text has no Wilds folder")
	}
	retag := encode(t, project.TagsArgs{ID: id(1), Tags: []string{" town ", "#guards", "Town", ""}})
	if err := model.VerifyUndo(index, project.Retag{}, retag, registry.Edit); err != nil {
		t.Fatal(err)
	}
	if _, err := (project.Retag{}).Apply(index, retag); err != nil {
		t.Fatal(err)
	}
	if tags := entry(t, index, 1).Tags; !slices.Equal(tags, []string{"town", "guards"}) {
		t.Fatalf("tags %q", tags)
	}
	if _, err := (project.Retag{}).Apply(index, encode(t, project.TagsArgs{ID: id(1), Tags: []string{"a,b"}})); err == nil {
		t.Fatal("a comma separates tags, so a tag holds none")
	}
	decoded, err := project.Decode(mustEncode(t, index))
	if err != nil {
		t.Fatal(err)
	}
	copied, ok := decoded.(*project.Index)
	if !ok || !slices.Equal(copied.Folders("workflow"), index.Folders("workflow")) ||
		!slices.Equal(entry(t, copied, 1).Tags, []string{"town", "guards"}) {
		t.Fatal("folders and tags survive a snapshot")
	}
}

func mustEncode(t *testing.T, index *project.Index) []byte {
	t.Helper()
	data, err := index.Encode()
	if err != nil {
		t.Fatal(err)
	}

	return data
}

// TestFolderNamesAndPaths checks the rules for folder names and the path
// helpers.
func TestFolderNamesAndPaths(t *testing.T) {
	t.Parallel()
	for _, good := range []string{"Town", "Night watch", "Café", "a-b_c (2)"} {
		if err := project.CheckFolderName(good); err != nil {
			t.Fatalf("%q: %v", good, err)
		}
	}
	for _, bad := range []string{"", " Town", "Town ", "a/b", `a\b`, "a:b", ".git", "end.", "a?b", "tab\there"} {
		if err := project.CheckFolderName(bad); err == nil {
			t.Fatalf("%q is not a folder name", bad)
		}
	}
	if project.FolderParent("A/B/C") != "A/B" || project.FolderName("A/B/C") != "C" || project.FolderParent("A") != "" ||
		project.JoinFolder("", "A") != "A" || project.JoinFolder("A", "B") != "A/B" {
		t.Fatal("path helpers")
	}
	if !project.Within("Town/Guards", "town") || project.Within("Townhall", "Town") || !project.Within("x", "") {
		t.Fatal("Within")
	}
	if project.ComparePaths("Town", "Town/Guards") >= 0 || project.ComparePaths("Town hall", "Town/Guards") <= 0 {
		t.Fatal("a folder sorts before what it holds")
	}
}
