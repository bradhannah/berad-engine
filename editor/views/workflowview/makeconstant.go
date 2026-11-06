package workflowview

import (
	"encoding/json"
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/valuepicker"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Make a constant, for input scripts: "ui:make-constant" beside a
// message's Insert value…, and in the list of other places typed the same
// "ui:other-<n>" to tick one, "ui:other-use" and "ui:other-skip".
const (
	makeTextID    = "make-constant"
	otherPrefix   = "other-"
	otherUseID    = "other-use"
	otherSkipID   = "other-skip"
	otherWidth    = 560
	otherRowGap   = 4
	gesturePrefix = "make-constant-"
)

// typedNumber is the number typed in a field a constant could stand for.
func typedNumber(node flow.Node, param string, option int) int {
	switch paramType(node, param) {
	case flow.TypeDuration:
		wait, _ := node.Duration(param) //nolint:errcheck // One that can't be read starts at nothing.

		return wait.Amount
	case flow.TypeShares:
		options, _ := node.Shares(param) //nolint:errcheck // As above.
		if option >= 0 && option < len(options) {
			return options[option].Share
		}
	case flow.TypeTiles, flow.TypeSeconds:
		amount, _ := node.Amount(param) //nolint:errcheck // As above.

		return amount.Number
	case flow.TypeWorkflow, flow.TypeInputs, flow.TypeKeeps, flow.TypeWays, flow.TypePort, flow.TypeAnchor,
		flow.TypeChoice, flow.TypeTimeRange, flow.TypeTimeRanges, flow.TypeText, flow.TypeCue, flow.TypeMessage,
		flow.TypeSignal, flow.TypeNPC, flow.TypeLanes, flow.TypeValue, flow.TypeOperand:
	}

	return 0
}

// newGesture names the edits one Make a constant makes, so one Undo takes
// back the constant and every place it went in.
func (s *State) newGesture() string { return gesturePrefix + s.gesture() }

// makesConstant reports whether a role's picker declares constants.
func makesConstant(role valueRole) bool { return role == roleNumber || role == roleMakeText }

// constantTargets keeps, for Make a constant, the places a constant can be
// declared: the world, an NPC template or a zone template, never a
// workflow's own or one NPC's.
func constantTargets(targets []declareTarget) []declareTarget {
	kept := targets[:0]
	for _, target := range targets {
		if target.Scope == worldstate.ScopeLocal || target.For != "" {
			continue
		}
		bare, _ := worldstate.Unprefixed(target.Path)
		target.Path = worldstate.ConstantPrefix + bare
		kept = append(kept, target)
	}

	return kept
}

// madeConstant is what happens once a constant is declared from a typed
// value: it stands in for the number, or replaces the words, and the other
// places typed the same are looked for.
func (s *State) madeConstant(ctx *frame.Context, node flow.Node, param string, made declaring, ref flow.ValueRef) {
	if made.text == nil {
		s.setNumberConstantIn(ctx, node, param, made.option, ref, made.gesture)
		s.lookForOthers(node, param, made.option, ref, made.target.Path, made.gesture, "")

		return
	}
	s.replaceWords(ctx, node, param, *made.text, ref, made.gesture)
	s.lookForOthers(node, param, -1, ref, made.target.Path, made.gesture, made.text.words)
}

// setNumberConstantIn puts a constant in for a typed-in number as part of a
// gesture.
func (s *State) setNumberConstantIn(ctx *frame.Context, node flow.Node, param string, option int, ref flow.ValueRef,
	gesture string,
) {
	changed, ok := withNumberConstant(node, param, option, ref)
	if !ok {
		return
	}
	s.command(ctx, workflowplugin.ReplaceEdit, gesture, workflowplugin.ReplaceArgs{Node: changed}, nil)
}

// withNumberConstant returns a node with a constant standing in for one of
// its numbers.
func withNumberConstant(node flow.Node, param string, option int, ref flow.ValueRef) (flow.Node, bool) {
	var value any
	switch paramType(node, param) {
	case flow.TypeDuration:
		wait, err := node.Duration(param)
		if err != nil {
			return node, false
		}
		wait.Constant = ref
		value = wait
	case flow.TypeShares:
		options, err := node.Shares(param)
		if err != nil || option < 0 || option >= len(options) {
			return node, false
		}
		options[option].Constant = ref
		value = options
	case flow.TypeTiles, flow.TypeSeconds:
		amount, err := node.Amount(param)
		if err != nil {
			return node, false
		}
		amount.Constant = ref
		value = amount
	case flow.TypeWorkflow, flow.TypeInputs, flow.TypeKeeps, flow.TypeWays, flow.TypePort, flow.TypeAnchor,
		flow.TypeChoice, flow.TypeTimeRange, flow.TypeTimeRanges, flow.TypeText, flow.TypeCue, flow.TypeMessage,
		flow.TypeSignal, flow.TypeNPC, flow.TypeLanes, flow.TypeValue, flow.TypeOperand:
		return node, false
	}
	changed, err := node.WithParam(param, value)

	return changed, err == nil
}

// selectedWords are words selected in a message's field: the text, and
// where it starts and ends in the field's text.
type selectedWords struct {
	words      string
	start, end int
	field      string
}

// selection returns the words selected in a message's field, when they are
// typed words, not a value's token or part of one.
func selection(made *control) (selectedWords, bool) {
	none := selectedWords{words: "", start: 0, end: 0, field: ""}
	buffer := made.field.Field.Buffer()
	if buffer == nil || !buffer.HasSelection() {
		return none, false
	}
	start, end := buffer.Selection()
	text := buffer.Text()
	words := text[start:end]
	before := text[:start]
	if strings.TrimSpace(words) == "" || strings.ContainsAny(words, tokenOpen+tokenClose) ||
		strings.Count(before, tokenOpen) != strings.Count(before, tokenClose) {
		return none, false
	}

	return selectedWords{words: words, start: start, end: end, field: text}, true
}

// messageControl returns the selected node's message control, if it has
// one.
func (s *State) messageControl() (*control, bool) {
	if s.form == nil {
		return nil, false
	}
	for index := range s.form.controls {
		if s.form.controls[index].param.Type == flow.TypeMessage {
			return &s.form.controls[index], true
		}
	}

	return nil, false
}

// updateMakeText opens the picker on text constants for the words selected
// in a message: one already made to put in their place, or a new one
// declared from them.
func (s *State) updateMakeText(input *ui.Input, ctx *frame.Context, node flow.Node) bool {
	made, has := s.messageControl()
	if !has || s.form.makeText == nil {
		return false
	}
	if words, selected := selection(made); selected {
		s.form.words = &words
	} else if made.field.Focused() {
		s.form.words = nil
	}
	if !s.form.makeText.Update(input) {
		return false
	}
	if s.form.words == nil {
		ctx.Frame.Status("Select typed words in the line first.")

		return true
	}
	words := *s.form.words
	source := &valueSource{
		state: s, node: node.ID, param: made.param.Name, role: roleMakeText, operand: false, targets: nil, option: -1,
		words: &words,
	}
	s.values = &valuePicker{
		picker: valuepicker.New(valuepicker.Config{
			Title: "Make a constant", For: "from “" + words.words + "”", Placeholder: "Name it, or find a text constant",
			Empty: "Type a name to declare it.", Only: "", Kinds: []worldstate.Type{worldstate.TypeText},
			Start: words.words, Rail: pickerRail(roleCheck), Selected: "", Query: "", LastGroup: &s.lastGroup,
			PickLabel: "Use", PickHint: "puts it in their place", DeclareAs: "Make constant",
			DeclareLead: "It takes the words' place at once.",
		}, source, s.theme),
		node: node.ID, param: made.param.Name, role: roleMakeText, operand: false,
	}

	return true
}

// replaceWords puts a constant in place of the words it was made from, in
// a message, as part of a gesture.
func (s *State) replaceWords(ctx *frame.Context, node flow.Node, param string, words selectedWords, ref flow.ValueRef,
	gesture string,
) {
	saved, err := node.Message()
	if err != nil {
		return
	}
	before, problem := s.parseMessage(ctx.Docs, words.field[:words.start], saved)
	after, laterProblem := s.parseMessage(ctx.Docs, words.field[words.end:], saved)
	if problem != "" || laterProblem != "" {
		ctx.Frame.Status(problem + laterProblem)

		return
	}
	segments := append(append(before, flow.Segment{Text: "", Ref: &ref}), after...)
	changed, err := node.WithParam(param, joinSegments(segments))
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	if made, has := s.messageControl(); has {
		made.field.Field.Revert()
	}
	s.command(ctx, workflowplugin.ReplaceEdit, gesture, workflowplugin.ReplaceArgs{Node: changed}, nil)
}

// joinSegments joins text segments side by side into one.
func joinSegments(segments []flow.Segment) []flow.Segment {
	joined := make([]flow.Segment, 0, len(segments))
	for _, segment := range segments {
		if last := len(joined) - 1; last >= 0 && segment.Ref == nil && joined[last].Ref == nil {
			joined[last].Text += segment.Text

			continue
		}
		joined = append(joined, segment)
	}

	return joined
}

// withWords returns a node's message with a constant in place of the first
// typed words that match, if any do.
func withWords(node flow.Node, param, words string, ref flow.ValueRef) (flow.Node, bool) {
	segments, err := node.Message()
	if err != nil {
		return node, false
	}
	for index, segment := range segments {
		before, after, found := strings.Cut(segment.Text, words)
		if segment.Ref != nil || !found {
			continue
		}
		replaced := slices3(segments[:index], []flow.Segment{
			{Text: before, Ref: nil}, {Text: "", Ref: &ref}, {Text: after, Ref: nil},
		}, segments[index+1:])
		changed, err := node.WithParam(param, dropEmpty(replaced))

		return changed, err == nil
	}

	return node, false
}

// slices3 joins three lists of segments.
func slices3(a, b, c []flow.Segment) []flow.Segment {
	return append(append(append([]flow.Segment{}, a...), b...), c...)
}

// dropEmpty leaves out empty text segments.
func dropEmpty(segments []flow.Segment) []flow.Segment {
	kept := segments[:0]
	for _, segment := range segments {
		if segment.Ref != nil || segment.Text != "" {
			kept = append(kept, segment)
		}
	}

	return joinSegments(kept)
}

// otherPlaces is the list of other places typed the same, offered once a
// constant is made: each with a box to tick, Use and Skip.
type otherPlaces struct {
	path, gesture string
	constant      flow.ValueRef
	kind, param   string
	option        int
	from          ids.ULID
	words         string
	number        int
	wait          flow.Duration
	scanned       bool
	rows          []otherPlace
	use, skip     *ui.Button
	rect          image.Rectangle
}

// otherPlace is one node elsewhere typed the same.
type otherPlace struct {
	workflow ids.ULID
	node     flow.Node
	tick     *ui.Button
}

// lookForOthers starts looking for the other places typed the same as what
// a constant was just made from: the same field of the same kind of node,
// with the same number, or a message with the same words.
func (s *State) lookForOthers(node flow.Node, param string, option int, ref flow.ValueRef, path, gesture,
	words string,
) {
	wait, _ := node.Duration(param) //nolint:errcheck // Only a Wait has one.
	s.others = &otherPlaces{
		path: path, gesture: gesture, constant: ref, kind: node.Kind, param: param, option: option, from: node.ID,
		words: words, number: typedNumber(node, param, option), wait: wait, scanned: false, rows: nil,
		use: ui.NewButton(otherUseID, "Use it"), skip: ui.NewButton(otherSkipID, "Skip"), rect: ui.Zero(),
	}
	s.others.use.Primary = true
}

// sameTyped reports whether a node elsewhere has what a constant was made
// from typed in, with no constant standing in.
func (o *otherPlaces) sameTyped(node flow.Node) bool {
	if node.ID == o.from {
		return false
	}
	if o.words != "" {
		_, matches := withWords(node, o.param, o.words, o.constant)

		return node.Kind == o.kind && matches
	}
	if node.Kind != o.kind || o.option >= 0 {
		return false
	}
	if wait, err := node.Duration(o.param); err == nil && paramType(node, o.param) == flow.TypeDuration {
		return wait.Constant.IsZero() && wait.Amount == o.number && wait.Unit == o.wait.Unit &&
			wait.Clock == o.wait.Clock
	}
	amount, err := node.Amount(o.param)

	return err == nil && amount.Constant.IsZero() && amount.Number == o.number
}

// scanOthers loads every workflow and lists the other places typed the
// same, once all are here; with none, the list closes unseen.
func (s *State) scanOthers(ctx *frame.Context) {
	others := s.others
	if others == nil || others.scanned {
		return
	}
	var workflows []project.Entry
	ctx.Docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			workflows = index.OfFormat(flow.FormatName)
		}
	})
	loaded := true
	var rows []otherPlace
	for _, entry := range workflows {
		s.subscribe(entry.ID, ctx)
		held := ctx.Docs.View(entry.ID, func(document client.Document) {
			if found, ok := document.Model.(*workflowplugin.Document); ok {
				for _, node := range found.Workflow.Nodes() {
					if others.sameTyped(node) {
						rows = append(rows, otherPlace{workflow: entry.ID, node: node, tick: nil})
					}
				}
			}
		})
		loaded = loaded && held
	}
	if !loaded {
		return
	}
	others.scanned = true
	if len(rows) == 0 {
		s.others = nil

		return
	}
	for index := range rows {
		rows[index].tick = ui.NewButton(otherPrefix+strconv.Itoa(index+1), s.otherLabel(ctx.Docs, rows[index]))
	}
	others.rows = rows
}

