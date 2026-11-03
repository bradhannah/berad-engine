package workflowview

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/zonepicker"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// A zone, a workflow or a sprite an operand holds is chosen from a box: the
// zone picker for a zone, a list for a workflow or a sprite.
const (
	refUseID   = "use"
	refNoneYet = "Choose…"
)

// formatOf is the document format a value type refers to.
func formatOf(kind worldstate.Type) string {
	switch kind {
	case worldstate.TypeWorkflow:
		return flow.FormatName
	case worldstate.TypeSprite:
		return art.SpriteFormatName
	case worldstate.TypeZone:
		return "zone"
	case worldstate.TypeInteger, worldstate.TypeBool, worldstate.TypeChoice, worldstate.TypeText, worldstate.TypeNPC,
		worldstate.TypePlace, worldstate.TypeGroup, worldstate.TypeTemplate:
	}

	return ""
}

// refHeld is the document an operand of a zone, workflow or sprite holds,
// zero for none.
func refHeld(node flow.Node, param string, kind worldstate.Type) ids.ULID {
	saved, err := node.Operand(param)
	if err != nil {
		return ids.ULID{}
	}
	declared := worldstate.New(ids.ULID{}, "held", worldstate.ScopeLocal, kind)
	value, err := declared.Parse(saved)
	if err != nil {
		return ids.ULID{}
	}

	return value.Ref
}

// layoutRef places the box of an operand that holds a zone, a workflow or a
// sprite, and returns where the next control goes.
func layoutRef(made *control, left, width, top int) int {
	made.cueBox = image.Rect(left, top, left+width, top+ui.ButtonH)

	return top + ui.ButtonH + fieldSpace
}

// drawRef draws the box of an operand that holds a zone, a workflow or a
// sprite: the one held, by name, or Choose….
func (s *State) drawRef(screen *ebiten.Image, docs frame.Docs, made *control, node flow.Node) {
	held := refHeld(node, made.param.Name, made.operandType)
	label, ink := refNoneYet, ui.ColorMuted()
	if !held.IsZero() {
		label, ink = s.names(docs).Name(held, "a deleted "+string(made.operandType)), ui.ColorInk()
	}
	ui.Box(screen, made.cueBox, ui.ColorBackground())
	if made.operandType == worldstate.TypeZone {
		corner := image.Pt(made.cueBox.Min.X+boxPad, made.cueBox.Min.Y+(ui.ButtonH-ui.IconSize)/ui.Half)
		ui.DrawIcon(screen, ui.IconZone, corner)
		s.theme.DrawText(screen, s.theme.FitText(label, made.cueBox.Dx()-ui.Half*boxPad-ui.IconSize-boxPad),
			made.cueBox.Min.X+boxPad+ui.IconSize+boxPad, made.cueBox.Min.Y+(ui.ButtonH-ui.TextSize)/ui.Half-1, ink)

		return
	}
	s.theme.DrawTextIn(screen, label, made.cueBox, ink)
}

// updateRef opens the chooser of an operand that holds a zone, a workflow or
// a sprite when its box is clicked: the zone picker, or a list.
func (s *State) updateRef(input *ui.Input, ctx *frame.Context, made *control, node flow.Node) {
	if !input.Pressed || !input.Cursor.In(made.cueBox) {
		return
	}
	input.ClearPointer()
	param, kind, nodeID := made.param.Name, made.operandType, node.ID
	held := refHeld(node, param, kind)
	save := func(ctx *frame.Context, chosen ids.ULID) {
		if current, ok := s.currentNode(ctx.Docs, nodeID); ok {
			value := worldstate.New(ids.ULID{}, "held", worldstate.ScopeLocal, kind).Empty()
			value.Ref = chosen
			s.saveParam(ctx, current, param, value.Encode())
		}
	}
	if kind == worldstate.TypeZone {
		s.zones = zonepicker.New(zonepicker.Config{
			For: "for this " + checkOrSet(node), Now: held, Recent: s.recentZones, NoZone: false, NoneLabel: "",
			GoesOn: false, Mine: nil,
		}, s.theme)
		s.zonesChose = func(ctx *frame.Context, choice zonepicker.Choice) {
			if !choice.None {
				s.rememberZone(choice.Zone)
				save(ctx, choice.Zone)
			}
		}

		return
	}
	s.chooseDocument(ctx, kind, held, save)
}

// checkOrSet names what a value node does, for the zone picker's line.
func checkOrSet(node flow.Node) string {
	if node.Kind == flow.KindCheckValue {
		return "check"
	}

	return "Set value"
}

// chooseDocument asks which workflow or sprite an operand holds, from a
// list of the project's.
func (s *State) chooseDocument(ctx *frame.Context, kind worldstate.Type, held ids.ULID,
	save func(*frame.Context, ids.ULID),
) {
	selected := ""
	if !held.IsZero() {
		selected = held.String()
	}
	list := ui.NewList("documents", selected)
	icon := ui.IconWorkflow
	if kind == worldstate.TypeSprite {
		icon = ui.IconSprite
	}
	for _, entry := range s.names(ctx.Docs).OfFormat(formatOf(kind)) {
		row := ui.NewRow(entry.ID.String(), entry.Name)
		row.Icon = icon
		list.Rows = append(list.Rows, row)
	}
	s.dialog = ui.NewDialog("Which "+string(kind)+"?", nil, ui.DialogButton(refUseID, "Use"),
		ui.DialogButton(ui.DialogCancel, "Cancel"))
	s.dialog.List = list
	s.dialogThen = func(ctx *frame.Context, _ string) {
		if chosen, err := ids.Parse(list.Selected); err == nil {
			save(ctx, chosen)
		}
	}
}

// rememberZone puts a zone first among those chosen lately.
func (s *State) rememberZone(zoneID ids.ULID) {
	kept := []ids.ULID{zoneID}
	for _, each := range s.recentZones {
		if each != zoneID && len(kept) < recentZones {
			kept = append(kept, each)
		}
	}
	s.recentZones = kept
}
