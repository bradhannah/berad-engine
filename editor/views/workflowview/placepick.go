package workflowview

import (
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/places"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// cancelledHere is why a pick ends when this window cancels it.
const cancelledHere = "cancelled in the workflow"

// litFrames is how long a place just picked or chosen glows in its box.
const litFrames = 90

// Owners name the control that started a pick, so only it reads Cancel pick:
// a Go to's anchor, a place local's start, or a Call's place input.
func anchorOwner(node ids.ULID, param string) string { return "anchor:" + node.String() + ":" + param }

func localOwner(local ids.ULID) string { return "local:" + local.String() }

func inputOwner(node, input ids.ULID) string { return "input:" + node.String() + ":" + input.String() }

// placeTarget is a place control: its owner, the place it holds now, what
// the place is for, as the status names it, and how a place is set, by
// picking on a map or choosing from the list of places.
type placeTarget struct {
	owner   string
	current floor.AnchorRef
	what    string
	set     func(*frame.Context, floor.AnchorRef)
	pick    func(*frame.Context)
}

// placeList is the open list of places and the control it chooses for.
type placeList struct {
	list   *places.List
	target placeTarget
}

// newPlaceTarget makes a place control's target; label says what a pick is
// for in the Map window.
func (s *State) newPlaceTarget(owner string, current floor.AnchorRef, what, label string,
	set func(*frame.Context, floor.AnchorRef),
) placeTarget {
	set = s.tying(set)
	target := placeTarget{owner: owner, current: current, what: what, set: set, pick: nil}
	target.pick = func(ctx *frame.Context) {
		was := s.placeWas(ctx.Docs, current)
		now := protocol.PickNow{Floor: ids.ULID{}, Item: "", Name: ""}
		if !current.IsZero() {
			now = protocol.PickNow{Floor: current.Floor, Item: current.Anchor.String(), Name: was}
		}
		s.picker.Start(ctx, owner, label, now, func(answer protocol.PickAnswer, ctx *frame.Context) {
			anchor, err := ids.Parse(answer.Item)
			if err != nil {
				ctx.Frame.Status("The map answered with something that is not an anchor")

				return
			}
			s.subscribe(answer.Floor, ctx)
			s.tied = false
			set(ctx, floor.AnchorRef{Floor: answer.Floor, Anchor: anchor})
			if !s.tied {
				s.said(ctx, owner, "Picked", answer.Name, what, was)
			}
		})
	}

	return target
}

// pickOrCancel starts a pick for a control, or cancels the one it started.
func (s *State) pickOrCancel(ctx *frame.Context, target placeTarget) {
	if s.picker.PickingFor(target.owner) {
		s.picker.Cancel(ctx, cancelledHere)

		return
	}
	target.pick(ctx)
}

// pickUpdate labels an owner's pick button, Cancel pick while its pick
// runs, and reports whether it was clicked.
func (s *State) pickUpdate(button *ui.Button, owner string, input *ui.Input) bool {
	button.Label = s.picker.Label(owner)

	return button.Update(input)
}

// openPlaces opens the list of places under a control's box, the places
// where the workflow's NPCs live first.
func (s *State) openPlaces(docs frame.Docs, box image.Rectangle, target placeTarget) {
	s.places = &placeList{list: places.Open(s.theme, box, target.current, s.home(docs)), target: target}
}

// placesOpen reports whether the list of places is open for an owner.
func (s *State) placesOpen(owner string) bool {
	return s.places != nil && s.places.target.owner == owner
}

// updatePlaces runs the open list of places: a place chosen is set, Pick on
// map picks instead.
func (s *State) updatePlaces(input *ui.Input, ctx *frame.Context) {
	all := places.All(ctx.Docs, func(document ids.ULID) { s.subscribe(document, ctx) })
	chosen, open := s.places.list.Update(input, ctx, s.screen(), all)
	target := s.places.target
	if !open {
		s.places = nil
	}
	if chosen == places.PickOnMap {
		target.pick(ctx)

		return
	}
	ref, ok := places.Parse(chosen)
	if !ok {
		return
	}
	name := ""
	for _, place := range all {
		if place.Ref == ref {
			name = place.Name
		}
	}
	was := s.placeWas(ctx.Docs, target.current)
	s.tied = false
	target.set(ctx, ref)
	if !s.tied {
		s.said(ctx, target.owner, "Chose", name, target.what, was)
	}
}

// drawPlaces draws the open list of places over the window.
func (s *State) drawPlaces(screen *ebiten.Image) { s.places.list.Draw(screen) }

// locatePlaces finds a part of the open list of places.
func (s *State) locatePlaces(docs frame.Docs, kind, value string) (image.Point, bool) {
	all := places.All(docs, func(ids.ULID) {})
	if kind == "ui" {
		return s.places.list.Locate(value, s.screen(), all)
	}

	return s.places.list.Locate(kind+":"+value, s.screen(), all)
}

// said says in the status what place was picked or chosen, for what, and
// what Undo puts back, and lights the control's box; was is the place's
// name before, or "" for none.
func (s *State) said(ctx *frame.Context, owner, verb, name, what, was string) {
	if name == "" {
		name = "a place"
	}
	message := verb + " “" + name + "” for " + what + "."
	if was != "" && was != name {
		message += " Undo puts “" + was + "” back."
	}
	ctx.Frame.Status(message)
	s.lit, s.litLeft = owner, litFrames
}

// isLit reports whether an owner's box glows, its place just set.
func (s *State) isLit(owner string) bool { return s.lit == owner && s.litLeft > 0 }

// fade counts down the glow of a place just set.
func (s *State) fade() {
	if s.litLeft > 0 {
		s.litLeft--
	}
}

// placeWas names an anchor before a pick changes it, or "" for none.
func (s *State) placeWas(docs frame.Docs, ref floor.AnchorRef) string {
	if ref.IsZero() {
		return ""
	}
	name, detail := s.anchorLabel(docs, ref)
	if detail == "" {
		return ""
	}

	return name
}

// SetShown is told whether this view's tab is in front.
func (s *State) SetShown(shown bool) { s.shown = shown }

// hasPlaceState answers "shown" and "hidden", whether the tab is in front,
// and "given:<input>=<place>", the place the selected Call gives an input.
func (s *State) hasPlaceState(state string, docs frame.Docs) (bool, bool) {
	switch state {
	case "shown":
		return s.shown, true
	case "hidden":
		return !s.shown, true
	}
	if name, isLocal := strings.CutPrefix(state, "out-of-reach:"); isLocal {
		for _, declared := range s.workflow.Locals().Values() {
			if _, out := s.localProblems[declared.ID]; out && declared.Name == name {
				return true, true
			}
		}

		return false, true
	}
	if shown, isList := strings.CutPrefix(state, "places-list="); isList {
		return s.places != nil && s.places.list.Shown(s.screen(), places.All(docs, func(ids.ULID) {})) == shown, true
	}
	rest, ok := strings.CutPrefix(state, "given:")
	if !ok {
		return false, false
	}
	name, want, _ := strings.Cut(rest, "=")
	node, selected := s.selectedNode()
	called, loaded := s.moduleOf(node)
	if !selected || !loaded {
		return false, true
	}
	for _, input := range called.workflow.Inputs() {
		if raw, given := node.Inputs()[input.ID]; given && input.Name == name {
			return s.givenLabel(docs, input, raw) == want, true
		}
	}

	return false, true
}
