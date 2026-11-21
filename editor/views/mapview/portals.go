package mapview

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/pen"
	"github.com/bradhannah/berad-engine/editor/views/docsource"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/picker"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/gameload"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/worldstate"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// Portals are cells that lead to an anchor (increment 64), in another zone
// or in this one. The right-click menu adds one, opens where it leads,
// leads it somewhere else, changes how the screen changes, moves it or
// removes it; the Map draws each as a teal diamond with where it leads
// beside it, red when it leads nowhere. Where a portal leads is chosen by
// zone, then by clicking an anchor in that zone's Map.

const (
	menuAddPortal    = "add-portal"
	menuOpenPortal   = "open-portal"
	menuLeadPortal   = "lead-portal"
	menuMovePortal   = "move-portal"
	menuRemovePortal = "remove-portal"
	// portalChangeItem prefixes the way the screen changes in the item that
	// gives a portal that way.
	portalChangeItem = "portal-change:"
	// portalOwner is who the Map's picks are for.
	portalOwner  = "portal"
	portalLead   = "lead"
	hexPortal    = 0x0e7c86
	portalReach  = 0.3
	minPortal    = 4
	portalArrow  = "→ "
	nowhereLabel = "nowhere"
	// lastPlace is a place in a floor's stored order past any it holds, so
	// what is added there goes last.
	lastPlace = 1 << 30
)

// portalParts is the Map's part for portals: its own picks, the portal
// being moved, the portal waiting for where it leads, and where each portal
// of the shown floor leads, as last found.
type portalParts struct {
	picker *picker.Picker
	moving ids.ULID
	asking *askedPortal
	leads  map[ids.ULID]portalLeads
}

// askedPortal is a portal waiting for where it leads: a new one on a cell
// of a floor, or one already there being led somewhere else.
type askedPortal struct {
	floor  ids.ULID
	cell   Cell
	portal ids.ULID
}

// portalLeads is where a portal leads: the place, once found; whether it is
// found, awaited or nowhere, and why nowhere; and its anchor's name, alone
// and with its floor and zone.
type portalLeads struct {
	to     floor.AnchorRef
	start  game.Start
	state  gameload.State
	why    string
	anchor string
	place  string
}

func newPortalParts(window ids.ULID) portalParts {
	return portalParts{picker: picker.New(window), moving: ids.ULID{}, asking: nil, leads: map[ids.ULID]portalLeads{}}
}

// syncPortals finds where each portal of the shown floor leads, asking for
// the documents that say: the anchor's floor, and the zones, one of which
// lists it. It runs before the floor is read for the frame, since the menu
// and the drawing are made while it is read.
func (s *State) syncPortals(ctx *frame.Context) {
	var portals []floor.Portal
	ctx.Docs.View(s.floor, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			portals = floorDoc.Floor.Portals()
		}
	})
	clear(s.portals.leads)
	if len(portals) == 0 {
		return
	}
	asking := docsource.New(ctx.Docs).Asking(func(documentID ids.ULID) { s.subscribe(documentID, ctx) })
	names := gameload.NewNames(asking)
	for _, portal := range portals {
		leads := portalLeads{
			to: portal.To, start: game.Start{Zone: ids.ULID{}, Floor: ids.ULID{}, Col: 0, Row: 0},
			state: gameload.Missing, why: "", anchor: "", place: "",
		}
		leads.start, leads.state, leads.why = gameload.Destination(asking, portal.To)
		if leads.state == gameload.Held {
			asking.View(portal.To.Floor, func(document gameload.Document) {
				if held, ok := document.Value.(*floor.Floor); ok {
					if anchor, _, found := held.Anchor(portal.To.Anchor); found {
						leads.anchor = anchor.Name
					}
				}
			})
			leads.place = names.Place(worldstate.Place{Floor: portal.To.Floor, Anchor: portal.To.Anchor})
		}
		s.portals.leads[portal.ID] = leads
	}
}

// markItems are the cell menu's choices for what marks a cell without
// standing on it: an ambient sound and a portal.
func (s *State) markItems(floorData *floor.Floor, cell Cell) []ui.MenuItem {
	return append(s.soundItems(floorData, cell), s.portalItems(floorData, cell)...)
}

