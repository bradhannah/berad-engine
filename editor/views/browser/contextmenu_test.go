package browser_test

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/ui"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// rightClick presses the right button on a named part of the window.
func (f *fixture) rightClick(name string) {
	f.t.Helper()
	input := ui.NewInput(f.at(name))
	input.Secondary = true
	f.frame(input)
}

// rightClickBelow presses the right button below the list's last row.
func (f *fixture) rightClickBelow() {
	f.t.Helper()
	input := ui.NewInput(image.Pt(40, 600))
	input.Secondary = true
	f.frame(input)
}

// created decodes the one document a test made.
func (f *fixture) created() project.DocumentArgs {
	f.t.Helper()
	var args project.DocumentArgs
	creates := f.requests.Of(project.CreateEdit)
	if len(creates) != 1 || creates[0].Decode(&args) != nil {
		f.t.Fatalf("expected one new document, got %d", len(creates))
	}

	return args
}

// TestRightClickOnASection makes a workflow at the top of its section, even
// when a folder was selected before, and offers a folder of that type.
func TestRightClickOnASection(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withFolders(f)
	f.click("row:Guards", false)
	f.rightClick("section:workflows")
	if _, ok := f.state.Locate("menu:new-folder-workflow", f.ctx.Docs); !ok {
		t.Fatal("the Workflows heading offers a workflow folder")
	}
	f.click("menu:new-workflow", false)
	if args := f.created(); args.Format != flow.FormatName || args.Folder != "" {
		t.Fatalf("a new workflow at the top of Workflows: %+v", args)
	}
}

// TestRightClickOnAFolder selects the folder and makes a workflow inside
// it, and deletes a folder only when it is empty.
func TestRightClickOnAFolder(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withFolders(f)
	f.rightClick("row:Guards")
	if row, ok := f.state.SelectedRow(); !ok || row.ID != "folder:workflow:Market/Guards" {
		t.Fatalf("a right-click selects the folder: %+v", row)
	}
	f.click("menu:new-workflow", false)
	if args := f.created(); args.Folder != "Market/Guards" {
		t.Fatalf("a new workflow inside Guards: %+v", args)
	}
	f.rightClick("row:Guards")
	f.click("menu:context-delete", false)
	if len(f.requests.Of(project.FolderEdit)) != 0 {
		t.Fatal("Delete folder is disabled while the folder holds a workflow")
	}
	f.press("Escape")
	f.rightClick("row:Wilds")
	f.click("menu:context-delete", false)
	var deletion project.FolderArgs
	if folders := f.requests.Of(project.FolderEdit); len(folders) != 1 || folders[0].Decode(&deletion) != nil ||
		deletion.From != "Wilds" || deletion.To != "" {
		t.Fatalf("delete Wilds: %+v", deletion)
	}
}

// TestRightClickOnADocument opens it, starts renaming it, moves it, and
// makes a folder beside it.
func TestRightClickOnADocument(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withFolders(f)
	f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs)
	f.rightClick("row:Clerk day")
	f.click("menu:context-open", false)
	if len(f.requests.Opened) != 1 || f.requests.Opened[0].Target != dayID {
		t.Fatalf("Open opens the workflow: %+v", f.requests.Opened)
	}
	f.rightClick("row:Clerk day")
	f.click("menu:context-rename", false)
	if f.state.Focused() == nil || f.state.Focused().ID != "name" {
		t.Fatal("Rename focuses the name")
	}
	f.press("Escape")
	f.rightClick("row:Clerk day")
	f.click("menu:context-move", false)
	f.click("menu:Wilds", false)
	var move project.MoveArgs
	if moves := f.requests.Of(project.MoveEdit); len(moves) != 1 || moves[0].Decode(&move) != nil ||
		move.ID != dayID || move.Folder != "Wilds" {
		t.Fatalf("Move to… Wilds: %+v", move)
	}
	f.rightClick("row:Clerk day")
	f.click("menu:new-folder-workflow", false)
	var change project.FolderArgs
	if folders := f.requests.Of(project.FolderEdit); len(folders) != 1 || folders[0].Decode(&change) != nil ||
		change.To != "Market/Folder 1" {
		t.Fatalf("a new folder beside Clerk day: %+v", change)
	}
}

// TestRightClickOnAZoneAddsAFloor adds a floor to the zone right-clicked,
// and a floor's menu has no Move to….
func TestRightClickOnAZoneAddsAFloor(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.rightClick("row:Town")
	f.click("menu:new-floor", false)
	if args := f.created(); args.Format != floor.FormatName || args.Scope != zoneID {
		t.Fatalf("a floor in Town: %+v", args)
	}
	f.rightClick("row:Ground")
	if _, ok := f.state.Locate("menu:context-move", f.ctx.Docs); ok {
		t.Fatal("a floor stays with its zone")
	}
}

// TestRightClickBelowTheRows offers what New ▾ makes, without a floor, and
// selects nothing.
func TestRightClickBelowTheRows(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("row:Notes", false)
	f.rightClickBelow()
	if _, ok := f.state.SelectedRow(); ok {
		t.Fatal("a right-click below the rows selects nothing")
	}
	if _, ok := f.state.Locate("menu:new-floor", f.ctx.Docs); ok {
		t.Fatal("no floor without a zone")
	}
	f.click("menu:new-text", false)
	if args := f.created(); args.Folder != "" {
		t.Fatalf("new text at the top: %+v", args)
	}
}

// TestRightClickWaitsForUnsavedChanges asks about a typed name first and
// opens no menu.
func TestRightClickWaitsForUnsavedChanges(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("row:Notes", false)
	f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs)
	f.typeInto("name", " draft")
	f.rightClick("row:Clerk day")
	if _, ok := f.state.Locate("menu:context-open", f.ctx.Docs); ok {
		t.Fatal("the menu waits until the unsaved name is dealt with")
	}
	if row, ok := f.state.SelectedRow(); !ok || row.Label != "Notes" {
		t.Fatalf("Notes stays selected while the guard asks: %+v", row)
	}
}