// otherLabel names a place typed the same: its workflow, its node, and what
// is typed there.
func (s *State) otherLabel(docs frame.Docs, place otherPlace) string {
	name := "this workflow"
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			name = index.Name(place.workflow, name)
		}
	})
	kind, _ := flow.KindOf(place.node.Kind)
	typed := strconv.Itoa(s.others.number)
	if s.others.words != "" {
		typed = "“" + s.others.words + "”"
	}

	return name + " › " + kind.Label + " · " + typed
}

// updateOthers ticks places, uses the constant in those ticked, or skips;
// it reports whether the list is open.
func (s *State) updateOthers(input *ui.Input, ctx *frame.Context) bool {
	others := s.others
	s.scanOthers(ctx)
	if s.others == nil || !others.scanned {
		return false
	}
	s.layoutOthers()
	if input.Consume(ui.KeyEscape, ui.Plain()) || others.skip.Update(input) {
		s.others = nil

		return true
	}
	ticked := 0
	for _, row := range others.rows {
		if row.tick.Update(input) {
			row.tick.On = !row.tick.On
		}
		if row.tick.On {
			ticked++
		}
	}
	others.use.Label, others.use.Disabled = "Use "+others.path+" in "+strconv.Itoa(ticked)+" more", ticked == 0
	if others.use.Update(input) {
		s.useInOthers(ctx)
		s.others = nil
	}

	return true
}

