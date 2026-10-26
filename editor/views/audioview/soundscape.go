package audioview

import (
	"image"
	"math"
	"strconv"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/sound"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// The soundscape plays up to four cues together, two ambient, one music and
// one SFX, so an author hears how they sit together; and the monitor sets
// how loud each category plays while listening, standing in for a player's
// volume settings. It shows in place of the inspector. Nothing of it is
// saved.

// The soundscape's layers, by slot.
const (
	slotAmbient1 = "ambient-1"
	slotAmbient2 = "ambient-2"
	slotMusic    = "music"
	slotSFX      = "sfx"
)

// Soundscape values: how long an SFX rests before it plays again, the
// monitor's levels and where they start, and how much a layer's gain
// buttons change it.
const (
	sfxRest       = 500 * time.Millisecond
	monitorLevels = 10
	monitorStart  = 7
	layerStep     = 0.1
	replaceItem   = "replace:"
	smallButton   = 26
	levelText     = 40
	gainText      = 52
)

// layerSlot is one layer: the cue it plays, its gain, whether it is muted,
// soloed or plays again, and its controls.
type layerSlot struct {
	id, label, category string
	cue                 string
	gain                float64
	muted, soloed       bool
	again               bool
	play, quieter       *ui.Button
	louder, mute, solo  *ui.Button
	remove              *ui.Button
	againToggle         *ui.Toggle
	// top is where its label is drawn.
	top int
}

// monitorRow sets one category's monitor level.
type monitorRow struct {
	category, label string
	level           int
	quieter, louder *ui.Button
}

// soundscape is the soundscape and monitor: whether they show, their
// layers and levels, the monitor last sent, and the menu that asks which
// ambient layer a cue replaces.
type soundscape struct {
	shown     bool
	toggle    *ui.Button
	slots     []*layerSlot
	monitor   []*monitorRow
	reference *ui.Toggle
	sent      *sound.Monitor
	replace   ui.Menu
	waiting   string
	top       int
}

func newSoundscape() *soundscape {
	slot := func(slotID, label, category, again string) *layerSlot {
		made := &layerSlot{
			id: slotID, label: label, category: category, cue: "", gain: 1, muted: false, soloed: false,
			again: false, play: ui.NewButton("layer-play-"+slotID, "Play"),
			quieter: ui.NewButton("layer-quieter-"+slotID, "−"), louder: ui.NewButton("layer-louder-"+slotID, "+"),
			mute: ui.NewButton("layer-mute-"+slotID, "Mute"), solo: ui.NewButton("layer-solo-"+slotID, "Solo"),
			remove: ui.NewButton("layer-remove-"+slotID, "×"), againToggle: nil, top: 0,
		}
		if again != "" {
			made.againToggle = &ui.Toggle{ID: "layer-again-" + slotID, Label: again, Rect: ui.Zero(), On: false}
		}

		return made
	}
	row := func(category, label string) *monitorRow {
		return &monitorRow{
			category: category, label: label, level: monitorStart,
			quieter: ui.NewButton("monitor-quieter-"+category, "−"), louder: ui.NewButton("monitor-louder-"+category, "+"),
		}
	}

	return &soundscape{
		shown: false, toggle: ui.NewButton("soundscape", "Soundscape"),
		slots: []*layerSlot{
			slot(slotAmbient1, "AMBIENT 1", protocol.CategoryAmbient, ""),
			slot(slotAmbient2, "AMBIENT 2", protocol.CategoryAmbient, ""),
			slot(slotMusic, "MUSIC", protocol.CategoryMusic, "Loop"),
			slot(slotSFX, "SFX", protocol.CategorySFX, "Again every half second"),
		},
		monitor: []*monitorRow{
			row(protocol.CategoryMusic, "Music"), row(protocol.CategorySFX, "SFX"),
			row(protocol.CategoryAmbient, "Ambient"),
		},
		reference: &ui.Toggle{
			ID: "reference", Label: "Reference: every layer evened out, at Normal", Rect: ui.Zero(), On: false,
		},
		sent: nil, replace: ui.Menu{Items: nil, Width: 0}, waiting: "", top: 0,
	}
}

func (slot *layerSlot) buttons() []*ui.Button {
	return []*ui.Button{slot.play, slot.quieter, slot.louder, slot.mute, slot.solo, slot.remove}
}

func (scape *soundscape) buttons() []*ui.Button {
	buttons := []*ui.Button{}
	for _, slot := range scape.slots {
		buttons = append(buttons, slot.buttons()...)
	}
	for _, row := range scape.monitor {
		buttons = append(buttons, row.quieter, row.louder)
	}

	return buttons
}

func (scape *soundscape) toggles() []*ui.Toggle {
	toggles := []*ui.Toggle{scape.reference}
	for _, slot := range scape.slots {
		if slot.againToggle != nil {
			toggles = append(toggles, slot.againToggle)
		}
	}

	return toggles
}

// hide leaves the soundscape's controls with no place, while the inspector
// shows.
func (scape *soundscape) hide() {
	for _, button := range scape.buttons() {
		button.Rect = ui.Zero()
	}
	for _, toggle := range scape.toggles() {
		toggle.Rect = ui.Zero()
	}
}

func (scape *soundscape) slot(id string) *layerSlot {
	for _, slot := range scape.slots {
		if slot.id == id {
			return slot
		}
	}

	return nil
}

// layer is what a slot plays: nothing when muted, or when another layer is
// soloed and it is not.
func (scape *soundscape) layer(slot *layerSlot) sound.Layer {
	soloing := false
	for _, other := range scape.slots {
		soloing = soloing || other.soloed && other.cue != ""
	}
	gain := slot.gain
	if slot.muted || soloing && !slot.soloed {
		gain = 0
	}
	layer := sound.Layer{Slot: slot.id, Cue: slot.cue, Gain: gain, Loop: false, Again: false, Rest: 0}
	switch slot.category {
	case protocol.CategoryAmbient:
		layer.Loop = true
	case protocol.CategorySFX:
		layer.Again, layer.Rest = slot.again, sfxRest
	default:
		layer.Again = slot.again
	}

	return layer
}

// addToSoundscape puts a cue in the layer for its category and plays it,
// keeping the layer's gain, mute and solo. An ambient cue fills an empty
// ambient layer, or asks which one it replaces.
func (s *State) addToSoundscape(ctx *frame.Context, cue protocol.AudioCue, anchor image.Rectangle) {
	scape := s.soundscape
	scape.shown = true
	switch cue.Category {
	case protocol.CategoryMusic:
		s.fillLayer(ctx, scape.slot(slotMusic), cue.ID)
	case protocol.CategoryAmbient:
		first, second := scape.slot(slotAmbient1), scape.slot(slotAmbient2)
		switch {
		case first.cue == "" || first.cue == cue.ID:
			s.fillLayer(ctx, first, cue.ID)
		case second.cue == "" || second.cue == cue.ID:
			s.fillLayer(ctx, second, cue.ID)
		default:
			scape.waiting = cue.ID
			scape.replace.Items = nil
			for _, slot := range []*layerSlot{first, second} {
				scape.replace.Items = append(scape.replace.Items, ui.MenuItem{
					ID: replaceItem + slot.id, Label: "Replace " + s.cueName(slot.cue), Icon: ui.IconSound,
					Disabled: false, Danger: false, Rule: false,
				})
			}
			window := image.Rectangle{Min: image.Pt(0, 0), Max: s.size}
			scape.replace.Width = ui.MenuWidth(scape.replace.Items, s.theme)
			scape.replace.Open(anchor, window)
		}
	default:
		s.fillLayer(ctx, scape.slot(slotSFX), cue.ID)
	}
}

// fillLayer puts a cue in a layer and plays it from its start.
func (s *State) fillLayer(ctx *frame.Context, slot *layerSlot, cue string) {
	slot.cue = cue
	ctx.Soundscape.PlayLayer(s.soundscape.layer(slot))
}

// cueName is a cue's name, or its ID once it is gone.
func (s *State) cueName(id string) string {
	if cue, found := s.cue(id); found {
		return cue.Name
	}

	return id
}

// updateReplace handles the menu asking which ambient layer a cue replaces.
func (s *State) updateReplace(input *ui.Input, ctx *frame.Context) {
	scape := s.soundscape
	chosen := scape.replace.Update(input)
	input.ClearAll()
	if slot := scape.slot(chosen[min(len(replaceItem), len(chosen)):]); slot != nil {
		s.fillLayer(ctx, slot, scape.waiting)
	}
	if !scape.replace.IsOpen() {
		scape.waiting = ""
	}
}

// syncSoundscape empties a layer whose cue is gone, and sends the monitor
// when it changed.
func (s *State) syncSoundscape(ctx *frame.Context) {
	scape := s.soundscape
	for _, slot := range scape.slots {
		if _, found := s.cue(slot.cue); slot.cue != "" && s.library != nil && !found {
			ctx.Soundscape.StopLayer(slot.id)
			slot.cue = ""
		}
	}
	monitor := sound.Monitor{Music: 1, SFX: 1, Ambient: 1, Reference: scape.reference.On}
	for _, row := range scape.monitor {
		gain := s.levelGain(row.level)
		switch row.category {
		case protocol.CategoryMusic:
			monitor.Music = gain
		case protocol.CategoryAmbient:
			monitor.Ambient = gain
		default:
			monitor.SFX = gain
		}
	}
	if scape.sent == nil || *scape.sent != monitor {
		ctx.Soundscape.SetMonitor(monitor)
		scape.sent = &monitor
	}
}

// levelGain is a monitor level's gain: the audio project's own volume
// levels when it has all eleven, or even steps.
func (s *State) levelGain(level int) float64 {
	if s.library != nil && len(s.library.SliderGains) == monitorLevels+1 {
		return s.library.SliderGains[level]
	}

	return float64(level) / monitorLevels
}

// layoutSoundscape places the layers and the monitor in the inspector's
// place.
func (s *State) layoutSoundscape() {
	scape, area := s.soundscape, s.inspectorArea()
	left, right := area.Min.X+ui.Inset, area.Max.X-ui.Inset
	top := area.Min.Y + ui.Inset + ui.LineHeight*ui.Half + ui.Inset
	for _, slot := range scape.slots {
		top = s.layoutLayer(slot, left, right, top)
	}
	scape.top = top
	top += ui.LineHeight
	for _, row := range scape.monitor {
		buttonLeft := left + s.theme.TextWidth("Ambient") + ui.Inset
		row.quieter.Rect = image.Rect(buttonLeft, top, buttonLeft+smallButton, top+ui.ButtonH)
		row.louder.Rect = row.quieter.Rect.Add(image.Pt(smallButton+levelText, 0))
		row.quieter.Disabled, row.louder.Disabled = row.level <= 0, row.level >= monitorLevels
		top += ui.ButtonH + ui.Gap
	}
	scape.reference.Rect = image.Rect(left, top, right, top+ui.ButtonH)
}

// layoutLayer places one layer's controls from a top edge and returns its
// bottom: its cue with Play and ×, then its gain, Mute and Solo, then
// whether it plays again.
func (s *State) layoutLayer(slot *layerSlot, left, right, top int) int {
	slot.top = top
	for _, button := range slot.buttons() {
		button.Rect = ui.Zero()
	}
	if slot.againToggle != nil {
		slot.againToggle.Rect = ui.Zero()
	}
	top += ui.LineHeight
	if slot.cue == "" {
		return top + ui.LineHeight + ui.Inset
	}
	slot.remove.Rect = image.Rect(right-smallButton, top, right, top+ui.ButtonH)
	slot.play.Label = "Play"
	if s.mixer != nil && s.mixer.LayerPlaying(slot.id) {
		slot.play.Label = labelStop
	}
	playWidth := s.theme.TextWidth("Play") + ui.Half*ui.Inset
	slot.play.Rect = image.Rect(slot.remove.Rect.Min.X-ui.Gap*ui.Half-playWidth, top,
		slot.remove.Rect.Min.X-ui.Gap*ui.Half, top+ui.ButtonH)
	top += ui.ButtonH + ui.Gap
	slot.quieter.Rect = image.Rect(left, top, left+smallButton, top+ui.ButtonH)
	slot.louder.Rect = slot.quieter.Rect.Add(image.Pt(smallButton+gainText, 0))
	s.row(slot.louder.Rect.Max.X+ui.Gap*ui.Half, top, slot.mute, slot.solo)
	slot.quieter.Disabled, slot.louder.Disabled = slot.gain <= 0, slot.gain >= 1
	slot.mute.On, slot.solo.On = slot.muted, slot.soloed
	top += ui.ButtonH + ui.Gap
	if slot.againToggle != nil {
		slot.againToggle.Rect = image.Rect(left, top, right, top+ui.ButtonH)
		slot.againToggle.On = slot.again
		top += ui.ButtonH
	}

	return top + ui.Inset
}

// updateSoundscape handles the layers' and the monitor's controls.
func (s *State) updateSoundscape(input *ui.Input, ctx *frame.Context) {
	scape := s.soundscape
	changed := false
	for _, slot := range scape.slots {
		changed = s.updateLayer(input, ctx, slot) || changed
	}
	if changed {
		for _, slot := range scape.slots {
			if slot.cue != "" {
				ctx.Soundscape.SetLayer(scape.layer(slot))
			}
		}
	}
	for _, row := range scape.monitor {
		switch {
		case row.quieter.Update(input):
			row.level--
		case row.louder.Update(input):
			row.level++
		}
		row.level = min(max(row.level, 0), monitorLevels)
	}
	scape.reference.Update(input)
}

// updateLayer handles one layer's controls and reports whether what plays
// changed: its gain, mute, solo or whether it plays again.
func (s *State) updateLayer(input *ui.Input, ctx *frame.Context, slot *layerSlot) bool {
	switch {
	case slot.play.Update(input):
		if ctx.Soundscape.LayerPlaying(slot.id) {
			ctx.Soundscape.StopLayer(slot.id)
		} else {
			ctx.Soundscape.PlayLayer(s.soundscape.layer(slot))
		}
	case slot.remove.Update(input):
		ctx.Soundscape.StopLayer(slot.id)
		slot.cue = ""
	case slot.quieter.Update(input):
		slot.gain = stepGain(slot.gain - layerStep)
	case slot.louder.Update(input):
		slot.gain = stepGain(slot.gain + layerStep)
	case slot.mute.Update(input):
		slot.muted = !slot.muted
	case slot.solo.Update(input):
		slot.soloed = !slot.soloed
	case slot.againToggle != nil && slot.againToggle.Update(input):
		slot.again = slot.againToggle.On
	default:
		return false
	}

	return true
}

// stepGain keeps a gain between 0 and 1, on a step.
func stepGain(gain float64) float64 {
	return math.Round(min(max(gain, 0), 1)/layerStep) * layerStep
}

// drawSoundscape draws the layers and the monitor in the inspector's place.
func (s *State) drawSoundscape(screen *ebiten.Image) {
	scape, area := s.soundscape, s.inspectorArea()
	frame.DrawInspector(screen)
	left, right := area.Min.X+ui.Inset, area.Max.X-ui.Inset
	top := area.Min.Y + ui.Inset
	s.theme.DrawLabel(screen, "SOUNDSCAPE · LISTENING ONLY, NEVER SAVED", left, top, ui.ColorMuted())
	s.theme.DrawText(screen, "Cues played together, to hear the mix. Meant to sound sets a cue's loudness.", left,
		top+ui.LineHeight, ui.ColorMuted())
	for _, slot := range scape.slots {
		s.drawLayer(screen, slot, left, right)
	}
	s.theme.DrawLabel(screen, "PRETEND PLAYER VOLUME · NOT SAVED", left, scape.top+ui.Gap, ui.ColorMuted())
	for _, row := range scape.monitor {
		textTop := row.quieter.Rect.Min.Y + (ui.ButtonH-ui.LineHeight)/ui.Half + ui.Gap
		s.theme.DrawText(screen, row.label, left, textTop, ui.ColorInk())
		s.theme.DrawTextIn(screen, strconv.Itoa(row.level), image.Rect(row.quieter.Rect.Max.X, row.quieter.Rect.Min.Y,
			row.louder.Rect.Min.X, row.louder.Rect.Max.Y), ui.ColorInk())
		percentText := strconv.Itoa(int(math.Round(s.levelGain(row.level)*percent))) + " %"
		s.theme.DrawText(screen, percentText, row.louder.Rect.Max.X+ui.Inset, textTop, ui.ColorMuted())
		s.drawButtons(screen, row.quieter, row.louder)
	}
	scape.reference.Draw(screen, s.theme)
	if scape.replace.IsOpen() {
		scape.replace.Draw(screen, s.theme)
	}
}

// drawLayer draws one layer: its label, its cue or that it is empty, and
// its controls.
func (s *State) drawLayer(screen *ebiten.Image, slot *layerSlot, left, right int) {
	s.theme.DrawLabel(screen, slot.label, left, slot.top+ui.Gap, ui.ColorMuted())
	top := slot.top + ui.LineHeight
	if slot.cue == "" {
		hint := "Empty: Add to soundscape on a cue fills it."
		s.theme.DrawText(screen, s.theme.FitText(hint, right-left), left, top, ui.ColorSubdued())

		return
	}
	name := s.theme.FitText(s.cueName(slot.cue), slot.play.Rect.Min.X-left-ui.Gap)
	s.theme.DrawStrong(screen, name, left, top+(ui.ButtonH-ui.LineHeight)/ui.Half+ui.Gap, ui.ColorInk())
	gain := "mix " + strconv.Itoa(int(math.Round(slot.gain*percent))) + " %"
	s.theme.DrawTextIn(screen, gain, image.Rect(slot.quieter.Rect.Max.X, slot.quieter.Rect.Min.Y,
		slot.louder.Rect.Min.X, slot.louder.Rect.Max.Y), ui.ColorInk())
	s.drawButtons(screen, slot.buttons()...)
	if slot.againToggle != nil {
		slot.againToggle.Draw(screen, s.theme)
	}
}
