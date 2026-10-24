package workflowview

import (
	"fmt"
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// Inspector layout, in screen pixels.
const (
	formTop      = ui.Inset + ui.LineHeight + ui.TitleSize + ui.Inset
	fieldSpace   = 14
	rowSpace     = 6
	noteField    = 120
	timeWidth    = 64
	dashWidth    = 16
	removeWidth  = 76
	shareWidth   = 48
	maxTextField = 200
	hoursPerDay  = 24
	minuteDigits = 2
	clockDigits  = 4
)

// rangeRow is one time of a time switch: its fields and its Remove button.
type rangeRow struct {
	port   string
	from   *ui.TextField
	to     *ui.TextField
	remove *ui.Button
}

// optionRow is one option of a random choice: its name, its share and its
// Remove button.
type optionRow struct {
	port   string
	label  *ui.TextField
	share  *ui.TextField
	remove *ui.Button
}

// control edits one parameter, built from its declared type.
type control struct {
	param flow.Param
	label int
	// field edits text, tiles or a wait's amount; from and to a time range;
	// rows a list of times; pick and clear an anchor; choices a choice;
	// units and clocks a wait's unit and clock; options a random choice's
	// options.
	field     *ui.TextField
	from, to  *ui.TextField
	rows      []rangeRow
	addRange  *ui.Button
	pick      *ui.Button
	clear     *ui.Button
	anchor    image.Rectangle
	choices   []*ui.Button
	units     []*ui.Button
	clocks    []*ui.Button
	options   []optionRow
	addOption *ui.Button
}

// form is the inspector for the selected node. rows counts a time switch's
// times or a random choice's options, so the form is rebuilt when they
// change.
type form struct {
	node     ids.ULID
	kind     flow.Kind
	rows     int
	controls []control
	remove   *ui.Button
}

// syncForm keeps the inspector built for the selected node and showing its
// saved values.
func (s *State) syncForm(ctx *frame.Context) {
	node, ok := s.selectedNode()
	if !ok {
		if s.form != nil {
			s.form = nil
			s.focus = ui.Focus{}
		}

		return
	}
	if s.form == nil || s.form.node != node.ID || s.form.rows != rowCount(node) {
		s.form = s.buildForm(node)
	}
	for index := range s.form.controls {
		s.setSaved(&s.form.controls[index], node, ctx)
	}
}

func (s *State) selectedNode() (flow.Node, bool) {
	if s.workflow == nil {
		var none flow.Node

		return none, false
	}

	return s.workflow.Node(s.selected)
}

// rowCount counts a time switch's times or a random choice's options.
func rowCount(node flow.Node) int {
	switch node.Kind {
	case flow.KindTimeSwitch:
		ranges, err := node.TimeRanges("ranges")
		if err != nil {
			return 0
		}

		return len(ranges)
	case flow.KindRandom:
		options, err := node.Shares("options")
		if err != nil {
			return 0
		}

		return len(options)
	default:
		return 0
	}
}

// buildForm makes a control for each of a node's parameters.
func (s *State) buildForm(node flow.Node) *form {
	kind, _ := flow.KindOf(node.Kind)
	built := &form{node: node.ID, kind: kind, rows: rowCount(node), controls: nil, remove: nil}
	s.focus = ui.Focus{}
	if node.Kind != flow.KindStart {
		built.remove = ui.NewButton("delete-node", "Delete node")
	}
	text := func(id string, multiline bool) *ui.TextField {
		field := ui.NewTextField(id, textedit.NewField("", multiline, maxTextField), multiline, s.theme)
		s.focus.Register(field)

		return field
	}
	for _, param := range kind.Params {
		made := control{param: param, label: 0, field: nil, from: nil, to: nil, rows: nil, addRange: nil, pick: nil,
			clear: nil, anchor: ui.Zero(), choices: nil, units: nil, clocks: nil, options: nil, addOption: nil}
		switch param.Type {
		case flow.TypeTiles:
			made.field = text("param-"+param.Name, false)
		case flow.TypeChoice:
			for _, choice := range param.Choices {
				made.choices = append(made.choices, ui.NewButton(param.Name+"-"+choice.Value, choice.Label))
			}
		case flow.TypeDuration:
			made.field = text("param-"+param.Name, false)
			made.units, made.clocks = durationButtons(param.Name)
		case flow.TypeShares:
			for index := range built.rows {
				prefix := param.Name + "-" + strconv.Itoa(index+1)
				made.options = append(made.options, optionRow{
					port: "", label: text(prefix+"-label", false), share: text(prefix+"-share", false),
					remove: ui.NewButton(prefix+"-remove", "Remove"),
				})
			}
			made.addOption = ui.NewButton("add-option", "Add option")
		case flow.TypeText:
			made.field = text("param-"+param.Name, node.Kind == flow.KindNote)
		case flow.TypeTimeRange:
			made.from, made.to = text(param.Name+"-from", false), text(param.Name+"-to", false)
		case flow.TypeTimeRanges:
			for index := range built.rows {
				prefix := param.Name + "-" + strconv.Itoa(index+1)
				made.rows = append(made.rows, rangeRow{
					port: "", from: text(prefix+"-from", false), to: text(prefix+"-to", false),
					remove: ui.NewButton(prefix+"-remove", "Remove"),
				})
			}
			made.addRange = ui.NewButton("add-time", "Add time")
		case flow.TypeAnchor:
			made.pick = ui.NewButton("pick", "Pick on map")
			made.pick.Primary = true
			made.clear = ui.NewButton("clear", "Clear")
		}
		built.controls = append(built.controls, made)
	}

	return built
}

// setSaved shows a parameter's saved value in its fields.
func (s *State) setSaved(made *control, node flow.Node, ctx *frame.Context) {
	switch made.param.Type {
	case flow.TypeTiles:
		if number, err := node.Number(made.param.Name); err == nil {
			made.field.Field.SetSaved(strconv.Itoa(number))
		}
	case flow.TypeChoice, flow.TypeDuration, flow.TypeShares:
		setSavedChoices(made, node)
	case flow.TypeText:
		if value, err := node.Text(made.param.Name); err == nil {
			made.field.Field.SetSaved(value)
		}
	case flow.TypeTimeRange:
		if span, err := node.TimeRange(made.param.Name); err == nil {
			made.from.Field.SetSaved(flow.FormatTime(span.From))
			made.to.Field.SetSaved(flow.FormatTime(span.To))
		}
	case flow.TypeTimeRanges:
		spans, err := node.TimeRanges(made.param.Name)
		if err != nil || len(spans) != len(made.rows) {
			return
		}
		for index, span := range spans {
			made.rows[index].port = span.Port
			made.rows[index].from.Field.SetSaved(flow.FormatTime(span.From))
			made.rows[index].to.Field.SetSaved(flow.FormatTime(span.To))
		}
	case flow.TypeAnchor:
		if destination, err := node.Anchor(made.param.Name); err == nil && !destination.IsZero() {
			s.subscribe(destination.Floor, ctx)
		}
	}
}

// layoutForm places the inspector's fields and buttons.
func (s *State) layoutForm(frame.Docs) {
	if s.form == nil {
		return
	}
	panel := frame.Inspector(s.size)
	left, width := panel.Min.X+ui.Inset, panel.Dx()-ui.Half*ui.Inset
	top := panel.Min.Y + formTop
	for index := range s.form.controls {
		made := &s.form.controls[index]
		made.label = top
		top += ui.LineHeight
		switch made.param.Type {
		case flow.TypeChoice, flow.TypeDuration, flow.TypeShares:
			top = layoutChoices(made, left, width, top)
		case flow.TypeTiles, flow.TypeText:
			height := ui.ButtonH
			if made.field.Multiline {
				height = noteField
			}
			made.field.Rect = image.Rect(left, top, left+width, top+height)
			top += height + fieldSpace
		case flow.TypeTimeRange:
			placeTimes(made.from, made.to, left, top)
			top += ui.ButtonH + fieldSpace
		case flow.TypeTimeRanges:
			for _, row := range made.rows {
				placeTimes(row.from, row.to, left, top)
				row.remove.Rect = image.Rect(left+width-removeWidth, top, left+width, top+ui.ButtonH)
				top += ui.ButtonH + rowSpace
			}
			made.addRange.Rect = image.Rect(left, top, left+width/ui.Half, top+ui.ButtonH)
			top += ui.ButtonH + fieldSpace
		case flow.TypeAnchor:
			made.anchor = image.Rect(left, top, left+width, top+ui.ButtonH)
			top += ui.ButtonH + rowSpace + ui.LineHeight
			half := (width - rowSpace) / ui.Half
			made.pick.Rect = image.Rect(left, top, left+half, top+ui.ButtonH)
			made.clear.Rect = image.Rect(left+half+rowSpace, top, left+width, top+ui.ButtonH)
			top += ui.ButtonH + fieldSpace
		}
	}
	if s.form.remove != nil {
		bottom := panel.Max.Y - ui.Inset
		s.form.remove.Rect = image.Rect(left, bottom-ui.ButtonH, left+width, bottom)
	}
}

func placeTimes(from, to *ui.TextField, left, top int) {
	from.Rect = image.Rect(left, top, left+timeWidth, top+ui.ButtonH)
	toLeft := left + timeWidth + dashWidth
	to.Rect = image.Rect(toLeft, top, toLeft+timeWidth, top+ui.ButtonH)
}

// updateForm handles the inspector's buttons.
func (s *State) updateForm(input *ui.Input, ctx *frame.Context) {
	if s.form == nil {
		return
	}
	if s.form.remove != nil && s.form.remove.Update(input) {
		s.removeSelected(ctx)

		return
	}
	node, ok := s.selectedNode()
	if !ok {
		return
	}
	for index := range s.form.controls {
		made := &s.form.controls[index]
		switch made.param.Type {
		case flow.TypeTimeRanges:
			s.updateRanges(input, ctx, made, node)
		case flow.TypeAnchor:
			s.updateAnchor(input, ctx, made, node)
		case flow.TypeChoice, flow.TypeDuration:
			s.updateChoices(input, ctx, made, node)
		case flow.TypeShares:
			s.updateOptions(input, ctx, made, node)
		case flow.TypeTiles, flow.TypeText, flow.TypeTimeRange:
		}
	}
}

// updateRanges adds a time after the last one, or removes one with the
// output it led to.
func (s *State) updateRanges(input *ui.Input, ctx *frame.Context, made *control, node flow.Node) {
	spans, err := node.TimeRanges(made.param.Name)
	if err != nil {
		return
	}
	if made.addRange.Update(input) {
		if len(spans) >= flow.MaxRanges {
			ctx.Frame.Status(fmt.Sprintf("A time switch has at most %d times", flow.MaxRanges))

			return
		}
		from := 0
		if len(spans) > 0 {
			from = spans[len(spans)-1].To
		}
		spans = append(spans, flow.TimeRange{Port: newPort(spans), From: from, To: (from + hour) % flow.MinutesPerDay})
		s.saveParam(ctx, node, made.param.Name, spans, nil)

		return
	}
	for index, row := range made.rows {
		if !row.remove.Update(input) || index >= len(spans) {
			continue
		}
		changed := node.WithLink(spans[index].Port, ids.ULID{})
		spans = slices.Delete(spans, index, index+1)
		s.saveParam(ctx, changed, made.param.Name, spans, nil)

		return
	}
}

const hour = 60

// newPort returns an output name no time of a switch uses yet.
func newPort(spans []flow.TimeRange) string {
	for number := 1; ; number++ {
		name := "r" + strconv.Itoa(number)
		if !slices.ContainsFunc(spans, func(span flow.TimeRange) bool { return span.Port == name }) {
			return name
		}
	}
}

// updateAnchor picks the destination on a map, or clears it.
func (s *State) updateAnchor(input *ui.Input, ctx *frame.Context, made *control, node flow.Node) {
	made.pick.Label = "Pick on map"
	if s.picker.Picking() {
		made.pick.Label = "Cancel pick"
	}
	switch {
	case made.pick.Update(input):
		if s.picker.Picking() {
			s.picker.Cancel(ctx, "cancelled in the workflow window")

			return
		}
		label := "a Go to in “" + s.name(ctx.Docs) + "”"
		nodeID, param := node.ID, made.param.Name
		s.picker.Start(ctx, label, func(answer protocol.PickAnswer, ctx *frame.Context) {
			anchor, err := ids.Parse(answer.Item)
			if err != nil {
				ctx.Frame.Status("The map answered with something that is not an anchor")

				return
			}
			current, ok := s.currentNode(ctx.Docs, nodeID)
			if !ok {
				return
			}
			s.subscribe(answer.Floor, ctx)
			s.saveParam(ctx, current, param, floor.AnchorRef{Floor: answer.Floor, Anchor: anchor}, nil)
		})
	case made.clear.Update(input):
		s.saveParam(ctx, node, made.param.Name, floor.AnchorRef{Floor: ids.ULID{}, Anchor: ids.ULID{}}, nil)
	}
}

// currentNode reads a node from the window's copy of the workflow as it is
// now, for replies that arrive between frames.
func (s *State) currentNode(docs frame.Docs, nodeID ids.ULID) (flow.Node, bool) {
	var found flow.Node
	loaded := false
	docs.View(s.document, func(document client.Document) {
		if workflowDoc, isWorkflow := document.Model.(*workflowplugin.Document); isWorkflow {
			found, loaded = workflowDoc.Workflow.Node(nodeID)
		}
	})

	return found, loaded
}

// saveField saves the parameter a field edits when Enter is pressed in it.
func (s *State) saveField(field *ui.TextField, ctx *frame.Context) {
	node, ok := s.selectedNode()
	if !ok || s.form == nil {
		return
	}
	value, changed := field.Field.Commit()
	if !changed {
		s.focus.Set(nil)

		return
	}
	for index := range s.form.controls {
		made := &s.form.controls[index]
		saved, handled, problem := parseFor(made, field, value, node)
		if !handled {
			continue
		}
		if problem != "" {
			field.Field.Failed()
			ctx.Frame.Status(problem)

			return
		}
		s.saveParam(ctx, node, made.param.Name, saved, field)

		return
	}
}

// parseFor reads a typed value from a field of a control: the new parameter
// value, whether the field belongs to the control, and what is wrong with
// the text.
func parseFor(made *control, field *ui.TextField, value string, node flow.Node) (any, bool, string) {
	switch made.param.Type {
	case flow.TypeTiles:
		if field != made.field {
			return nil, false, ""
		}

		return parseWhole(value, 1, flow.MaxTiles)
	case flow.TypeDuration:
		return parseAmount(made, field, value, node)
	case flow.TypeShares:
		return parseOptionField(made, field, value, node)
	case flow.TypeChoice:
		return nil, false, ""
	case flow.TypeText:
		if field != made.field {
			return nil, false, ""
		}

		return strings.TrimSpace(value), true, ""
	case flow.TypeTimeRange:
		return parseRangeField(made, field, value, node)
	case flow.TypeTimeRanges:
		return parseRangesField(made, field, value, node)
	case flow.TypeAnchor:
	}

	return nil, false, ""
}

// parseRangeField reads the start or end of a time range.
func parseRangeField(made *control, field *ui.TextField, value string, node flow.Node) (any, bool, string) {
	if field != made.from && field != made.to {
		return nil, false, ""
	}
	span, err := node.TimeRange(made.param.Name)
	if err != nil {
		return nil, true, err.Error()
	}
	changed, problem := timeInto(span, value, field == made.from)
	if problem != "" {
		return nil, true, problem
	}

	return changed, true, ""
}

// parseRangesField reads the start or end of one time of a time switch.
func parseRangesField(made *control, field *ui.TextField, value string, node flow.Node) (any, bool, string) {
	spans, err := node.TimeRanges(made.param.Name)
	if err != nil {
		return nil, false, ""
	}
	for index, row := range made.rows {
		if (field != row.from && field != row.to) || index >= len(spans) {
			continue
		}
		changed, problem := timeInto(spans[index], value, field == row.from)
		if problem != "" {
			return nil, true, problem
		}
		spans[index] = changed

		return spans, true, ""
	}

	return nil, false, ""
}

// parseWhole reads a whole number from low to high.
func parseWhole(value string, low, high int) (any, bool, string) {
	number, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || number < low || number > high {
		return nil, true, fmt.Sprintf("Enter a whole number from %d to %d", low, high)
	}

	return number, true, ""
}

// timeInto sets the start or end of a range from typed text, or says what
// is wrong with it.
func timeInto(span flow.TimeRange, value string, start bool) (flow.TimeRange, string) {
	minute, ok := parseTime(value)
	if !ok {
		return span, "Enter a time of day such as 09:30"
	}
	if start {
		span.From = minute
	} else {
		span.To = minute
	}
	if span.From == span.To {
		return span, "A time needs to end after it starts"
	}

	return span, ""
}

// parseTime reads "9", "9:30", "09:30" or "0930" as minutes from midnight.
func parseTime(value string) (int, bool) {
	value = strings.TrimSpace(value)
	hours, minutes, found := strings.Cut(value, ":")
	if !found {
		hours, minutes = value, "0"
		if len(value) == clockDigits {
			hours, minutes = value[:minuteDigits], value[minuteDigits:]
		}
	}
	hour, hourErr := strconv.Atoi(hours)
	minute, minuteErr := strconv.Atoi(minutes)
	if hourErr != nil || minuteErr != nil || hour < 0 || hour >= hoursPerDay || minute < 0 || minute >= 60 {
		return 0, false
	}

	return hour*60 + minute, true
}

// saveParam sends a node with one parameter changed; field, when given, is
// marked as failed if the change is refused.
func (s *State) saveParam(ctx *frame.Context, node flow.Node, name string, value any, field *ui.TextField) {
	changed, err := node.WithParam(name, value)
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	s.command(ctx, workflowplugin.ReplaceEdit, "", workflowplugin.ReplaceArgs{Node: changed}, func(protocol.Result) {
		if field != nil {
			s.focus.Set(nil)
		}
	})
}

// floorName returns a floor's name, or "" for a floor the project no longer
// has.
func (s *State) floorName(docs frame.Docs, floorID ids.ULID) string {
	name := ""
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			name = index.Name(floorID, "")
		}
	})

	return name
}

// anchorLabel describes an anchor parameter's value: the anchor's name and
// where it is, or what is wrong with the reference.
func (s *State) anchorLabel(docs frame.Docs, ref floor.AnchorRef) (string, string) {
	if ref.IsZero() {
		return "No destination yet", ""
	}
	floorName := s.floorName(docs, ref.Floor)
	if floorName == "" {
		return "Broken: its floor was deleted", ""
	}
	name, detail := "", ""
	loaded := docs.View(ref.Floor, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			if anchor, _, found := floorDoc.Floor.Anchor(ref.Anchor); found {
				name, detail = anchor.Name, fmt.Sprintf("Anchor · %s · %d, %d", floorName, anchor.Col, anchor.Row)
			}
		}
	})
	switch {
	case !loaded:
		return "…", floorName
	case name == "":
		return "Broken: the anchor was deleted", floorName
	default:
		return name, detail
	}
}
