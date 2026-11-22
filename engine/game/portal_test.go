package game_test

import (
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/play"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
)

// door is a portal on a cell that leads to the harbour's quay.
func door(col, row int, change string) tilefloor.Portal {
	return tilefloor.Portal{
		ID: id(90), Col: col, Row: row, To: tilefloor.AnchorRef{Floor: id(72), Anchor: id(73)}, Change: change,
	}
}

// townWithDoor is the town with a portal one cell south of the player start.
func townWithDoor(t *testing.T, change string) game.Project {
	t.Helper()
	project := town(t)
	if err := project.Floors[id(groundID)].InsertPortal(0, door(5, 6, change)); err != nil {
		t.Fatal(err)
	}

	return project
}

// still runs a game for some ticks with no key held.
func still(played *game.Game, count int) {
	walk(played, 0, count)
}

// TestAStepOntoAPortalStartsThePlayerLeaving steps onto a portal that cuts:
// the player is leaving by it at once, and can be taken through once the
// step has finished and they stand on its cell. Nothing fades, and the keys
// wait while the world carries on. Entering the zone it leads to ends the
// leaving, and the player walks there.
func TestAStepOntoAPortalStartsThePlayerLeaving(t *testing.T) {
	t.Parallel()
	played, err := game.New(townWithDoor(t, tilefloor.ChangeCut))
	if err != nil {
		t.Fatal(err)
	}
	if _, leaving := played.Leaving(); leaving || played.Dark() {
		t.Fatal("nobody is leaving before a step")
	}
	walk(played, play.South, 1)
	portal, leaving := played.Leaving()
	if !leaving || portal.ID != id(90) || played.Dark() || !played.Player().Moving() {
		t.Fatalf("the player is leaving by the portal, and still stepping onto it: %+v %v", portal, leaving)
	}
	walk(played, play.South, play.StepTicks-1)
	if played.Dark() {
		t.Fatal("the step has a tick to go, so nothing changes yet")
	}
	walk(played, play.South, 1)
	if !played.Dark() || played.Darkness() != 0 || played.Player().Moving() {
		t.Fatal("standing on the portal, the player can be taken through, with nothing faded")
	}
	at, clock := played.Player().Cell, played.Clock().Seconds()
	walk(played, play.South, 3*play.StepTicks)
	if played.Player().Cell != at || at != image.Pt(5, 6) {
		t.Fatalf("the keys wait while the player leaves: %v", played.Player().Cell)
	}
	if played.Clock().Seconds() == clock {
		t.Fatal("the world carries on while the player leaves")
	}
	if err := played.Enter(harbour(t)); err != nil {
		t.Fatal(err)
	}
	if _, leaving := played.Leaving(); leaving || played.Darkness() != 0 || played.Zone() != id(71) {
		t.Fatal("the player has arrived in the harbour, with nothing faded")
	}
	walk(played, play.East, 1)
	if played.Player().Cell != image.Pt(5, 4) {
		t.Fatalf("the player walks where the portal led: %v", played.Player().Cell)
	}
}

// TestAPortalThatFadesFadesOutAndIn steps onto a portal that fades: once
// the player stands on it the screen darkens a tick at a time, and only once
// it is black can the player be taken through. Where it leads the screen fades in as long, the keys
// waiting until it has.
func TestAPortalThatFadesFadesOutAndIn(t *testing.T) {
	t.Parallel()
	played, err := game.New(townWithDoor(t, tilefloor.ChangeFade))
	if err != nil {
		t.Fatal(err)
	}
	walk(played, play.South, 1)
	still(played, play.StepTicks)
	if _, leaving := played.Leaving(); !leaving || played.Dark() || played.Darkness() != 0 || played.Player().Moving() {
		t.Fatalf("the fade begins once the player stands on the portal: %v", played.Darkness())
	}
	still(played, game.FadeTicks/2)
	if played.Dark() || played.Darkness() != 0.5 {
		t.Fatalf("half way, the screen is half dark: %v", played.Darkness())
	}
	still(played, game.FadeTicks)
	if !played.Dark() || played.Darkness() != 1 {
		t.Fatalf("the screen is black and stays black until the player is taken: %v", played.Darkness())
	}
	if err := played.Enter(harbour(t)); err != nil {
		t.Fatal(err)
	}
	if played.Darkness() != 1 {
		t.Fatalf("the harbour begins black: %v", played.Darkness())
	}
	walk(played, play.East, game.FadeTicks/2)
	if played.Darkness() != 0.5 || played.Player().Cell != image.Pt(4, 4) {
		t.Fatalf("half way in, the keys still wait: %v at %v", played.Darkness(), played.Player().Cell)
	}
	walk(played, play.East, game.FadeTicks/2)
	if played.Darkness() != 0 {
		t.Fatalf("the screen has faded in: %v", played.Darkness())
	}
	walk(played, play.East, 1)
	if played.Player().Cell != image.Pt(5, 4) {
		t.Fatalf("then the player walks: %v", played.Player().Cell)
	}
}

