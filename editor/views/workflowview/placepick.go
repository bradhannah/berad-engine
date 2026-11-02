package workflowview

import (
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// cancelledHere is why a pick ends when this window cancels it.
const cancelledHere = "cancelled in the workflow"

// Owners name the control that started a pick, so only it reads Cancel pick:
// a Go to's anchor, a place local's start, or a Call's place input.
func anchorOwner(node ids.ULID, param string) string { return "anchor:" + node.String() + ":" + param }

func localOwner(local ids.ULID) string { return "local:" + local.String() }

func inputOwner(node, input ids.ULID) string { return "input:" + node.String() + ":" + input.String() }

// pickUpdate labels an owner's pick button, Cancel pick while its pick
// runs, and reports whether it was clicked.
func (s *State) pickUpdate(button *ui.Button, owner string, input *ui.Input) bool {
	button.Label = s.picker.Label(owner)

	return button.Update(input)
}

// picked reads a pick's answer as an anchor and says in the status what was
// picked, for what, and what Undo puts back; was is the place's name before,
// or "" for none.
func (s *State) picked(ctx *frame.Context, answer protocol.PickAnswer, what, was string) (floor.AnchorRef, bool) {
	anchor, err := ids.Parse(answer.Item)
	if err != nil {
		ctx.Frame.Status("The map answered with something that is not an anchor")

		return floor.AnchorRef{Floor: ids.ULID{}, Anchor: ids.ULID{}}, false
	}
	s.subscribe(answer.Floor, ctx)
	name := answer.Name
	if name == "" {
		name = "an anchor"
	}
	message := "Picked “" + name + "” for " + what + "."
	if was != "" && was != name {
		message += " Undo puts “" + was + "” back."
	}
	ctx.Frame.Status(message)

	return floor.AnchorRef{Floor: answer.Floor, Anchor: anchor}, true
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
