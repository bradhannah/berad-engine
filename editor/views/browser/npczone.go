package browser

import (
	"fmt"
	"image"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/mapview"
	"github.com/bradhannah/berad-engine/editor/views/zonepicker"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// The Zone box in an NPC's details, and the zone picker it opens.
const (
	zoneBoxID   = "npc-zone"
	showOnMapID = "show-on-map"
	noZoneYet   = "No zone yet"
	// recentZones is how many zones the picker remembers as chosen lately.
	recentZones = 8
	showMapW    = 110
	boxPad      = 8
)

// placed is where an NPC is placed: the floor, its name, the placement and
// its cell.
type placed struct {
	floor, placement ids.ULID
	name             string
	cell             image.Point
}

// zoneOfNPC returns the zone an NPC lives in, zero for none or for a zone
// since deleted.
func (s *State) zoneOfNPC(docs frame.Docs, npcID ids.ULID) ids.ULID {
	found, _ := s.npcOf(docs, npcID)
	if _, exists := s.index(docs).Entry(found.Zone); !exists {
		return ids.ULID{}
	}

	return found.Zone
}

// placementOf finds where an NPC is placed on its zone's floors, as far as
// the window has them.
func (s *State) placementOf(docs frame.Docs, npcID ids.ULID) (placed, bool) {
	zoneID := s.zoneOfNPC(docs, npcID)
	for _, floorID := range s.floorsOf(docs, zoneID) {
		var found placed
		onFloor := false
		docs.View(floorID, func(document client.Document) {
			floorDoc, isFloor := document.Model.(*tilefloor.Document)
			if !isFloor {
				return
			}
			for _, placement := range floorDoc.Floor.Placements() {
				if placement.NPC == npcID {
					found, onFloor = placed{floor: floorID, placement: placement.ID, name: "", cell: image.Pt(placement.Col,
						placement.Row)}, true
				}
			}
		})
		if onFloor {
			found.name = s.index(docs).Name(floorID, "a floor")

			return found, true
		}
	}

	return placed{floor: ids.ULID{}, placement: ids.ULID{}, name: "", cell: image.Pt(0, 0)}, false
}

// subscribeHome keeps the selected NPC's zone and its floors loaded, so the
// panel can say where it is placed.
func (s *State) subscribeHome(ctx *frame.Context, npcID ids.ULID) {
	zoneID := s.zoneOfNPC(ctx.Docs, npcID)
	if zoneID.IsZero() {
		return
	}
	if _, exists := s.index(ctx.Docs).Entry(zoneID); !exists {
		return
	}
	s.subscribe(zoneID, ctx)
	for _, floorID := range s.floorsOf(ctx.Docs, zoneID) {
		s.subscribe(floorID, ctx)
	}
}

// layoutZone places the Zone box at top, and Placed on with Show on map
// under it, returning where the next heading goes.
func (s *State) layoutZone(docs frame.Docs, npcID ids.ULID, left, width, top int) int {
	panel := s.npc
	rowY := top + ui.LineHeight
	panel.zone = image.Rect(left, rowY, left+width, rowY+ui.ButtonH)
	rowY += ui.ButtonH + ui.Gap*ui.Half
	panel.placedTop = rowY
	panel.showOnMap.Rect = ui.Zero()
	if _, isPlaced := s.placementOf(docs, npcID); isPlaced {
		panel.showOnMap.Rect = image.Rect(left+width-showMapW, rowY, left+width, rowY+ui.ButtonH)
	}

	return rowY + ui.ButtonH + panelGap
}

// drawZone draws the Zone box, the zone's name in ink or No zone yet grey
// and dashed, and where the NPC is placed.
func (s *State) drawZone(screen *ebiten.Image, docs frame.Docs, npcID ids.ULID, left int) {
	panel := s.npc
	s.theme.Heading(screen, "ZONE", left, panel.zone.Min.Y-ui.LineHeight)
	zoneID := s.zoneOfNPC(docs, npcID)
	name, known := "", false
	if entry, exists := s.index(docs).Entry(zoneID); exists && !zoneID.IsZero() {
		name, known = entry.Name, true
	}
	ui.FillRound(screen, panel.zone, ui.Radius, ui.ColorBackground())
	ink := ui.ColorInk()
	if known {
		ui.StrokeRound(screen, panel.zone, ui.Radius, ui.Border, ui.ColorInk())
	} else {
		ink = ui.ColorMuted()
		ui.StrokeRound(screen, panel.zone, ui.Radius, ui.Hairline, ui.ColorMuted())
		name = noZoneYet
	}
	ui.DrawIcon(screen, ui.IconZone, image.Pt(panel.zone.Min.X+boxPad, panel.zone.Min.Y+(ui.ButtonH-ui.IconSize)/ui.Half))
	textTop := panel.zone.Min.Y + (ui.ButtonH-ui.TextSize)/ui.Half - 1
	choose := "Choose…"
	chooseLeft := panel.zone.Max.X - boxPad - s.theme.TextWidth(choose)
	nameLeft := panel.zone.Min.X + boxPad + ui.IconSize + boxPad
	if known {
		s.theme.DrawStrong(screen, s.theme.FitText(name, chooseLeft-nameLeft-boxPad), nameLeft, textTop, ink)
	} else {
		s.theme.DrawText(screen, name, nameLeft, textTop, ink)
	}
	s.theme.DrawText(screen, choose, chooseLeft, textTop, ui.ColorMuted())
	line, lineInk := "Placed nowhere yet", ui.ColorMuted()
	if where, isPlaced := s.placementOf(docs, npcID); isPlaced {
		line, lineInk = fmt.Sprintf("Placed on %s, at %d, %d", where.name, where.cell.X, where.cell.Y), ui.ColorInk()
	}
	room := panel.zone.Dx()
	if !panel.showOnMap.Rect.Empty() {
		room = panel.showOnMap.Rect.Min.X - left - ui.Gap
	}
	s.theme.DrawText(screen, s.theme.FitText(line, room), left, panel.placedTop+(ui.ButtonH-ui.TextSize)/ui.Half-1,
		lineInk)
	panel.showOnMap.Draw(screen, s.theme)
}

// updateZone opens the zone picker from the Zone box, and shows the NPC on
// its map from Show on map. It reports whether either was clicked.
func (s *State) updateZone(input *ui.Input, ctx *frame.Context, npcID ids.ULID) bool {
	switch {
	case input.Pressed && input.Cursor.In(s.npc.zone):
		input.ClearPointer()
		s.openZonePicker(ctx, npcID)

		return true
	case s.npc.showOnMap.Update(input):
		if where, isPlaced := s.placementOf(ctx.Docs, npcID); isPlaced {
			item := mapview.PlacementItem + where.floor.String() + ":" + where.placement.String()
			ctx.Requests.OpenItem(mapView, s.zoneOfNPC(ctx.Docs, npcID), item)
		}

		return true
	}

	return false
}

// openZonePicker opens the zone picker for the NPC to live in.
func (s *State) openZonePicker(ctx *frame.Context, npcID ids.ULID) {
	now := s.zoneOfNPC(ctx.Docs, npcID)
	s.zones = zonepicker.New(zonepicker.Config{
		For: "for " + s.index(ctx.Docs).Name(npcID, "the NPC") + " to live in", Now: now,
		Recent: slices.Clone(s.recentZones), NoZone: false, NoneLabel: "", GoesOn: !now.IsZero(), Mine: nil,
	}, s.theme)
	s.zonesFor = npcID
}

// zoneOffer is what the zone picker lists, subscribing to what it needs
// when ctx is given.
func (s *State) zoneOffer(docs frame.Docs, ctx *frame.Context) zonepicker.Offer {
	subscribe := func(ids.ULID) {}
	if ctx != nil {
		subscribe = func(documentID ids.ULID) {
			if _, exists := s.index(docs).Entry(documentID); exists {
				s.subscribe(documentID, ctx)
			}
		}
	}

	return zonepicker.Offer{Zones: zonepicker.Gather(docs, subscribe), Folders: zonepicker.Folders(docs)}
}

// updateZonePicker runs the zone picker while it is open, and acts on what
// it chose.
func (s *State) updateZonePicker(input *ui.Input, ctx *frame.Context) {
	window := image.Rectangle{Min: image.Pt(0, 0), Max: s.size}
	choice, chosen, open := s.zones.Update(input, ctx.Clipboard, window, s.zoneOffer(ctx.Docs, ctx))
	if !open {
		s.zones = nil
	}
	if !chosen || choice.None {
		return
	}
	s.recentZones = slices.DeleteFunc(s.recentZones, func(zoneID ids.ULID) bool { return zoneID == choice.Zone })
	s.recentZones = append([]ids.ULID{choice.Zone}, s.recentZones...)
	s.recentZones = s.recentZones[:min(len(s.recentZones), recentZones)]
	s.chooseZone(ctx, s.zonesFor, choice.Zone)
}

// chooseZone gives an NPC the zone chosen. An NPC that lives nowhere yet
// takes it at once, as nothing can break; one that lives in a zone moves,
// asking first when anything changes but its zone.
func (s *State) chooseZone(ctx *frame.Context, npcID, zoneID ids.ULID) {
	now := s.zoneOfNPC(ctx.Docs, npcID)
	switch {
	case now == zoneID:
		return
	case !now.IsZero():
		s.moveZone(npcID, zoneID)

		return
	}
	name, zoneName := s.index(ctx.Docs).Name(npcID, "The NPC"), s.index(ctx.Docs).Name(zoneID, "the zone")
	s.command(ctx, npcID, npcplugin.SetZoneEdit, "", npcplugin.SetZoneArgs{Zone: zoneID},
		func(protocol.Result, error) {
			ctx.Frame.Status(name + " lives in " + zoneName + ". Undo takes it back.")
		}, nil)
}

// drawZonePicker draws the zone picker over the window while it is open.
func (s *State) drawZonePicker(screen *ebiten.Image, docs frame.Docs) {
	if s.zones == nil {
		return
	}
	s.zones.Draw(screen, image.Rectangle{Min: image.Pt(0, 0), Max: s.size}, s.zoneOffer(docs, nil))
}

// locateZonePicker finds a part of the open zone picker, or answers one of
// its states.
func (s *State) locateZonePicker(name string, docs frame.Docs) (image.Point, bool) {
	window := image.Rectangle{Min: image.Pt(0, 0), Max: s.size}
	offer := s.zoneOffer(docs, nil)
	if state, isState := strings.CutPrefix(name, "state:"); isState {
		if state == "zone-picker" {
			return image.Pt(0, 0), true
		}
		found, _ := s.zones.HasState(window, offer, state)

		return image.Pt(0, 0), found
	}

	return s.zones.Locate(window, offer, strings.TrimPrefix(name, "ui:"))
}

// hasZoneState answers the Zone box's states: "npc-zone=<name>", the zone
// the selected NPC lives in, or No zone yet, and "npc-placed=<line>", where
// it is placed.
func (s *State) hasZoneState(docs frame.Docs, state string) (bool, bool) {
	npcID, isNPC := s.selectedNPC(docs)
	key, want, _ := strings.Cut(state, "=")
	switch key {
	case "npc-zone":
		name := noZoneYet
		if zoneID := s.zoneOfNPC(docs, npcID); !zoneID.IsZero() {
			name = s.index(docs).Name(zoneID, noZoneYet)
		}

		return isNPC && name == want, true
	case "npc-placed":
		where, isPlaced := s.placementOf(docs, npcID)

		return isNPC && isPlaced && fmt.Sprintf("%s, at %d, %d", where.name, where.cell.X, where.cell.Y) == want, true
	}

	return false, false
}