// TestStayCallsALeavingOff leaves the player on the portal's cell, free to
// walk, with a fade that had begun fading back in. Standing there does not
// set the portal off again; stepping off and on does.
func TestStayCallsALeavingOff(t *testing.T) {
	t.Parallel()
	played, err := game.New(townWithDoor(t, tilefloor.ChangeFade))
	if err != nil {
		t.Fatal(err)
	}
	played.Stay()
	walk(played, play.South, 1)
	still(played, play.StepTicks+game.FadeTicks/2)
	played.Stay()
	if _, leaving := played.Leaving(); leaving || played.Darkness() != 0.5 {
		t.Fatalf("the leaving is off, and the screen as dark as it had got: %v", played.Darkness())
	}
	still(played, game.FadeTicks)
	if _, leaving := played.Leaving(); leaving || played.Darkness() != 0 || played.Player().Cell != image.Pt(5, 6) {
		t.Fatal("the screen fades back in, and standing on the portal does not set it off")
	}
	walk(played, play.North, play.StepTicks)
	if _, leaving := played.Leaving(); leaving || played.Player().Cell != image.Pt(5, 5) {
		t.Fatal("the player walks off the portal")
	}
	walk(played, play.South, play.StepTicks)
	if _, leaving := played.Leaving(); !leaving {
		t.Fatal("stepping onto the portal again sets it off again")
	}
}

// TestAPortalLeadsWithinItsZone takes the player to a cell of the zone they
// are in, as a portal to one of its own anchors does: nothing of the zone
// starts again, and the leaving is over.
func TestAPortalLeadsWithinItsZone(t *testing.T) {
	t.Parallel()
	played, err := game.New(townWithDoor(t, tilefloor.ChangeFade))
	if err != nil {
		t.Fatal(err)
	}
	walk(played, play.South, 1)
	still(played, play.StepTicks+game.FadeTicks)
	if !played.Dark() {
		t.Fatal("the screen is black")
	}
	if err := played.Place(id(upperID), image.Pt(1, 1)); err != nil {
		t.Fatal(err)
	}
	player := played.Player()
	if _, leaving := played.Leaving(); leaving || player.Floor != id(upperID) || player.Cell != image.Pt(1, 1) ||
		played.Darkness() != 1 || len(played.Runners()) != 1 {
		t.Fatalf("the player is upstairs, the screen about to fade in: %+v %v", player, played.Darkness())
	}
}

// TestArrivingOnAPortalDoesNotSetItOff puts the way back where the player
// arrives: arriving does nothing, and stepping off and on again leaves.
// Stairs on a portal's cell win: the player climbs.
func TestArrivingOnAPortalDoesNotSetItOff(t *testing.T) {
	t.Parallel()
	away := harbour(t)
	back := tilefloor.Portal{
		ID: id(91), Col: 4, Row: 4, To: tilefloor.AnchorRef{Floor: id(groundID), Anchor: id(92)},
		Change: tilefloor.ChangeCut,
	}
	if err := away.Floors[id(72)].InsertPortal(0, back); err != nil {
		t.Fatal(err)
	}
	project := townWithDoor(t, tilefloor.ChangeCut)
	stairs := door(2, 2, tilefloor.ChangeCut)
	stairs.ID = id(93)
	if err := project.Floors[id(groundID)].InsertPortal(1, stairs); err != nil {
		t.Fatal(err)
	}
	project.Settings.Start = game.Start{Zone: id(zoneID), Floor: id(groundID), Col: 2, Row: 3}
	played, err := game.New(project)
	if err != nil {
		t.Fatal(err)
	}
	steps := walk(played, play.North, 1)
	if _, leaving := played.Leaving(); leaving || steps[0].Outcome != play.Climbed || played.Player().Floor != id(upperID) {
		t.Fatalf("stairs on a portal's cell take the player up: %+v", steps[0])
	}
	if err := played.Enter(away); err != nil {
		t.Fatal(err)
	}
	still(played, 2*play.StepTicks)
	if _, leaving := played.Leaving(); leaving {
		t.Fatal("arriving on a portal does not set it off")
	}
	walk(played, play.East, play.StepTicks)
	walk(played, play.West, play.StepTicks)
	if portal, leaving := played.Leaving(); !leaving || portal.ID != id(91) {
		t.Fatal("stepping back onto it does")
	}
}

// TestAGameWithPortalsRepeats plays the same keys twice through a fade, and
// finds the player and the darkness the same on every tick.
func TestAGameWithPortalsRepeats(t *testing.T) {
	t.Parallel()
	type moment struct {
		cell     image.Point
		darkness float64
		zone     ids.ULID
	}
	run := func() []moment {
		played, err := game.New(townWithDoor(t, tilefloor.ChangeFade))
		if err != nil {
			t.Fatal(err)
		}
		var seen []moment
		for tick := range 5 * game.FadeTicks {
			if played.Dark() {
				if err := played.Enter(harbour(t)); err != nil {
					t.Fatal(err)
				}
			}
			input := play.Input{Held: play.South, Pressed: 0}
			if tick == 0 {
				input.Pressed = play.South
			}
			played.Step(input)
			seen = append(seen, moment{cell: played.Player().Cell, darkness: played.Darkness(), zone: played.Zone()})
		}

		return seen
	}
	first, second := run(), run()
	for tick := range first {
		if first[tick] != second[tick] {
			t.Fatalf("tick %d differs: %+v and %+v", tick, first[tick], second[tick])
		}
	}
	if last := first[len(first)-1]; last.zone != id(71) || last.darkness != 0 || last.cell == image.Pt(4, 4) {
		t.Fatalf("the run ends walking in the harbour: %+v", last)
	}
}
