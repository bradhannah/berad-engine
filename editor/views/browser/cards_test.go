package browser_test

import (
	"slices"
	"testing"
)

// TestCardsShowPeopleAndPlaces shows NPCs and zones as cards by default,
// asking for a zone's ground floor to draw it small, and workflows as rows
// until Cards is picked.
func TestCardsShowPeopleAndPlaces(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("place:NPCs", false)
	if !f.has("pane=cards") {
		t.Fatal("NPCs show as cards")
	}
	f.click("row:Ada Clerk", false)
	if row, _ := f.state.SelectedRow(f.ctx.Docs); row.Label != "Ada Clerk" {
		t.Fatalf("a click selects a card: %+v", row)
	}
	f.click("place:Zones", false)
	if !f.has("pane=cards") {
		t.Fatal("zones show as cards")
	}
	f.click("row:Town", false)
	if !slices.Contains(f.requests.Subscribed, floorID) {
		t.Fatal("a zone's card asks for its ground floor")
	}
	f.click("place:Workflows", false)
	if !f.has("pane=rows") {
		t.Fatal("workflows show as rows")
	}
	f.click("ui:view-cards", false)
	if !f.has("pane=cards") {
		t.Fatal("Cards shows workflows as cards")
	}
}
