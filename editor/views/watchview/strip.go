package watchview

import (
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/clock"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// One strip under the workflow (increment 46): the Log, the watched NPC's
// Trail and its Values share it, one at a time, chosen by the tabs at its
// left. The Log shows first, as it did before; the workflow takes the
// window's width, where the values stood beside it.
//
// The trail is each node the watched NPC passed, newest at the foot: the
// game time, the node it left and the one it reached, and what the node
// read or did that sent it that way, or why it started again.

// stripPart is the part of the strip shown.
type stripPart string

// The strip's parts.
const (
	partLog    stripPart = "log"
	partTrail  stripPart = "trail"
	partValues stripPart = "values"
)

// The trail: how many changes Watch keeps, its states and its messages.
const (
	MaxTrail         = 500
	trailLinesState  = "trail-lines="
	trailState       = "trail:"
	stripState       = "strip="
	firstValueState  = "first-value="
	textNoTrail      = "Each node the watched NPC passes shows here, with what sent it on."
	textTrailNoNPC   = "Click an NPC on the map to see each node it passes."
	textNoValues     = "The watched NPC's values show here."
	trailPathShare   = 2
	trailWholeShare  = 5
	trailSeparator   = " · "
	trailOtherFlow   = "a node in another workflow"
	stripTabIDPrefix = "strip-"
)

// strip is the strip's tabs, the part chosen, and the watched NPC's trail:
// the changes kept, the number of the latest, whose they are, how far up
// from the newest it is scrolled, and the rows drawn.
type strip struct {
	part  stripPart
	tabs  []*ui.Button
	trail []protocol.PreviewTrail
	seen  uint64
	whose ids.ULID
	back  int
	shown []trailRow
	// firstValue is the first value shown, as the values scroll.
	firstValue int
}

// trailRow is one change drawn, and where.
type trailRow struct {
	entry protocol.PreviewTrail
	top   int
}

func newStrip() *strip {
	return &strip{
		part: partLog, tabs: []*ui.Button{
			ui.NewButton(stripTabIDPrefix+string(partLog), "Log"),
			ui.NewButton(stripTabIDPrefix+string(partTrail), "Trail"),
			ui.NewButton(stripTabIDPrefix+string(partValues), "Values"),
		},
		trail: nil, seen: 0, whose: ids.ULID{}, back: 0, shown: nil, firstValue: 0,
	}
}

// restart empties the trail, as a preview starts.
func (t *strip) restart() {
	t.trail, t.seen, t.whose, t.back = nil, 0, ids.ULID{}, 0
}

// keepTrail adds the changes of a preview's state not kept yet, starting
// afresh when another NPC is watched, and keeps the latest MaxTrail.
func (t *strip) keepTrail(state protocol.PreviewState) {
	if state.Watched != t.whose {
		t.restart()
		t.whose = state.Watched
	}
	for _, entry := range state.Trail {
		if entry.Seq > t.seen {
			t.trail, t.seen = append(t.trail, entry), entry.Seq
		}
	}
	if over := len(t.trail) - MaxTrail; over > 0 {
		t.trail = slices.Delete(t.trail, 0, over)
	}
}

// stripLeft is where the chosen part's own buttons start, after the tabs.
func (s *State) stripLeft() int {
	last := s.strip.tabs[len(s.strip.tabs)-1]

	return last.Rect.Max.X + ui.Inset
}

// layoutStrip places the tabs at the strip's left, and the trail's rows,
// the newest at the foot.
func (s *State) layoutStrip(docs frame.Docs) {
	area := s.logArea()
	left, top := ui.Inset, area.Min.Y+ui.Gap
	for _, tab := range s.strip.tabs {
		width := s.theme.TextWidth(tab.Label) + ui.Half*ui.Inset
		tab.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
		tab.On = tab.ID == stripTabIDPrefix+string(s.strip.part)
		left += width
	}
	s.layoutLog(docs)
	s.strip.shown = s.strip.shown[:0]
	if s.strip.part != partTrail {
		return
	}
	trail := s.strip.trail
	s.strip.back = max(0, min(s.strip.back, len(trail)-logRows))
	end := len(trail) - s.strip.back
	shown := trail[max(0, end-logRows):end]
	rowTop := area.Max.Y - ui.Gap - len(shown)*ui.LineHeight
	for index, entry := range shown {
		s.strip.shown = append(s.strip.shown, trailRow{entry: entry, top: rowTop + index*ui.LineHeight})
	}
}

// updateStrip chooses a part by its tab, scrolls the trail, and runs the
// log.
func (s *State) updateStrip(input *ui.Input, ctx *frame.Context) {
	defer s.updateLog(input, ctx)
	s.layoutStrip(ctx.Docs)
	for _, tab := range s.strip.tabs {
		if tab.Update(input) {
			s.strip.part = stripPart(strings.TrimPrefix(tab.ID, stripTabIDPrefix))
			s.layoutStrip(ctx.Docs)
		}
	}
	if s.strip.part == partLog || !input.Cursor.In(s.logArea()) || input.WheelY == 0 {
		return
	}
	step := 1
	if input.WheelY < 0 {
		step = -1
	}
	if s.strip.part == partTrail {
		s.strip.back += step
	} else {
		s.strip.firstValue -= step
	}
	input.WheelY = 0
}

// drawStrip draws the strip's rule and tabs, and the part chosen.
func (s *State) drawStrip(screen *ebiten.Image, docs frame.Docs) {
	area := s.logArea()
	ui.Divider(screen, area.Min, image.Pt(area.Max.X, area.Min.Y))
	for _, tab := range s.strip.tabs {
		tab.Draw(screen, s.theme)
	}
	switch s.strip.part {
	case partLog:
		s.drawLog(screen, docs)
	case partTrail:
		s.drawTrail(screen, docs)
	case partValues:
		s.drawValues(screen)
	}
}

// drawTrail draws the trail's rows: each with its game time, the node left
// and the one reached, and what sent the NPC on.
func (s *State) drawTrail(screen *ebiten.Image, docs frame.Docs) {
	area := s.logArea()
	if len(s.strip.shown) == 0 {
		empty := textNoTrail
		if _, watching := s.watched(); !watching {
			empty = textTrailNoNPC
		}
		s.theme.DrawText(screen, empty, ui.Inset, area.Min.Y+logHeaderSpace, ui.ColorMuted())

		return
	}
	timeWidth := s.theme.MonoWidth("00:00")
	left := ui.Inset + timeWidth + ui.Inset
	room := area.Max.X - ui.Inset - left
	for _, row := range s.strip.shown {
		s.theme.DrawMono(screen, clock.Format(row.entry.Clock), ui.Inset, row.top, ui.ColorMuted())
		way := s.theme.FitText(s.trailWay(docs, row.entry.Change), room*trailPathShare/trailWholeShare)
		s.theme.DrawText(screen, way, left, row.top, ui.ColorInk())
		if row.entry.Change.Why == "" {
			continue
		}
		whyLeft := left + s.theme.TextWidth(way) + ui.Inset
		why := s.theme.FitText(row.entry.Change.Why, area.Max.X-ui.Inset-whyLeft)
		s.theme.DrawText(screen, why, whyLeft, row.top, ui.ColorMuted())
	}
}

// trailWay writes a change as the node left and the node reached, as
// "Time switch: Morning → Wait · 30 min", naming them in the workflow shown;
// a restart reached the start from no node.
func (s *State) trailWay(docs frame.Docs, change protocol.PreviewChange) string {
	if s.flow == nil {
		return trailOtherFlow
	}
	parts := s.flow.Path([]protocol.PreviewChange{change}, docs)
	switch {
	case len(parts) == 0:
		return trailOtherFlow
	case len(parts) == 1 && change.From.IsZero():
		return pathSeparator[1:] + parts[0]
	case len(parts) == 1:
		return parts[0] + pathSeparator + trailOtherFlow
	}

	return parts[0] + pathSeparator + parts[1]
}

// drawValues lists the watched NPC's values in as many columns as fit: its
// workflow's locals and its own, its zone's and the world's. When they do
// not all fit, the wheel scrolls them a row at a time, and the strip's
// header says which show.
func (s *State) drawValues(screen *ebiten.Image) {
	area := s.logArea()
	rows := s.valueRows()
	if len(rows) == 0 {
		s.theme.DrawText(screen, textNoValues, ui.Inset, area.Min.Y+logHeaderSpace, ui.ColorMuted())

		return
	}
	top := area.Min.Y + logHeaderSpace
	perColumn := max(1, (area.Max.Y-ui.Gap-top)/ui.LineHeight)
	columns := max(1, (area.Dx()-ui.Inset)/(valuesWidth+ui.Inset))
	room := perColumn * columns
	s.strip.firstValue = max(0, min(s.strip.firstValue, len(rows)-room))
	shown := rows[s.strip.firstValue:min(len(rows), s.strip.firstValue+room)]
	for index, row := range shown {
		left := ui.Inset + index/perColumn*(valuesWidth+ui.Inset)
		y := top + index%perColumn*ui.LineHeight
		s.theme.DrawMono(screen, row.path, left, y, ui.ColorInk())
		s.theme.DrawText(screen, row.value, left+s.valuesRoom()-s.theme.TextWidth(row.value), y, ui.ColorLive())
	}
	if len(shown) < len(rows) {
		which := strconv.Itoa(s.strip.firstValue+1) + "–" + strconv.Itoa(s.strip.firstValue+len(shown)) + " of " +
			strconv.Itoa(len(rows)) + " · scroll for more"
		tab := s.strip.tabs[0].Rect
		s.theme.DrawText(screen, which, s.stripLeft(), tab.Min.Y+(ui.ButtonH-ui.TextSize)/ui.Half-1, ui.ColorMuted())
	}
}

// hasStripState answers "strip=<part>", the part shown; "first-value=<n>",
// the first value shown, from 1;
// "trail-lines=<count>", how many changes the trail holds; and
// "trail:<words>", whether a change on it says the words.
func (s *State) hasStripState(state string) (bool, bool) {
	switch {
	case strings.HasPrefix(state, stripState):
		return string(s.strip.part) == strings.TrimPrefix(state, stripState), true
	case strings.HasPrefix(state, firstValueState):
		return strconv.Itoa(s.strip.firstValue+1) == strings.TrimPrefix(state, firstValueState), true
	case strings.HasPrefix(state, trailLinesState):
		return strconv.Itoa(len(s.strip.trail)) == strings.TrimPrefix(state, trailLinesState), true
	case strings.HasPrefix(state, trailState):
		words := strings.TrimPrefix(state, trailState)

		return slices.ContainsFunc(s.strip.trail, func(entry protocol.PreviewTrail) bool {
			return strings.Contains(entry.Change.Why, words)
		}), true
	}

	return false, false
}