// useInOthers puts the constant in every place ticked, as part of the
// gesture that made it.
func (s *State) useInOthers(ctx *frame.Context) {
	others := s.others
	for _, row := range others.rows {
		if !row.tick.On {
			continue
		}
		changed, ok := withNumberConstant(row.node, others.param, -1, others.constant)
		if others.words != "" {
			changed, ok = withWords(row.node, others.param, others.words, others.constant)
		}
		if !ok {
			continue
		}
		data, err := json.Marshal(workflowplugin.ReplaceArgs{Node: changed})
		if err != nil {
			continue
		}
		ctx.Requests.Command(protocol.Command{
			Document: row.workflow, Edit: workflowplugin.ReplaceEdit, Gesture: others.gesture, Args: data,
		}, ctx.Frame.Report(""))
	}
}

// layoutOthers places the list in the middle of the window.
func (s *State) layoutOthers() {
	others := s.others
	rows := len(others.rows)
	height := ui.Half*ui.Inset + ui.Half*ui.LineHeight + rows*(ui.ButtonH+otherRowGap) + ui.Gap + ui.ButtonH
	width := min(otherWidth, s.size.X-ui.Half*ui.Inset)
	left, top := (s.size.X-width)/ui.Half, (s.size.Y-height)/ui.Half
	others.rect = image.Rect(left, top, left+width, top+height)
	inner := others.rect.Inset(ui.Inset)
	rowTop := inner.Min.Y + ui.Half*ui.LineHeight
	for _, row := range others.rows {
		row.tick.Rect = image.Rect(inner.Min.X, rowTop, inner.Max.X, rowTop+ui.ButtonH)
		rowTop += ui.ButtonH + otherRowGap
	}
	skipWidth := s.theme.TextWidth(others.skip.Label) + ui.Half*ui.Inset
	others.skip.Rect = image.Rect(inner.Max.X-skipWidth, inner.Max.Y-ui.ButtonH, inner.Max.X, inner.Max.Y)
	others.use.Rect = image.Rect(inner.Min.X, inner.Max.Y-ui.ButtonH, others.skip.Rect.Min.X-ui.Gap, inner.Max.Y)
}