// chooseMark carries out a choice for an ambient sound or a portal.
func (s *State) chooseMark(ctx *frame.Context, floorData *floor.Floor, chosen string, cell Cell) {
	if !s.choosePortal(ctx, floorData, chosen, cell) {
		s.chooseSound(ctx, floorData, chosen, cell)
	}
}

// portalItems are the cell menu's choices for a portal on the cell, or for
// adding one.
func (s *State) portalItems(floorData *floor.Floor, cell Cell) []ui.MenuItem {
	portal, found := floorData.PortalAt(cell.X, cell.Y)
	if !found {
		return []ui.MenuItem{menuItem(menuAddPortal, "Add portal here…", false)}
	}
	var open ui.MenuItem
	switch leads := s.portals.leads[portal.ID]; leads.state {
	case gameload.Held:
		open = menuItem(menuOpenPortal, "Open where the portal leads: "+leads.place, false)
	case gameload.Waiting:
		open = menuItem(menuOpenPortal, "Finding where the portal leads…", true)
	case gameload.Missing:
		open = menuItem(menuOpenPortal, "The portal leads nowhere: "+leads.why, true)
	}
	items := []ui.MenuItem{open, menuItem(menuLeadPortal, "Lead the portal somewhere else…", false)}
	for _, change := range floor.Changes() {
		if change != portal.Change {
			items = append(items, menuItem(portalChangeItem+change, "Change the portal to "+changeNoun(change), false))
		}
	}

	return append(items, menuItem(menuMovePortal, "Move portal", false), dangerItem(menuRemovePortal, "Remove portal"))
}

// changeNoun names a way the screen changes, as "a cut".
func changeNoun(change string) string {
	if change == floor.ChangeFade {
		return "a fade through black"
	}

	return "a cut"
}

// changeVerb says how a portal changes the screen, as "cuts".
func changeVerb(change string) string {
	if change == floor.ChangeFade {
		return "fades through black"
	}

	return "cuts"
}

// choosePortal carries out a portal choice from the cell menu, and reports
// whether the choice was one.
func (s *State) choosePortal(ctx *frame.Context, floorData *floor.Floor, chosen string, cell Cell) bool {
	portal, found := floorData.PortalAt(cell.X, cell.Y)
	change, changing := strings.CutPrefix(chosen, portalChangeItem)
	switch {
	case chosen == menuAddPortal:
		s.askLeads(askedPortal{floor: s.floor, cell: cell, portal: ids.ULID{}})
	case chosen == menuLeadPortal && found:
		s.askLeads(askedPortal{floor: s.floor, cell: cell, portal: portal.ID})
	case chosen == menuOpenPortal && found:
		if leads := s.portals.leads[portal.ID]; leads.state == gameload.Held {
			ctx.Requests.OpenItem(ViewName, leads.start.Zone,
				AnchorItem+portal.To.Floor.String()+":"+portal.To.Anchor.String())
		}
	case changing && found:
		portal.Change = change
		s.command(ctx, s.floor, tilefloor.UpdatePortalEdit, "", tilefloor.UpdatePortalArgs{Portal: portal},
			func(protocol.Result) { ctx.Frame.Status("The portal now " + changeVerb(change) + ".") })
	case chosen == menuMovePortal && found:
		s.portals.moving = portal.ID
		ctx.Frame.Status("Click the cell to move the portal to. Escape cancels.")
	case chosen == menuRemovePortal && found:
		s.command(ctx, s.floor, tilefloor.RemovePortalEdit, "", tilefloor.RemovePortalArgs{ID: portal.ID},
			func(protocol.Result) { ctx.Frame.Status("Removed the portal. Undo brings it back.") })
	default:
		return false
	}

	return true
}

// askLeads asks which zone a portal leads into, this zone first in the
// list: the anchor is then picked in that zone's Map.
func (s *State) askLeads(asked askedPortal) {
	zones := s.names.OfFormat(zone.FormatName)
	if len(zones) == 0 {
		return
	}
	list := ui.NewList("zones", s.zone.String())
	for _, entry := range zones {
		row := ui.NewRow(entry.ID.String(), entry.Name)
		row.Icon = ui.IconZone
		if entry.ID == s.zone {
			row.Detail = "this zone"
		}
		list.Rows = append(list.Rows, row)
	}
	s.portals.asking = &asked
	s.dialog = ui.NewDialog("Lead to an anchor in which zone?",
		[]string{"Its Map opens for you to click the anchor. A zone needs an anchor to lead to."},
		ui.DialogButton(portalLead, "Pick an anchor there"), ui.DialogButton(ui.DialogCancel, "Cancel"))
	s.dialog.List = list
	s.dialogKind = dialogPortalZone
}

