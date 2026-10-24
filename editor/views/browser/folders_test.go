package browser_test

import (
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

var watchID = frametest.ID(6)

// withFolders gives the fixture's project workflow folders Market and
// Market/Guards and an empty Wilds: Clerk day in Market, Night watch tagged
// "night" and "guards" in Market/Guards.
func withFolders(f *fixture) {
	docs := f.ctx.Docs.(*frametest.Docs)
	docs.Put(project.IndexID(), "Project", project.NewIndexWithFolders([]project.Entry{
		{ID: textID, Name: "Notes", Format: textdoc.FormatName}, {ID: zoneID, Name: "Town", Format: "zone"},
		{ID: floorID, Name: "Ground", Format: "tilefloor"}, {ID: clerkID, Name: "Ada Clerk", Format: "npc"},
		{ID: dayID, Name: "Clerk day", Format: flow.FormatName, Folder: "Market"},
		{ID: watchID, Name: "Night watch", Format: flow.FormatName, Folder: "Market/Guards", Tags: []string{"night", "guards"}},
	}, []project.Folder{
		{Format: flow.FormatName, Path: "Market"}, {Format: flow.FormatName, Path: "Market/Guards"},
		{Format: flow.FormatName, Path: "Wilds"},
	}))
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

func (f *fixture) frame(input ui.Input) { f.state.Update(&input, f.ctx) }

// workflowRows lists the Workflows section's rows as "label/indent".
func (f *fixture) workflowRows() []string {
	var shown []string
	inside := false
	for _, row := range f.state.Rows(f.ctx.Docs) {
		if row.Section {
			inside = row.ID == "section:workflows"

			continue
		}
		if inside {
			shown = append(shown, row.Label+"/"+string(rune('0'+row.Indent)))
		}
	}

	return shown
}

func (f *fixture) typeInto(field, text string) {
	f.t.Helper()
	f.click("ui:"+field, false)
	f.typeText(text)
}

func (f *fixture) press(key string) {
	input := ui.NewInput(image.Pt(0, 0))
	input.Presses = []ui.Press{ui.ParsePress(key)}
	f.frame(input)
}

func (f *fixture) at(name string) image.Point {
	f.t.Helper()
	point, ok := f.state.Locate(name, f.ctx.Docs)
	if !ok {
		f.t.Fatalf("cannot locate %s", name)
	}

	return point
}

// TestFoldersInTheTree lists each folder before what it holds and folders
// before documents, indented under their folder, and a folder closes.
func TestFoldersInTheTree(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withFolders(f)
	want := []string{"Market/0", "Guards/1", "Night watch/2", "Clerk day/1", "Wilds/0"}
	if got := f.workflowRows(); !slices.Equal(got, want) {
		t.Fatalf("rows %v", got)
	}
	f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs)
	f.click("toggle:Market", false)
	if got := f.workflowRows(); !slices.Equal(got, []string{"Market/0", "Wilds/0"}) {
		t.Fatalf("Market closed: %v", got)
	}
}

// TestSearchFilters keeps what matches by name, folder name or tag, with
// its folders, and "#" searches tags only; Escape clears the search.
func TestSearchFilters(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withFolders(f)
	f.typeInto("search", "night")
	if got := f.workflowRows(); !slices.Equal(got, []string{"Market/0", "Guards/1", "Night watch/2"}) {
		t.Fatalf("night: %v", got)
	}
	if _, ok := f.state.Locate("row:Notes", f.ctx.Docs); ok {
		t.Fatal("Notes does not match")
	}
	f.press("Escape")
	f.typeInto("search", "guards")
	if got := f.workflowRows(); !slices.Equal(got, []string{"Market/0", "Guards/1", "Night watch/2"}) {
		t.Fatalf("a folder's name: %v", got)
	}
	f.press("Escape")
	f.typeInto("search", "#town")
	if got := f.workflowRows(); len(got) != 0 {
		t.Fatalf("no tag says town: %v", got)
	}
	f.press("Escape")
	f.press("Escape")
	if got := f.workflowRows(); len(got) != 5 {
		t.Fatalf("Escape clears the search: %v", got)
	}
}

// TestNewThingsGoInTheSelectedFolder makes a workflow and a folder inside
// the selected folder, and selects the new folder with its name focused.
func TestNewThingsGoInTheSelectedFolder(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withFolders(f)
	f.click("row:Guards", false)
	f.click("ui:new", false)
	f.click("ui:new-workflow", false)
	var created project.DocumentArgs
	if creates := f.requests.Of(project.CreateEdit); len(creates) != 1 || creates[0].Decode(&created) != nil ||
		created.Folder != "Market/Guards" {
		t.Fatalf("new workflow in Guards: %+v", created)
	}
	f.click("ui:new", false)
	f.click("ui:new-folder-workflow", false)
	var change project.FolderArgs
	folders := f.requests.Of(project.FolderEdit)
	if len(folders) != 1 || folders[0].Decode(&change) != nil || change.To != "Market/Guards/Folder 1" || change.From != "" {
		t.Fatalf("new folder inside Guards: %+v", change)
	}
	folders[0].Succeed(frametest.ID(0))
	docs := f.ctx.Docs.(*frametest.Docs)
	var index *project.Index
	docs.View(project.IndexID(), func(document client.Document) { index = document.Model.(*project.Index) })
	docs.Put(project.IndexID(), "Project", project.NewIndexWithFolders(index.Entries(),
		append(index.AllFolders(), project.Folder{Format: flow.FormatName, Path: "Market/Guards/Folder 1"})))
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if row, ok := f.state.SelectedRow(); !ok || row.ID != "folder:workflow:Market/Guards/Folder 1" || f.state.Focused() == nil {
		t.Fatalf("the new folder is selected with its name focused: %+v %v", row, ok)
	}
}

// TestMovingAndDragging moves a document with Move to… and by dragging it
// onto a folder, moves a folder by dragging it onto a section's heading,
// and refuses a drop on another type's section.
func TestMovingAndDragging(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withFolders(f)
	f.click("row:Clerk day", false)
	f.click("ui:move-to", false)
	f.click("menu:", false)
	var move project.MoveArgs
	if moves := f.requests.Of(project.MoveEdit); len(moves) != 1 || moves[0].Decode(&move) != nil ||
		move.ID != dayID || move.Folder != "" {
		t.Fatalf("Move to the top: %+v", move)
	}
	f.drag("row:Clerk day", "row:Wilds")
	if moves := f.requests.Of(project.MoveEdit); len(moves) != 2 || moves[1].Decode(&move) != nil || move.Folder != "Wilds" {
		t.Fatalf("dragged onto Wilds: %+v", move)
	}
	f.drag("row:Clerk day", "section:text")
	if moves := f.requests.Of(project.MoveEdit); len(moves) != 2 {
		t.Fatal("a workflow does not move into Text")
	}
	f.drag("row:Guards", "section:workflows")
	var change project.FolderArgs
	if folders := f.requests.Of(project.FolderEdit); len(folders) != 1 || folders[0].Decode(&change) != nil ||
		change.From != "Market/Guards" || change.To != "Guards" {
		t.Fatalf("Guards dragged to the top: %+v", change)
	}
}

// drag presses on one place, moves to another and releases there.
func (f *fixture) drag(from, to string) {
	f.t.Helper()
	start, end := f.at(from), f.at(to)
	press := ui.NewInput(start)
	press.Pressed, press.Down = true, true
	f.frame(press)
	held := ui.NewInput(end)
	held.Down = true
	f.frame(held)
	f.frame(held)
	release := ui.NewInput(end)
	release.Released = true
	f.frame(release)
}

// TestFolderInspector renames a folder with Save, refuses to delete one that
// holds anything, and deletes an empty one.
func TestFolderInspector(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withFolders(f)
	f.click("row:Guards", false)
	f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs)
	f.click("ui:name", false)
	f.press("Cmd+A")
	f.typeText("Watch")
	f.click("ui:save", false)
	var change project.FolderArgs
	if folders := f.requests.Of(project.FolderEdit); len(folders) != 1 || folders[0].Decode(&change) != nil ||
		change.From != "Market/Guards" || change.To != "Market/Watch" {
		t.Fatalf("rename: %+v", change)
	}
	f.click("ui:delete", false)
	if len(f.requests.Of(project.FolderEdit)) != 1 {
		t.Fatal("a folder holding a workflow is not deleted")
	}
	f.click("row:Wilds", false)
	f.click("ui:delete", false)
	var deletion project.FolderArgs
	if folders := f.requests.Of(project.FolderEdit); len(folders) != 2 || folders[1].Decode(&deletion) != nil ||
		deletion.From != "Wilds" || deletion.To != "" {
		t.Fatalf("delete Wilds: %+v", deletion)
	}
}

// TestTagsSaveAndSearch saves typed tags, and a tag's button searches for
// it.
func TestTagsSaveAndSearch(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withFolders(f)
	f.click("row:Night watch", false)
	f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs)
	f.click("ui:tag:guards", false)
	if got := f.workflowRows(); !slices.Equal(got, []string{"Market/0", "Guards/1", "Night watch/2"}) {
		t.Fatalf("#guards: %v", got)
	}
	f.click("ui:tags", false)
	f.press("Cmd+A")
	f.typeText("night, #patrol , night")
	f.click("ui:save", false)
	var retag project.TagsArgs
	if tags := f.requests.Of(project.TagsEdit); len(tags) != 1 || tags[0].Decode(&retag) != nil ||
		retag.ID != watchID || strings.Join(retag.Tags, ",") != "night,patrol" {
		t.Fatalf("tags: %+v", retag)
	}
}
