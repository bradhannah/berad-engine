package workflowview

import (
	"image"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/places"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npcrun"
	"github.com/bradhannah/berad-engine/engine/show"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// A Log's Message: its Insert value… button, how a value stands in its
// text, and how a value whose NPC is only known as it runs reads.
const (
	insertValueID = "insert-value"
	tokenOpen     = "{"
	tokenClose    = "}"
	chipPad       = 4
	chipGap       = 3
)

// typeForms lists how each type reads as text, under a Log's Message.
func typeForms() [][2]string {
	return [][2]string{
		{"zone, workflow, sprite", "its name"},
		{worldstate.TypePlace.Label(), "anchor, floor, zone"},
		{"NPC", "its name value"},
		{"integer, bool", "12, true"},
		{"choice", "the choice's name"},
		{"text", "as it is, without quotes"},
	}
}

// tokenOf is how a value stands in a message's text: its path in braces, or
// its ID for one deleted, which the node's problem names.
func (s *State) tokenOf(docs frame.Docs, ref flow.ValueRef) string {
	if _, path, problem := s.resolve(docs, ref); problem == "" {
		return tokenOpen + path + tokenClose
	}

	return tokenOpen + "#" + ref.Value.Short(shortID) + tokenClose
}

// messageText is a message as its field shows it: text as it is, each
// value as its token.
func (s *State) messageText(docs frame.Docs, segments []flow.Segment) string {
	var text strings.Builder
	for _, segment := range segments {
		if segment.Ref == nil {
			text.WriteString(segment.Text)

			continue
		}
		text.WriteString(s.tokenOf(docs, *segment.Ref))
	}

	return text.String()
}

// applyMessageField reads a Log's Message field into its node, or says what
// is wrong with it, reporting whether the field is a message's.
func (s *State) applyMessageField(docs frame.Docs, node flow.Node, field *ui.TextField) (flow.Node, string, bool) {
	for index := range s.form.controls {
		made := &s.form.controls[index]
		if made.param.Type != flow.TypeMessage || field != made.field {
			continue
		}
		saved, err := node.Message()
		if err != nil {
			return node, err.Error(), true
		}
		segments, problem := s.parseMessage(docs, field.Field.Value(), saved)
		if problem != "" {
			return node, problem, true
		}
		changed, err := node.WithParam(made.param.Name, segments)
		if err != nil {
			return node, err.Error(), true
		}

		return changed, "", true
	}

	return node, "", false
}

// parseMessage reads a message's text: each token a value the workflow can
// read, by its path, or one the message already holds; the rest text, as
// typed, so a space before a value inserted later stays.
func (s *State) parseMessage(docs frame.Docs, value string, saved []flow.Segment) ([]flow.Segment, string) {
	known := map[string]flow.ValueRef{}
	for _, option := range s.valueOptions(docs, roleGive) {
		known[option.path] = option.ref
	}
	for _, segment := range saved {
		if segment.Ref != nil {
			known[strings.Trim(s.tokenOf(docs, *segment.Ref), tokenOpen+tokenClose)] = *segment.Ref
		}
	}
	var segments []flow.Segment
	addText := func(text string) {
		if text == "" {
			return
		}
		if last := len(segments) - 1; last >= 0 && segments[last].Ref == nil {
			segments[last].Text += text

			return
		}
		segments = append(segments, flow.Segment{Text: text, Ref: nil})
	}
	rest := value
	for rest != "" {
		before, after, opened := strings.Cut(rest, tokenOpen)
		token, next, closed := strings.Cut(after, tokenClose)
		if !opened || !closed {
			addText(rest)

			break
		}
		addText(before)
		ref, found := known[strings.TrimSpace(token)]
		if !found {
			// A path typed as before prefixes, as world.coins, is a value's.
			ref, found = known[worldstate.ValuePrefix+strings.TrimSpace(token)]
		}
		if !found {
			return nil, "No value is called " + strings.TrimSpace(token) + ". Put one in with Insert value…"
		}
		segments = append(segments, flow.Segment{Text: "", Ref: &ref})
		rest = next
	}

	return segments, messageLimits(segments)
}

// messageLimits says what is wrong with a message too long to keep, or "".
func messageLimits(segments []flow.Segment) string {
	text := 0
	for _, segment := range segments {
		text += utf8.RuneCountInString(segment.Text)
	}
	switch {
	case len(segments) > flow.MaxSegments:
		return "A message has at most " + strconv.Itoa(flow.MaxSegments) + " parts, its values and the text between them."
	case text > flow.MaxMessageText:
		return "A message's text is at most " + strconv.Itoa(flow.MaxMessageText) + " characters, besides its values."
	}

	return ""
}

// insertForLog puts a value picked for a Log into its message where the
// caret was, and saves it, reporting whether the node is a Log.
func (s *State) insertForLog(ctx *frame.Context, node flow.Node, ref flow.ValueRef) bool {
	if !hasMessage(node.Kind) || s.form == nil || s.form.node != node.ID {
		return hasMessage(node.Kind)
	}
	for index := range s.form.controls {
		made := &s.form.controls[index]
		if made.param.Type != flow.TypeMessage || made.field == nil {
			continue
		}
		made.field.Field.Focus()
		made.field.Field.Type(s.tokenOf(ctx.Docs, ref))
		s.Save(ctx)
	}

	return true
}

// logProblem says what is wrong with a value a Log writes: one deleted,
// one an NPC following it lacks, or its own NPC's in a zone loop.
func (s *State) logProblem(node flow.Node, docs frame.Docs) string {
	segments, err := node.Message()
	if err != nil {
		return err.Error()
	}
	for _, segment := range segments {
		if segment.Ref == nil {
			continue
		}
		ref := *segment.Ref
		_, path, problem := s.resolve(docs, ref)
		if problem != "" {
			return problem
		}
		if s.workflow.Purpose() == flow.PurposeZoneLoop && ref.Scope == worldstate.ScopeNPC && !ref.Others() {
			return problemLoopNPC
		}
		if missing := s.missing(docs, ref, path); missing != "" {
			return missing
		}
	}

	return ""
}

// problemLoopNPC is a zone loop reading an NPC's own value.
const problemLoopNPC = "A zone loop has no NPC of its own. Write a named NPC's value instead."

// viewNamer names what values refer to in the editor: documents and
// places by the project's names, and an NPC by its name value as it starts.
type viewNamer struct {
	places.Names

	catalog *worldstate.Catalog
}

func (n viewNamer) NPC(npcID ids.ULID) string {
	if name, ok := n.catalog.Lookup(worldstate.ScopeNPC, npcID, worldstate.NameID()); ok {
		return name.Zero().Text
	}

	return n.Document(npcID)
}

// namer names what values refer to, for their text forms in the editor.
func (s *State) namer(docs frame.Docs) viewNamer {
	return viewNamer{Names: places.NewNames(docs), catalog: s.catalog(docs)}
}

// sampleOwner is the NPC whose values a Log's line reads as it would for:
// the first NPC following the workflow, or none.
func (s *State) sampleOwner(docs frame.Docs, catalog *worldstate.Catalog) (ids.ULID, bool) {
	if s.workflow.Purpose() == flow.PurposeZoneLoop {
		return ids.ULID{}, false
	}
	followers := s.followers(docs, catalog)
	if len(followers) == 0 {
		return ids.ULID{}, false
	}

	return followers[0], true
}

// sampleLine is how a Log's line would read as its values start, for the
// first NPC following the workflow; a value known only as it runs, as the
// NPC a local holds, stands as its path.
func (s *State) sampleLine(docs frame.Docs, node flow.Node) string {
	segments, err := node.Message()
	if err != nil {
		return ""
	}
	catalog := s.catalog(docs)
	namer := s.namer(docs)
	sample, hasSample := s.sampleOwner(docs, catalog)
	var line strings.Builder
	for _, segment := range segments {
		if segment.Ref == nil {
			line.WriteString(segment.Text)

			continue
		}
		ref := *segment.Ref
		declared, path, problem := s.resolve(docs, ref)
		owner, startsKnown := sample, hasSample
		switch {
		case problem != "":
			line.WriteString(npcrun.Unreadable)

			continue
		case !ref.Found.IsZero():
			startsKnown = false
		case !ref.Owner.IsZero():
			owner, startsKnown = ref.Owner, true
		case ref.Scope == worldstate.ScopeZone:
			owner, startsKnown = s.workflow.RunsIn(), true
		case ref.Scope != worldstate.ScopeNPC:
			owner, startsKnown = ids.ULID{}, true
		}
		if !startsKnown {
			line.WriteString(show.Escape("‹" + path + "›"))

			continue
		}
		declared, value := constantValue(catalog, ref.Scope, owner, declared)
		said := worldstate.Text(declared, value, namer)
		if !declared.IsConstant() {
			said = show.Escape(said)
		}
		line.WriteString(said)
	}

	return show.Plain(line.String())
}

// sampleHeading heads the line as it would read, naming whose values it
// reads.
func (s *State) sampleHeading(docs frame.Docs) string {
	if sample, ok := s.sampleOwner(docs, s.catalog(docs)); ok {
		return "As it would read for " + s.namer(docs).NPC(sample)
	}

	return "As it would read"
}

// layoutMessage places a Log's Message field and Insert value…, leaving
// room under them for the line as it would read and each type as text.
func (s *State) layoutMessage(docs frame.Docs, made *control, left, width, top int) int {
	made.field.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
	top += ui.ButtonH + rowSpace
	made.pick.Rect = image.Rect(left, top, left+s.theme.TextWidth(made.pick.Label)+ui.Inset*ui.Half, top+ui.ButtonH)
	if s.form != nil && s.form.makeText != nil {
		from := made.pick.Rect.Max.X + ui.Gap
		s.form.makeText.Rect = image.Rect(from, top, from+s.theme.TextWidth(s.form.makeText.Label)+ui.Inset*ui.Half,
			top+ui.ButtonH)
	}
	top += ui.ButtonH + fieldSpace
	if node, ok := s.selectedNode(); ok {
		top += (1 + len(s.wrap(s.sampleLine(docs, node), width))) * ui.LineHeight
	}

	return top + fieldSpace + (1+len(typeForms()))*ui.LineHeight
}

// drawMessage draws a Log's Message, Insert value…, the line as it would
// read and each type as text.
func (s *State) drawMessage(screen *ebiten.Image, docs frame.Docs, made *control, node flow.Node, left int) {
	made.field.Draw(screen)
	made.pick.Draw(screen, s.theme)
	if s.form != nil && s.form.makeText != nil {
		s.form.makeText.Draw(screen, s.theme)
	}
	width := made.field.Rect.Dx()
	top := made.pick.Rect.Max.Y + fieldSpace
	s.theme.DrawStrong(screen, s.sampleHeading(docs), left, top, ui.ColorInk())
	for _, line := range s.wrap(s.sampleLine(docs, node), width) {
		top += ui.LineHeight
		s.theme.DrawText(screen, line, left, top, ui.ColorInk())
	}
	top += ui.LineHeight + fieldSpace
	s.theme.DrawStrong(screen, "Each type as text", left, top, ui.ColorInk())
	column := left
	for _, form := range typeForms() {
		column = max(column, left+s.theme.SmallWidth(form[0])+ui.Inset)
	}
	for _, form := range typeForms() {
		top += ui.LineHeight
		s.theme.DrawSmall(screen, form[0], left, top, ui.ColorMuted())
		s.theme.DrawSmall(screen, form[1], column, top, ui.ColorInk())
	}
}

// drawLogNode writes LOG and a Log's message on its node: its text, and
// each value as a chip holding its path, cut short at the node's edge.
func (s *State) drawLogNode(area *ebiten.Image, drawn shape, docs frame.Docs) {
	rect := s.screenRect(drawn.rect)
	s.theme.DrawLabel(area, labelOf(drawn.node.Kind), rect.Min.X+textPad, rect.Min.Y+labelDrop, ui.ColorMuted())
	segments, err := drawn.node.Message()
	if err != nil || len(segments) == 0 {
		s.theme.DrawText(area, "no message", rect.Min.X+textPad, rect.Min.Y+titleDrop, ui.ColorMuted())

		return
	}
	along, right, baseline := rect.Min.X+textPad, rect.Max.X-textPad, rect.Min.Y+titleDrop
	for _, segment := range segments {
		room := right - along
		if room <= 0 {
			return
		}
		if segment.Ref == nil {
			text := s.clip(segment.Text, room)
			s.theme.DrawText(area, text, along, baseline, ui.ColorInk())
			along += s.theme.TextWidth(text)

			continue
		}
		path := strings.Trim(s.tokenOf(docs, *segment.Ref), tokenOpen+tokenClose)
		chipW := min(s.theme.SmallWidth(path)+ui.Half*chipPad, room)
		chip := image.Rect(along+1, baseline-chipPad/ui.Half, along+1+chipW, baseline+ui.TextSize+chipPad/ui.Half)
		ui.FillRound(area, chip, chipPad, ui.ColorHover())
		ui.StrokeRound(area, chip, chipPad, 1, ui.ColorRule())
		s.theme.DrawSmall(area, s.clip(path, chipW-chipPad), chip.Min.X+chipPad, baseline+1, ui.ColorInk())
		along = chip.Max.X + chipGap
	}
}

// hasMessage reports whether a kind of node has a message of text and
// values: a Log, a Say or a Show message.
func hasMessage(kind string) bool {
	declared, ok := flow.KindOf(kind)

	return ok && slices.ContainsFunc(declared.Params, func(param flow.Param) bool {
		return param.Type == flow.TypeMessage
	})
}

// insertButton is Insert value… for a message, and none for plain text.
func insertButton(paramType flow.ParamType) *ui.Button {
	if paramType != flow.TypeMessage {
		return nil
	}

	return ui.NewButton(insertValueID, "Insert value…")
}

// hasLogState answers "message=<text>", the selected Log's message as its
// field shows it, and "reads=<line>", how its line would read.
func (s *State) hasLogState(state string, docs frame.Docs) (bool, bool) {
	node, selected := s.selectedNode()
	switch {
	case strings.HasPrefix(state, "message="):
		return selected && s.logSummary(docs, node) == strings.TrimPrefix(state, "message="), true
	case strings.HasPrefix(state, "reads="):
		return selected && s.sampleLine(docs, node) == strings.TrimPrefix(state, "reads="), true
	}

	return false, false
}

// logSummary is a Log's message as its field shows it.
func (s *State) logSummary(docs frame.Docs, node flow.Node) string {
	segments, err := node.Message()
	if err != nil {
		return "?"
	}

	return s.messageText(docs, segments)
}

// shakeSummary is a Shake screen's strength and seconds, as strong · 2 s.
func shakeSummary(node flow.Node) string {
	strength, err := node.Choice(flow.ParamStrength)
	seconds, secondsErr := node.Seconds()
	if err != nil || secondsErr != nil {
		return "?"
	}

	return strength + " · " + strconv.Itoa(seconds) + " s"
}
