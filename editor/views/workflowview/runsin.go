package workflowview

import (
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/places"
	"github.com/bradhannah/berad-engine/editor/views/zonepicker"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
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
		NoZone: true, NoneLabel: anyZone, GoesOn: len(s.runs.uses) > 0 || len(s.placesByZone(ctx.Docs)) > 0,
		Mine: s.placesByZone(ctx.Docs),
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

// hasRunsInState answers "strip=<line>", the strip along the canvas;
// "runs-in=<zone>", where the workflow runs, its zone's name or Any zone;
// and "call-menu=<items>", the open list of workflows a Call can call, as
// "Errand / Roam / (Harbour errand · runs in Harbour)".
func (s *State) hasRunsInState(state string, docs frame.Docs) (bool, bool) {
	if line, asked := strings.CutPrefix(state, "strip="); asked {
		return s.stripLine(docs) == line, true
	}
	if where, asked := strings.CutPrefix(state, "runs-in="); asked {
		return s.workflow != nil && s.runsInName(docs) == where, true
	}
	if listed, asked := strings.CutPrefix(state, "call-menu="); asked {
		var shown []string
		for _, item := range s.contextMenu.Items {
			if item.Disabled {
				shown = append(shown, "("+item.Label+")")
			} else {
				shown = append(shown, item.Label)
			}
		}

		return s.contextMenu.IsOpen() && strings.Join(shown, " / ") == listed, true
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

// changeRunsIn sets where the workflow runs, asking first when that breaks
// anything.
func (s *State) changeRunsIn(ctx *frame.Context, zoneID ids.ULID) {
	if s.workflow == nil || zoneID == s.runsIn(ctx.Docs) {
		return
	}
	if change := s.changeFor(ctx.Docs, zoneID); !change.empty() {
		s.askRunsIn(ctx, change, func(ctx *frame.Context, gesture string) { s.applyRunsIn(ctx, zoneID, gesture) })

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

// runsInChangeID is the button of the dialog asking before changing where a
// workflow runs, and dialogWidth how wide its lines wrap.
const (
	runsInChangeID = "runs-in-change"
	dialogLineW    = 380
)

// runsInChange is what changing where the workflow runs would break: the
// NPCs following it, or the zones looping it, that live elsewhere, and the
// fixed places it names elsewhere.
type runsInChange struct {
	to      ids.ULID
	runners []string
	places  []flow.PlaceUse
}

// empty reports whether the change breaks nothing.
func (c runsInChange) empty() bool { return len(c.runners) == 0 && len(c.places) == 0 }

// changeFor works out what running in a zone, or in any zone, would break.
// Running in any zone breaks no runner, but every fixed place.
func (s *State) changeFor(docs frame.Docs, zoneID ids.ULID) runsInChange {
	change := runsInChange{to: zoneID, runners: nil, places: nil}
	if s.workflow == nil {
		return change
	}
	if !zoneID.IsZero() {
		for _, each := range s.runnersOf(docs) {
			if each.home && !each.zone.IsZero() && each.zone != zoneID {
				change.runners = append(change.runners, each.name)
			}
		}
	}
	for _, use := range flow.PlacesIn(s.workflow, s.loadedModule) {
		if placeZone, known := places.ZoneOf(docs, use.Place.Floor); known && (zoneID.IsZero() || placeZone != zoneID) {
			change.places = append(change.places, use)
		}
	}

	return change
}

// askRunsIn asks before a change that breaks something, listing what, and
// carries it out as one gesture when it is confirmed.
func (s *State) askRunsIn(ctx *frame.Context, change runsInChange, then func(*frame.Context, string)) {
	index := s.names(ctx.Docs)
	going, from := anyZone, s.runsInName(ctx.Docs)
	if !change.to.IsZero() {
		going = index.Name(change.to, "the zone")
	}
	lines := []string{from + " → " + going}
	if len(change.runners) > 0 {
		who, several := andList(change.runners), len(change.runners) > 1
		line := who + " lives elsewhere, and is flagged in its details until it follows a behaviour that runs " +
			"where it lives, or in any zone."
		if several {
			line = who + " live elsewhere. Each is flagged in its details until it follows a behaviour that runs " +
				"where it lives, or in any zone."
		}
		if s.workflow.Purpose() == flow.PurposeZoneLoop {
			line = who + " runs it as its zone loop, and is flagged in its details until its loop runs there, or " +
				"in any zone."
			if several {
				line = who + " run it as their zone loop. Each is flagged in its details until its loop runs there, " +
					"or in any zone."
			}
		}
		lines = append(lines, line)
	}
	if len(change.places) > 0 {
		line := "Each place listed is flagged on its node until it is one in " + going + "."
		if change.to.IsZero() {
			line = "Each place listed is flagged on its node until it comes from an input or another value."
		}
		lines = append(lines, line)
	}
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		wrapped = append(wrapped, s.theme.Wrap(line, dialogLineW)...)
	}
	title, button := "Run "+s.name(ctx.Docs)+" in "+going+"?", "Run in "+going
	if change.to.IsZero() {
		title, button = "Run "+s.name(ctx.Docs)+" in any zone?", "Run in any zone"
	}
	s.dialog = ui.NewDialog(title, wrapped, ui.DialogButton(runsInChangeID, button),
		ui.DialogButton(ui.DialogCancel, "Cancel"))
	if len(change.places) > 0 {
		s.dialog.List = s.placesLeft(ctx.Docs, change.places)
	}
	s.dialogThen = then
}

// placesLeft lists the places a change leaves behind, each with what names
// it and its zone.
func (s *State) placesLeft(docs frame.Docs, uses []flow.PlaceUse) *ui.List {
	index := s.names(docs)
	list := ui.NewList("places-left", "")
	for number, use := range uses {
		row := ui.NewRow("left-"+strconv.Itoa(number), anchorName(docs, use.Place))
		row.Icon = ui.IconPlace
		what := "Go to"
		switch {
		case use.Local != "":
			what = "local " + use.Local
		case !use.Called.IsZero():
			what = "Call " + index.Name(use.Called, "a workflow") + " · " + use.Input
		}
		zoneID, _ := places.ZoneOf(docs, use.Place.Floor)
		row.Detail = what + " · " + index.Name(zoneID, "")
		list.Rows = append(list.Rows, row)
	}

	return list
}

// updateDialog runs the dialog asking before a change, carrying the change
// out when it is confirmed.
func (s *State) updateDialog(input *ui.Input, ctx *frame.Context) {
	choice := s.dialog.Update(input, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
	if choice == "" {
		return
	}
	then := s.dialogThen
	s.dialog, s.dialogThen = nil, nil
	if choice == runsInChangeID && then != nil {
		then(ctx, s.gesture())
	}
}

// locateInDialog finds a dialog button or list row, or answers
// "dialog-line=<text>", a line of the dialog holding the text, and
// "dialog-row=<name>", a row of its list.
func (s *State) locateInDialog(kind, value string) (image.Point, bool) {
	if kind == locateState {
		line, isLine := strings.CutPrefix(value, "dialog-line=")
		row, isRow := strings.CutPrefix(value, "dialog-row=")
		found := isLine && strings.Contains(strings.Join(s.dialog.Lines, " "), line)
		if isRow && s.dialog.List != nil {
			found = slices.ContainsFunc(s.dialog.List.Rows, func(listed ui.Row) bool { return listed.Label == row })
		}

		return image.Pt(0, 0), found || value == "dialog"
	}

	return s.dialog.Locate(value, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
}

// tying wraps a place control's set, so a fixed place chosen in a workflow
// that runs in any zone ties the workflow to the place's zone in the same
// gesture, asking first when that breaks anything. It marks the window
// tied, so the place's own status gives way to the tie's.
func (s *State) tying(set func(*frame.Context, floor.AnchorRef)) func(*frame.Context, floor.AnchorRef) {
	return func(ctx *frame.Context, ref floor.AnchorRef) {
		zoneID, known := places.ZoneOf(ctx.Docs, ref.Floor)
		if s.workflow == nil || !s.runsIn(ctx.Docs).IsZero() || !known {
			set(ctx, ref)

			return
		}
		s.tied = true
		both := func(ctx *frame.Context, gesture string) {
			s.joining = gesture
			set(ctx, ref)
			s.joining = ""
			name, where := s.name(ctx.Docs), s.names(ctx.Docs).Name(zoneID, "its zone")
			s.command(ctx, workflowplugin.SetRunsInEdit, gesture, workflowplugin.SetRunsInArgs{Zone: zoneID},
				func(protocol.Result) {
					ctx.Frame.Status(name + " runs in " + where + " now, the zone of the place chosen. Undo takes both " +
						"back.")
				})
		}
		if change := s.changeFor(ctx.Docs, zoneID); !change.empty() {
			s.askRunsIn(ctx, change, both)

			return
		}
		both(ctx, s.gesture())
	}
}
