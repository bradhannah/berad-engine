package browser_test

import (
	"encoding/json"
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

var tideID = frametest.ID(83)

// withSayings gives the shop's project a world constant, tide_minutes, and
// Ada Clerk her own greeting, with markup, that mentions the tide.
func (f *fixture) withSayings() {
	f.t.Helper()
	f.withShop()
	f.docs().View(worldstate.ID(), func(document client.Document) {
		tide := worldstate.NewConstant(tideID, "tide_minutes", worldstate.ScopeWorld, worldstate.TypeInteger)
		tide.Default = json.RawMessage(`30`)
		if err := document.Model.(*stateplugin.Document).State.Values.Insert(tide); err != nil {
			f.t.Fatal(err)
		}
	})
	f.docs().Touch(worldstate.ID())
	f.docs().View(clerkID, func(document client.Document) {
		document.Model.(*npcplugin.Document).NPC.Owned.SetConstant(greetingID,
			json.RawMessage(`"[b]Come in[/b], before the tide turns and we close early today!"`))
	})
	f.docs().Touch(clerkID)
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

// searchFor types a search in place of the one there.
func (f *fixture) searchFor(text string) {
	f.t.Helper()
	f.click("ui:search", false)
	f.press("Cmd+A")
	f.typeText(text)
}

// foundRow is the row Matches shows for a found item.
func (f *fixture) foundRow(prefix string) (ui.Row, bool) {
	for _, row := range f.state.Rows(f.ctx.Docs) {
		if strings.HasPrefix(row.ID, prefix) {
			return row, true
		}
	}

	return ui.Row{}, false
}

// TestSearchFindsConstants searches for "tide": tide_minutes by its path,
// with its value, and the shop's greeting by Ada's own text, markup left
// out; a search of tags finds neither.
func TestSearchFindsConstants(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withSayings()
	f.searchFor("tide")
	labels := f.matchLabels()
	byPath, found := f.foundRow("found-constant:constant:" + tideID.String())
	if !found || byPath.Detail != "30" || !slices.Contains(labels, byPath.Label) {
		t.Fatalf("tide finds tide_minutes by its path, with its value: %+v %v", byPath, labels)
	}
	byText, found := f.foundRow("found-constant:constant:" + greetingID.String() + ":" + clerkID.String())
	if !found || !strings.HasPrefix(byText.Detail, "Ada Clerk: “") || !strings.Contains(byText.Detail, "tide") ||
		strings.Contains(byText.Detail, "[b]") {
		t.Fatalf("tide finds Ada's own greeting, without its tags: %+v", byText)
	}
	if _, found := f.foundRow("found-constant:constant:" + serveID.String()); found {
		t.Fatal("serve_minutes says nothing of the tide")
	}
	f.searchFor("welcome")
	if _, found := f.foundRow("found-constant:constant:" + greetingID.String()); !found {
		t.Fatal("welcome finds the greeting's default")
	}
	f.searchFor("#tide")
	if _, found := f.foundRow("found-constant:"); found {
		t.Fatal("a search of tags finds no constant")
	}
}

// TestOpeningAFoundConstant selects Ada's greeting in Matches: the
// inspector shows her whole text, and Open opens the Constants window at
// her own value.
func TestOpeningAFoundConstant(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withSayings()
	f.searchFor("tide")
	item := "constant:" + greetingID.String() + ":" + clerkID.String()
	row, _ := f.foundRow("found-constant:" + item)
	f.click("row:"+row.Label, false)
	f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs)
	f.click("ui:open", false)
	opened := f.requests.Opened
	if len(opened) == 0 || opened[len(opened)-1].View != "world-constants" || opened[len(opened)-1].Item != item {
		t.Fatalf("Open opens the Constants window at Ada's greeting: %+v", opened)
	}
}