// pickLeads starts picking the anchor a portal leads to, in the zone
// chosen. The answer adds the portal, or leads the one asked about there.
func (s *State) pickLeads(ctx *frame.Context, chosen string) {
	asked := s.portals.asking
	zoneID, err := ids.Parse(chosen)
	if asked == nil || err != nil {
		s.portals.asking = nil

		return
	}
	now := protocol.PickNow{Floor: ids.ULID{}, Item: "", Name: ""}
	if leads, known := s.portals.leads[asked.portal]; known {
		now = protocol.PickNow{Floor: leads.to.Floor, Item: leads.to.Anchor.String(), Name: leads.anchor}
	}
	label := fmt.Sprintf("the portal at %d,%d of %s", asked.cell.X, asked.cell.Y, s.names.Name(asked.floor, "its floor"))
	s.portals.picker.StartIn(ctx, portalOwner, label, zoneID, now, func(answer protocol.PickAnswer, ctx *frame.Context) {
		s.portals.asking = nil
		anchorID, err := ids.Parse(answer.Item)
		if err != nil {
			return
		}
		s.leadPortal(ctx, *asked, floor.AnchorRef{Floor: answer.Floor, Anchor: anchorID}, answer.Name)
	})
}

// leadPortal adds a portal that leads to an anchor, or leads the portal
// asked about there. It reads the portal's floor, so it runs outside any
// reading of one.
func (s *State) leadPortal(ctx *frame.Context, asked askedPortal, leads floor.AnchorRef, name string) {
	if asked.portal.IsZero() {
		portal := floor.Portal{ID: ids.ULID{}, Col: asked.cell.X, Row: asked.cell.Y, To: leads, Change: floor.Changes()[0]}
		s.command(ctx, asked.floor, tilefloor.AddPortalEdit, "", tilefloor.AddPortalArgs{Portal: portal, Index: lastPlace},
			func(protocol.Result) {
				ctx.Frame.Status(fmt.Sprintf(
					"Added a portal at %d,%d that leads to %s. It %s; the way back is a portal of its own.",
					asked.cell.X, asked.cell.Y, labelOr(name, "the anchor"), changeVerb(portal.Change)))
			})

		return
	}
	var portal floor.Portal
	found := false
	ctx.Docs.View(asked.floor, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			portal, _, found = floorDoc.Floor.Portal(asked.portal)
		}
	})
	if !found {
		ctx.Frame.Refuse("That portal is no longer on its floor.")

		return
	}
	portal.To = leads
	s.command(ctx, asked.floor, tilefloor.UpdatePortalEdit, "", tilefloor.UpdatePortalArgs{Portal: portal},
		func(protocol.Result) {
			ctx.Frame.Status("The portal now leads to " + labelOr(name, "the anchor") + ".")
		})
}

// updateMovingPortal moves the portal being moved to the clicked cell, or
// stops on Escape.
func (s *State) updateMovingPortal(
	input *ui.Input, ctx *frame.Context, floorData *floor.Floor, cell Cell, onFloor bool,
) {
	stop, place := movingClick(input, ctx, onFloor)
	moving := s.portals.moving
	if stop || place {
		s.portals.moving = ids.ULID{}
	}
	portal, _, found := floorData.Portal(moving)
	_, taken := floorData.PortalAt(cell.X, cell.Y)
	switch {
	case !place || !found:
	case taken:
		ctx.Frame.Refuse("Another portal is already there")
	default:
		portal.Col, portal.Row = cell.X, cell.Y
		s.command(ctx, s.floor, tilefloor.UpdatePortalEdit, "", tilefloor.UpdatePortalArgs{Portal: portal},
			func(protocol.Result) { ctx.Frame.Status(fmt.Sprintf("Moved the portal to %d,%d", cell.X, cell.Y)) })
	}
}

