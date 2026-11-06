package browser_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/bradhannah/berad-engine/editor/protocol"
)

// TestRowsShowColumnsAndSort shows a folder of workflows: its folder first,
// then its workflow with its details in columns. A folder opens in place,
// and a heading sorts, which the folder keeps.
func TestRowsShowColumnsAndSort(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withFolders(f)
	f.click("place:Market", false)
	if got := f.labels(); !slices.Equal(got, []string{"Guards", "Clerk day"}) {
		t.Fatalf("Market's folder, then its workflow: %v", got)
	}
	if day := f.rowNamed("Clerk day"); len(day.Cells) < 3 || day.Cells[0] != "NPC behaviour" {
		t.Fatalf("Clerk day's details in columns: %+v", day.Cells)
	}
	if _, found := f.state.Locate("sort:Runs as", f.ctx.Docs); !found {
		t.Fatal("the columns have headings")
	}
	f.click("toggle:Guards", false)
	if got := f.rowsIn("folder:workflow:Market"); !slices.Equal(got, []string{"Guards/0", "Night watch/1", "Clerk day/0"}) {
		t.Fatalf("Guards opens in place: %v", got)
	}
	f.click("sort:Runs as", false)
	var kept protocol.HomeKept
	published := f.requests.Published
	if len(published) == 0 || json.Unmarshal(published[len(published)-1].Value, &kept) != nil ||
		kept.Folders["folder:workflow:Market"].Sort != "Runs as" {
		t.Fatalf("the folder keeps its sort: %+v", kept)
	}
	f.click("sort:Name", false)
	f.click("sort:Name", false)
	if got := f.labels(); !slices.Equal(got, []string{"Guards", "Night watch", "Clerk day"}) {
		t.Fatalf("folders stay first, whatever the sort: %v", got)
	}
}