// drawOthers draws the list over the window.
func (s *State) drawOthers(screen *ebiten.Image) {
	others := s.others
	if others == nil || !others.scanned {
		return
	}
	s.layoutOthers()
	ui.FillRect(screen, image.Rect(0, 0, s.size.X, s.size.Y), ui.ColorShade())
	ui.FillRound(screen, others.rect, ui.Radius, ui.ColorBackground())
	ui.StrokeRound(screen, others.rect, ui.Radius, ui.Border, ui.ColorRule())
	inner := others.rect.Inset(ui.Inset)
	what := strconv.Itoa(others.number)
	if others.words != "" {
		what = "“" + others.words + "”"
	}
	places := strconv.Itoa(len(others.rows)) + " other places have "
	if len(others.rows) == 1 {
		places = "1 other place has "
	}
	s.theme.DrawStrong(screen, s.theme.FitText(places+what+" typed in. Tick the ones that mean "+others.path+":",
		inner.Dx()), inner.Min.X, inner.Min.Y, ui.ColorInk())
	for _, row := range others.rows {
		row.tick.Draw(screen, s.theme)
	}
	others.use.Draw(screen, s.theme)
	others.skip.Draw(screen, s.theme)
}

// locateOthers finds a part of the list for input scripts.
func (s *State) locateOthers(widget string) (image.Point, bool) {
	if s.others == nil || !s.others.scanned {
		return image.Pt(0, 0), false
	}
	s.layoutOthers()
	buttons := []*ui.Button{s.others.use, s.others.skip}
	for _, row := range s.others.rows {
		buttons = append(buttons, row.tick)
	}
	for _, button := range buttons {
		if button.ID == widget {
			return ui.Center(button.Rect), true
		}
	}

	return image.Pt(0, 0), false
}

// hasOthersState answers "others=<n>", how many other places the open list
// offers.
func (s *State) hasOthersState(state string) (bool, bool) {
	count, asked := strings.CutPrefix(state, "others=")
	if !asked {
		return false, false
	}
	if s.others == nil || !s.others.scanned {
		return count == "0", true
	}

	return strconv.Itoa(len(s.others.rows)) == count, true
}
