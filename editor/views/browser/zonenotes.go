package browser

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// newZoneNotes makes the field for a zone's private notes.
func newZoneNotes(theme *ui.Theme) *ui.TextField {
	notes := ui.NewTextField("zone-notes", textedit.NewField("", true, zone.MaxNotes), true, theme)
	notes.Placeholder = "Private notes about this zone. Players never see them; the zone picker shows them."

	return notes
}

// notedZone returns the selected document's ID when it is a zone.
func (s *State) notedZone(docs frame.Docs) (ids.ULID, bool) {
	entry, ok := s.selected(docs)

	return entry.ID, ok && entry.Format == zone.FormatName
}

// syncZoneNotes keeps the notes field showing the selected zone's saved
// notes. The guard has already asked about unsaved notes before the
// selection moved.
func (s *State) syncZoneNotes(ctx *frame.Context) {
	zoneID, ok := s.notedZone(ctx.Docs)
	if !ok || zoneID != s.zoneNoted {
		s.zoneNotes.Field.Revert()
		s.zoneNoted = ids.ULID{}
	}
	if !ok {
		return
	}
	s.zoneNoted = zoneID
	ctx.Docs.View(zoneID, func(document client.Document) {
		if zoneDoc, isZone := document.Model.(*zoneplugin.Document); isZone {
			s.zoneNotes.Field.SetSaved(zoneDoc.Zone.Notes)
		}
	})
}

// layoutZoneNotes places a selected zone's notes below its palettes, down
// to limit.
func (s *State) layoutZoneNotes(docs frame.Docs, left, width, top, limit int) {
	s.zoneNotes.Rect = ui.Zero()
	if _, ok := s.notedZone(docs); !ok {
		return
	}
	rowY := top + panelGap + ui.LineHeight
	s.zoneNotes.Rect = image.Rect(left, rowY, left+width, rowY+max(min(notesHeight, limit-rowY), notesLeast))
}

// drawZoneNotes draws a selected zone's notes.
func (s *State) drawZoneNotes(screen *ebiten.Image, left int) {
	if s.zoneNotes.Rect.Empty() {
		return
	}
	s.theme.Heading(screen, "NOTES", left, s.zoneNotes.Rect.Min.Y-ui.LineHeight)
	s.zoneNotes.Draw(screen)
	if s.zoneNotes.Field.Conflict() {
		s.theme.DrawText(screen, "Changed elsewhere. Saving keeps yours.", left, s.zoneNotes.Rect.Max.Y+ui.Gap,
			ui.ColorWarn())
	}
}

// commitZoneNotes saves the selected zone's notes, from Save or Cmd+S.
func (s *State) commitZoneNotes(ctx *frame.Context) {
	notes, changed := s.zoneNotes.Field.Commit()
	if !changed {
		s.focus.Set(nil)

		return
	}
	s.command(ctx, s.zoneNoted, zoneplugin.SetNotesEdit, "", zoneplugin.SetNotesArgs{Notes: notes},
		func(protocol.Result, error) {
			s.focus.Set(nil)
			ctx.Frame.Status("Notes saved")
		},
		func() { s.zoneNotes.Field.Failed() })
}