// leadingHere lists the portals of a floor that lead to one of its own
// anchors, as where-used names them. The project's answer to where an
// anchor is used leaves out the anchor's own floor.
func leadingHere(floorData *floor.Floor, floorID, anchorID ids.ULID) []string {
	var leading []string
	for _, portal := range floorData.Portals() {
		if portal.To.Floor == floorID && portal.To.Anchor == anchorID {
			leading = append(leading, fmt.Sprintf("%s%d, %d", tilefloor.PortalLabel, portal.Col, portal.Row))
		}
	}

	return leading
}

// drawPortals draws each portal on the floor: a teal diamond, with where it
// leads beside its cell when there is room, and how it changes the screen
// when that is not a cut. One that leads nowhere is red, and says so.
func (s *State) drawPortals(area *ebiten.Image, floorData *floor.Floor, grid image.Rectangle) {
	size := float32(s.camera.cellSize())
	for _, portal := range floorData.Portals() {
		rect := s.camera.cellRect(Cell{X: portal.Col, Y: portal.Row}, grid)
		if !rect.Overlaps(grid) {
			continue
		}
		leads := s.portals.leads[portal.ID]
		ink, name := ui.Hex(hexPortal), leads.anchor
		if leads.state == gameload.Missing {
			ink, name = ui.Hex(hexRemoved), nowhereLabel
		}
		centreX, centreY := float32(rect.Min.X)+size/ui.Half, float32(rect.Min.Y)+size/ui.Half
		reach := max(size*portalReach, minPortal)
		drawDiamond(area, centreX, centreY, reach+tokenRing, ui.ColorReverse())
		drawDiamond(area, centreX, centreY, reach, ink)
		if rect.Dx() < minLabel || leads.state == gameload.Waiting {
			continue
		}
		label := portalArrow + name
		if portal.Change != floor.ChangeCut {
			label += " · " + portal.Change
		}
		labelX, labelY := rect.Max.X+ui.Half, rect.Min.Y-ui.Gap
		backing := image.Rect(labelX-1, labelY, labelX+s.theme.SmallWidth(label)+ui.Half, labelY+ui.LabelSize+ui.Gap+ui.Half)
		ui.FillRect(area, backing, ui.HexAlpha(hexWhite, labelBacking))
		s.theme.DrawSmall(area, label, labelX, labelY, ink)
	}
}

// drawDiamond fills a square stood on its corner, reach from its centre to
// each corner.
func drawDiamond(area *ebiten.Image, centreX, centreY, reach float32, fill color.Color) {
	var diamond vector.Path
	diamond.MoveTo(centreX, centreY-reach)
	diamond.LineTo(centreX+reach, centreY)
	diamond.LineTo(centreX, centreY+reach)
	diamond.LineTo(centreX-reach, centreY)
	diamond.Close()
	options := &vector.DrawPathOptions{AntiAlias: true, ColorScale: ebiten.ColorScale{}, Blend: ebiten.BlendSourceOver}
	options.ColorScale.ScaleWithColor(fill)
	pen.FillPath(area, &diamond, &vector.FillOptions{FillRule: vector.FillRuleNonZero}, options)
}

// hasMark answers the states of what marks a cell: "ambient:" for an
// ambient sound and "portal:" for a portal.
func (s *State) hasMark(state string, docs frame.Docs) bool {
	if value, portal := strings.CutPrefix(state, "portal:"); portal {
		return s.hasPortal(value, docs)
	}

	return s.hasSound(strings.TrimPrefix(state, "ambient:"), docs)
}

// hasPortal answers "portal:C,R" for input scripts: a portal is on that cell
// of the shown floor; "portal:C,R:<change>", one that changes the screen that
// way; "portal:C,R:to:<anchor>", one that leads to the anchor of that name;
// and "portal:C,R:nowhere", one that leads nowhere.
func (s *State) hasPortal(value string, docs frame.Docs) bool {
	where, rest, _ := strings.Cut(value, ":")
	var col, row int
	if _, err := fmt.Sscanf(where, "%d,%d", &col, &row); err != nil {
		return false
	}
	var portal floor.Portal
	found := false
	docs.View(s.floor, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			portal, found = floorDoc.Floor.PortalAt(col, row)
		}
	})
	anchor, named := strings.CutPrefix(rest, "to:")
	switch {
	case !found:
		return false
	case named:
		return s.portals.leads[portal.ID].anchor == anchor
	case rest == nowhereLabel:
		return s.portals.leads[portal.ID].state == gameload.Missing
	}

	return rest == "" || rest == portal.Change
}
