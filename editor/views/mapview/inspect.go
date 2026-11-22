package mapview

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/gameload"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// The inspector's sections for a selected ambient sound, portal or player
// start (increment 65). What the right-click menu used to set is set here:
// a sound's cue, and where a portal leads and how the screen changes. A
// button or a choice applies at once, as the standing rule has it, and
// every change is one edit with undo.

// Where a section's parts are, from the inspector's top, and its buttons'
// IDs.
const (
	sectionTop       = ui.Inset + 4*ui.LineHeight + ui.LineHeight/ui.Half
	soundButtonsTop  = sectionTop + 4*ui.LineHeight + ui.LineHeight/ui.Half
	portalButtonsTop = sectionTop + 6*ui.LineHeight
	soundChangeID    = "sound-change"
	soundRemoveID    = "sound-remove"
	portalOpenID     = "portal-open"
	portalLeadID     = "portal-lead"
	portalFadeID     = "portal-fade"
	portalCutID      = "portal-cut"
	portalRemoveID   = "portal-remove"
)

// inspectParts are the inspector's buttons for a selected ambient sound and
// a selected portal.
type inspectParts struct {
	soundChange, soundRemove                                    *ui.Button
	portalOpen, portalLead, portalFade, portalCut, portalRemove *ui.Button
}

func newInspectParts() inspectParts {
	parts := inspectParts{
		soundChange: ui.NewButton(soundChangeID, "Change sound"), soundRemove: ui.NewButton(soundRemoveID, "Remove sound"),
		portalOpen: ui.NewButton(portalOpenID, "Open where it leads"),
		portalLead: ui.NewButton(portalLeadID, "Lead somewhere else…"),
		portalFade: ui.NewButton(portalFadeID, "Fade through black"), portalCut: ui.NewButton(portalCutID, "Cut"),
		portalRemove: ui.NewButton(portalRemoveID, "Remove portal"),
	}
	parts.soundChange.Caret = true
	parts.soundRemove.Danger, parts.portalRemove.Danger = true, true

	return parts
}

// buttons lists every button of the sections.
func (p *inspectParts) buttons() []*ui.Button {
	return []*ui.Button{
		p.soundChange, p.soundRemove, p.portalOpen, p.portalLead, p.portalFade, p.portalCut, p.portalRemove,
	}
}

// layoutInspect places the buttons of the section for what is selected,
// and hides the rest.
func (s *State) layoutInspect(left, width int) {
	parts := &s.inspect
	for _, button := range parts.buttons() {
		button.Rect = ui.Zero()
	}
	row := func(top int) image.Rectangle { return image.Rect(left, top, left+width, top+ui.ButtonH) }
	switch {
	case !s.anchor.IsZero() || !s.placement.IsZero():
	case !s.sound.IsZero():
		top := s.down(soundButtonsTop)
		parts.soundChange.Rect = row(top)
		parts.soundRemove.Rect = row(top + ui.ButtonH + ui.Gap)
	case !s.portal.IsZero():
		top := s.down(portalButtonsTop)
		parts.portalOpen.Rect = row(top)
		top += ui.ButtonH + ui.Gap
		parts.portalLead.Rect = row(top)
		top += ui.ButtonH + ui.Half*ui.LineHeight
		half := (width - ui.Gap) / ui.Half
		parts.portalFade.Rect = image.Rect(left, top, left+half, top+ui.ButtonH)
		parts.portalCut.Rect = image.Rect(left+half+ui.Gap, top, left+width, top+ui.ButtonH)
		parts.portalRemove.Rect = row(top + ui.ButtonH + ui.LineHeight)
	}
}

// updateInspect handles the buttons of the section for a selected ambient
// sound or portal.
func (s *State) updateInspect(input *ui.Input, ctx *frame.Context) {
	parts := &s.inspect
	pressed := ""
	for _, button := range parts.buttons() {
		if !button.Rect.Empty() && button.Update(input) {
			pressed = button.ID
		}
	}
	ctx.Docs.View(s.floor, func(document client.Document) {
		floorDoc, ok := document.Model.(*tilefloor.Document)
		if !ok {
			return
		}
		if portal, _, found := floorDoc.Floor.Portal(s.portal); found && !s.portal.IsZero() {
			parts.portalFade.On, parts.portalCut.On = portal.Change == floor.ChangeFade, portal.Change == floor.ChangeCut
			parts.portalOpen.Disabled = s.portals.leads[portal.ID].state != gameload.Held
			s.pressPortal(ctx, portal, pressed)
		}
		if sound, _, found := floorDoc.Floor.Sound(s.sound); found && !s.sound.IsZero() {
			parts.soundChange.Disabled = len(s.ambientCues()) == 0
			s.pressSound(ctx, sound, pressed)
		}
	})
}

// pressPortal carries out a press of one of the selected portal's buttons.
func (s *State) pressPortal(ctx *frame.Context, portal floor.Portal, pressed string) {
	change := map[string]string{portalFadeID: floor.ChangeFade, portalCutID: floor.ChangeCut}[pressed]
	switch {
	case pressed == portalOpenID:
		if leads := s.portals.leads[portal.ID]; leads.state == gameload.Held {
			ctx.Requests.OpenItem(ViewName, leads.start.Zone,
				AnchorItem+portal.To.Floor.String()+":"+portal.To.Anchor.String())
		}
	case pressed == portalLeadID:
		s.askLeads(askedPortal{floor: s.floor, cell: Cell{X: portal.Col, Y: portal.Row}, portal: portal.ID})
	case pressed == portalRemoveID:
		s.removePortal(ctx, portal.ID)
	case change != "" && change != portal.Change:
		portal.Change = change
		s.command(ctx, s.floor, tilefloor.UpdatePortalEdit, "", tilefloor.UpdatePortalArgs{Portal: portal},
			func(protocol.Result) { ctx.Frame.Status("The portal now " + changeVerb(change) + ".") })
	}
}

