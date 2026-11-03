package workflowview

import (
	"image"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/places"
	"github.com/bradhannah/berad-engine/editor/views/zonepicker"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// A workflow runs in one zone or in any zone, set in its inspector's Runs
// in box, which opens the zone picker; a strip along the top of the canvas
// names its kind, where it runs and who runs it.
const (
	runsInID = "runs-in"
	anyZone  = "Any zone"
	boxPad   = 8
	// recentZones is how many zones the picker remembers as chosen lately.
	recentZones = 8
)

// runsIn returns the zone the workflow runs in: zero for any zone, or for a
// zone since deleted.
func (s *State) runsIn(docs frame.Docs) ids.ULID {
	if s.workflow == nil {
		return ids.ULID{}
	}
	zoneID := s.workflow.RunsIn()
	if _, exists := s.names(docs).Entry(zoneID); !exists {
		return ids.ULID{}
	}

	return zoneID
}

// runsInName names where the workflow runs: its zone, or Any zone.
func (s *State) runsInName(docs frame.Docs) string {
	if zoneID := s.runsIn(docs); !zoneID.IsZero() {
		return s.names(docs).Name(zoneID, anyZone)
	}

	return anyZone
}

// stripLine names the workflow's kind, where it runs and who runs it, then
// who calls it and its ways out, as "NPC behaviour · Runs in Market ·
// Followed by Clerk, Customer".
func (s *State) stripLine(docs frame.Docs) string {
	if s.workflow == nil {
		return ""
	}
	purpose := s.workflow.Purpose()
	where := "any zone"
	if zoneID := s.runsIn(docs); !zoneID.IsZero() {
		where = s.names(docs).Name(zoneID, "its zone")
	}
	parts := []string{purpose.Label(), "Runs in " + where}
	label, how, nobody := protocol.RunsAsBehaviour, "Followed by ", "Followed by nobody yet"
	if purpose == flow.PurposeZoneLoop {
		label, how, nobody = protocol.RunsAsLoop, "Loop of ", "Loop of no zone yet"
	}
	var who []string
	for _, use := range s.runs.uses {
		if use.Label == label {
			who = append(who, use.Name)
		}
	}
	if len(who) > 0 {
		parts = append(parts, how+strings.Join(who, ", "))
	} else {
		parts = append(parts, nobody)
	}
	if callers := s.callersLine(); callers != "" {
		parts = append(parts, callers)
	}

	return strings.Join(parts, " · ")
}

// layoutRunsIn places the Runs in box at the top of the inspector and
// returns where what follows it starts.
func (s *State) layoutRunsIn(top int) int {
	inspector := frame.Inspector(s.size)
	left, width := inspector.Min.X+ui.Inset, inspector.Dx()-ui.Half*ui.Inset
	box := top + ui.LineHeight
	s.runsInBox = image.Rect(left, box, left+width, box+ui.ButtonH)

	return s.runsInBox.Max.Y + ui.Gap + ui.LineHeight + fieldSpace
}

// drawRunsIn draws the Runs in box, the zone's name in ink or Any zone
// grey, and what that means for the workflow's places.
func (s *State) drawRunsIn(screen *ebiten.Image, docs frame.Docs) {
	box := s.runsInBox
	if box.Empty() || s.workflow == nil {
		return
	}
	s.theme.Heading(screen, "Runs in", box.Min.X, box.Min.Y-ui.LineHeight)
	zoneID := s.runsIn(docs)
	ui.FillRound(screen, box, ui.Radius, ui.ColorBackground())
	ink, note := ui.ColorInk(), "Its places come from inputs and values, so NPCs in any zone can run it."
	if zoneID.IsZero() {
		ink = ui.ColorMuted()
		ui.StrokeRound(screen, box, ui.Radius, ui.Hairline, ui.ColorMuted())
	} else {
		ui.StrokeRound(screen, box, ui.Radius, ui.Border, ui.ColorInk())
		note = "Its places come from " + s.runsInName(docs) + "'s floors."
	}
	ui.DrawIcon(screen, ui.IconZone, image.Pt(box.Min.X+boxPad, box.Min.Y+(ui.ButtonH-ui.IconSize)/ui.Half))
	textTop := box.Min.Y + (ui.ButtonH-ui.TextSize)/ui.Half - 1
	choose := "Choose…"
	chooseLeft := box.Max.X - boxPad - s.theme.TextWidth(choose)
	nameLeft := box.Min.X + boxPad + ui.IconSize + boxPad
	name := s.theme.FitText(s.runsInName(docs), chooseLeft-nameLeft-boxPad)
	if zoneID.IsZero() {
		s.theme.DrawText(screen, name, nameLeft, textTop, ink)
	} else {
		s.theme.DrawStrong(screen, name, nameLeft, textTop, ink)
	}
	s.theme.DrawText(screen, choose, chooseLeft, textTop, ui.ColorMuted())
	s.theme.DrawText(screen, s.theme.FitText(note, box.Dx()), box.Min.X, box.Max.Y+ui.Gap, ui.ColorMuted())
}

// updateRunsIn opens the zone picker from the Runs in box, reporting
// whether it was clicked.
func (s *State) updateRunsIn(input *ui.Input, ctx *frame.Context) bool {
	if s.runsInBox.Empty() || !input.Pressed || !input.Cursor.In(s.runsInBox) {
		return false
	}
	input.ClearPointer()
	s.zones = zonepicker.New(zonepicker.Config{
		For: "for " + s.name(ctx.Docs) + " to run in", Now: s.runsIn(ctx.Docs), Recent: slices.Clone(s.recentZones),
		NoZone: true, NoneLabel: anyZone, GoesOn: false, Mine: s.placesByZone(ctx.Docs),
	}, s.theme)

	return true
}

// placesByZone counts the workflow's fixed places in each zone.
func (s *State) placesByZone(docs frame.Docs) map[ids.ULID]int {
	counts := map[ids.ULID]int{}
	if s.workflow == nil {
		return counts
	}
	for _, use := range flow.PlacesIn(s.workflow, s.loadedModule) {
		if zoneID, known := places.ZoneOf(docs, use.Place.Floor); known {
			counts[zoneID]++
		}
	}

	return counts
}

// loadedModule finds a workflow this one calls, when the window has it.
func (s *State) loadedModule(workflowID ids.ULID) (*flow.Workflow, bool) {
	called, loaded := s.modules[workflowID]

	return called.workflow, loaded && called.workflow != nil
}

// zoneOffer is what the zone picker lists, subscribing to what it needs
// when ctx is given.
func (s *State) zoneOffer(docs frame.Docs, ctx *frame.Context) zonepicker.Offer {
	subscribe := func(ids.ULID) {}
	if ctx != nil {
		subscribe = func(documentID ids.ULID) {
			if _, exists := s.names(docs).Entry(documentID); exists {
				s.subscribe(documentID, ctx)
			}
		}
	}

	return zonepicker.Offer{Zones: zonepicker.Gather(docs, subscribe), Folders: zonepicker.Folders(docs)}
}

// hasRunsInState answers "strip=<line>", the strip along the canvas, and
// "runs-in=<zone>", where the workflow runs, its zone's name or Any zone.
func (s *State) hasRunsInState(state string, docs frame.Docs) (bool, bool) {
	if line, asked := strings.CutPrefix(state, "strip="); asked {
		return s.stripLine(docs) == line, true
	}
	if where, asked := strings.CutPrefix(state, "runs-in="); asked {
		return s.workflow != nil && s.runsInName(docs) == where, true
	}

	return false, false
}

// updateZones runs the zone picker while it is open, and acts on what it
// chose.
func (s *State) updateZones(input *ui.Input, ctx *frame.Context) {
	window := image.Rectangle{Min: image.Pt(0, 0), Max: s.size}
	choice, chosen, open := s.zones.Update(input, ctx.Clipboard, window, s.zoneOffer(ctx.Docs, ctx))
	if !open {
		s.zones = nil
	}
	if !chosen {
		return
	}
	target := ids.ULID{}
	if !choice.None {
		target = choice.Zone
		s.recentZones = slices.DeleteFunc(s.recentZones, func(zoneID ids.ULID) bool { return zoneID == target })
		s.recentZones = append([]ids.ULID{target}, s.recentZones...)
		s.recentZones = s.recentZones[:min(len(s.recentZones), recentZones)]
	}
	s.changeRunsIn(ctx, target)
}

// changeRunsIn sets where the workflow runs.
func (s *State) changeRunsIn(ctx *frame.Context, zoneID ids.ULID) {
	if s.workflow == nil || zoneID == s.runsIn(ctx.Docs) {
		return
	}
	s.applyRunsIn(ctx, zoneID, s.gesture())
}

// applyRunsIn sets where the workflow runs as part of a gesture, saying so.
func (s *State) applyRunsIn(ctx *frame.Context, zoneID ids.ULID, gesture string) {
	name, where := s.name(ctx.Docs), "any zone"
	if !zoneID.IsZero() {
		where = s.names(ctx.Docs).Name(zoneID, "the zone")
	}
	s.command(ctx, workflowplugin.SetRunsInEdit, gesture, workflowplugin.SetRunsInArgs{Zone: zoneID},
		func(protocol.Result) { ctx.Frame.Status(name + " runs in " + where + ". Undo takes it back.") })
}

// drawZones draws the zone picker over the window while it is open.
func (s *State) drawZones(screen *ebiten.Image, docs frame.Docs) {
	if s.zones != nil {
		s.zones.Draw(screen, image.Rectangle{Min: image.Pt(0, 0), Max: s.size}, s.zoneOffer(docs, nil))
	}
}

// locateInZones finds a part of the open zone picker, or answers one of its
// states.
func (s *State) locateInZones(docs frame.Docs, kind, value string) (image.Point, bool) {
	window := image.Rectangle{Min: image.Pt(0, 0), Max: s.size}
	offer := s.zoneOffer(docs, nil)
	if kind == locateState {
		if value == "zone-picker" {
			return image.Pt(0, 0), true
		}
		found, _ := s.zones.HasState(window, offer, value)

		return image.Pt(0, 0), found
	}
	name := value
	if kind != "ui" {
		name = kind + ":" + value
	}

	return s.zones.Locate(window, offer, name)
}
