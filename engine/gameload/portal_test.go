package gameload_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/gameload"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
)

// TestDestinationFindsWhereAPortalLeads finds the zone, floor and cell of a
// portal's anchor; waits while the anchor's floor, or a zone that might list
// it, has not arrived; and says why a portal leads nowhere.
func TestDestinationFindsWhereAPortalLeads(t *testing.T) {
	t.Parallel()
	project := town(t)
	counter := tilefloor.AnchorRef{Floor: id(groundID), Anchor: id(counterID)}
	start, state, why := gameload.Destination(project, counter)
	if want := (game.Start{Zone: id(townID), Floor: id(groundID), Col: 3, Row: 3}); state != gameload.Held ||
		start != want || why != "" {
		t.Fatalf("the counter is in the town: %+v %v %q", start, state, why)
	}
	loaded, ready := gameload.Load(project, &start)
	if !ready || loaded.Project.Settings.Start != start || loaded.Project.Zone == nil {
		t.Fatalf("a game can be gathered for where the portal leads: %+v", loaded.Project.Settings.Start)
	}
	project.wait(true, groundID)
	if _, state, _ := gameload.Destination(project, counter); state != gameload.Waiting {
		t.Fatalf("the anchor's floor has not arrived: %v", state)
	}
	project.wait(false, groundID)
	project.wait(true, townID)
	if _, state, _ := gameload.Destination(project, counter); state != gameload.Waiting {
		t.Fatalf("the zone that lists the floor has not arrived: %v", state)
	}
	project.wait(false, townID)
	for name, nowhere := range map[string]struct {
		leads tilefloor.AnchorRef
		why   string
	}{
		"no floor":  {tilefloor.AnchorRef{Floor: id(99), Anchor: id(counterID)}, gameload.NoFloor},
		"no anchor": {tilefloor.AnchorRef{Floor: id(groundID), Anchor: id(99)}, gameload.NoAnchor},
	} {
		if _, state, why := gameload.Destination(project, nowhere.leads); state != gameload.Missing || why != nowhere.why {
			t.Fatalf("%s: %v %q", name, state, why)
		}
	}
	delete(project, id(townID))
	if _, state, why := gameload.Destination(project, counter); state != gameload.Missing || why != gameload.NoZone {
		t.Fatalf("a floor in no zone: %v %q", state, why)
	}
}