// pressSound carries out a press of one of the selected sound's buttons.
// Change sound opens the ambient cues under its button, the sound's own
// ticked off.
func (s *State) pressSound(ctx *frame.Context, sound floor.Sound, pressed string) {
	switch pressed {
	case soundRemoveID:
		s.removeSound(ctx, sound.ID)
	case soundChangeID:
		s.ambient.changing = sound.ID
		rect := s.inspect.soundChange.Rect
		s.openCueMenu(Cell{X: sound.Col, Y: sound.Row}, sound.Cue, image.Pt(rect.Min.X, rect.Max.Y),
			image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
	}
}

// drawSoundInspector draws the section for the selected ambient sound.
func (s *State) drawSoundInspector(screen *ebiten.Image, floorData *floor.Floor, textX int, textY *int) {
	sound, _, found := floorData.Sound(s.sound)
	if !found {
		return
	}
	line := func(text string, ink color.Color) {
		s.theme.DrawText(screen, text, textX, *textY, ink)
		*textY += ui.LineHeight
	}
	s.theme.Heading(screen, "AMBIENT SOUND", textX, *textY)
	*textY += ui.LineHeight
	s.theme.DrawStrong(screen, s.cueName(sound.Cue), textX, *textY, ui.ColorInk())
	*textY += ui.LineHeight
	line(fmt.Sprintf("at %d, %d", sound.Col, sound.Row), ui.ColorInk())
	line("Heard "+s.heardWithin(sound.Cue)+".", ui.ColorMuted())
	s.inspect.soundChange.Draw(screen, s.theme)
	s.inspect.soundRemove.Draw(screen, s.theme)
	*textY = s.inspect.soundRemove.Rect.Max.Y + ui.Gap
	line("Drag the marker to move it.", ui.ColorMuted())
}

// drawPortalInspector draws the section for the selected portal: where it
// leads, by its anchor and then its floor and zone, or that it leads
// nowhere and why; and how the screen changes, the way chosen filled.
func (s *State) drawPortalInspector(screen *ebiten.Image, floorData *floor.Floor, textX int, textY *int) {
	portal, _, found := floorData.Portal(s.portal)
	if !found {
		return
	}
	line := func(text string, ink color.Color) {
		s.theme.DrawText(screen, text, textX, *textY, ink)
		*textY += ui.LineHeight
	}
	s.theme.Heading(screen, "PORTAL", textX, *textY)
	*textY += ui.LineHeight
	line(fmt.Sprintf("at %d, %d", portal.Col, portal.Row), ui.ColorInk())
	*textY += ui.LineHeight / ui.Half
	s.theme.Heading(screen, "LEADS TO", textX, *textY)
	*textY += ui.LineHeight
	switch leads := s.portals.leads[portal.ID]; leads.state {
	case gameload.Held:
		s.theme.DrawStrong(screen, leads.anchor, textX, *textY, ui.ColorInk())
		*textY += ui.LineHeight
		line(strings.TrimPrefix(leads.place, leads.anchor+", "), ui.ColorMuted())
	case gameload.Waiting:
		line("Finding where it leads…", ui.ColorMuted())
	case gameload.Missing:
		s.theme.DrawStrong(screen, "Nowhere", textX, *textY, ui.Hex(hexRemoved))
		*textY += ui.LineHeight
		line(upperFirst(leads.why)+".", ui.ColorMuted())
	}
	parts := &s.inspect
	parts.portalOpen.Draw(screen, s.theme)
	parts.portalLead.Draw(screen, s.theme)
	s.theme.Heading(screen, "HOW THE SCREEN CHANGES", textX, parts.portalFade.Rect.Min.Y-ui.LineHeight)
	parts.portalFade.Draw(screen, s.theme)
	parts.portalCut.Draw(screen, s.theme)
	parts.portalRemove.Draw(screen, s.theme)
	*textY = parts.portalRemove.Rect.Max.Y + ui.Gap
	line("Drag the diamond to move it.", ui.ColorMuted())
}

// drawStartInspector draws the section for the selected player start.
func (s *State) drawStartInspector(screen *ebiten.Image, textX int, textY *int) {
	line := func(text string, ink color.Color) {
		s.theme.DrawText(screen, text, textX, *textY, ink)
		*textY += ui.LineHeight
	}
	s.theme.Heading(screen, "PLAYER START", textX, *textY)
	*textY += ui.LineHeight
	line(fmt.Sprintf("at %d, %d", s.playerStart.Col, s.playerStart.Row), ui.ColorInk())
	line("Launch puts the player here.", ui.ColorMuted())
	line("Drag the marker to move it.", ui.ColorMuted())
}

// upperFirst starts a sentence with a capital.
func upperFirst(text string) string {
	if text == "" {
		return text
	}

	return strings.ToUpper(text[:1]) + text[1:]
}
