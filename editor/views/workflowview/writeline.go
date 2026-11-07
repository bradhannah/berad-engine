package workflowview

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/writesheet"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/show"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// writeLineID is Write… beside a message's Insert value…, which opens the
// line in the writing sheet, for input scripts "ui:write-line".
const writeLineID = "write-line"

// lineSheet is the writing sheet while it writes the selected node's line.
type lineSheet struct {
	sheet *writesheet.Sheet
	node  ids.ULID
}

// writeLineButton is Write… for a node with a message, and none for others.
func writeLineButton(kind string) *ui.Button {
	if !hasMessage(kind) {
		return nil
	}

	return ui.NewButton(writeLineID, "Write…")
}

// layoutMessageButtons places Insert value…, Make a constant… and Write…
// in a row from the top given, Write… under the others when the row is too
// narrow, and returns where the row ends.
func (s *State) layoutMessageButtons(made *control, left, width, top int) int {
	place := func(button *ui.Button, from int) image.Rectangle {
		return image.Rect(from, top, from+s.theme.TextWidth(button.Label)+ui.Inset*ui.Half, top+ui.ButtonH)
	}
	made.pick.Rect = place(made.pick, left)
	from := made.pick.Rect.Max.X + ui.Gap
	if s.form == nil {
		return top + ui.ButtonH
	}
	if s.form.makeText != nil {
		s.form.makeText.Rect = place(s.form.makeText, from)
		from = s.form.makeText.Rect.Max.X + ui.Gap
	}
	if s.form.writeLine != nil {
		s.form.writeLine.Rect = place(s.form.writeLine, from)
		if s.form.writeLine.Rect.Max.X > left+width {
			top += ui.ButtonH + rowSpace
			s.form.writeLine.Rect = place(s.form.writeLine, left)
		}
	}

	return top + ui.ButtonH
}

// updateWriteLine opens the selected node's line in the writing sheet, as
// its field holds it now, when Write… is clicked; it reports whether it
// did.
func (s *State) updateWriteLine(input *ui.Input, ctx *frame.Context, node flow.Node) bool {
	made, has := s.messageControl()
	if !has || s.form.writeLine == nil || !s.form.writeLine.Update(input) {
		return false
	}
	kind, _ := flow.KindOf(node.Kind)
	look := writesheet.LookBox
	switch node.Kind {
	case flow.KindSay:
		look = writesheet.LookBubble
	case flow.KindLog:
		look = writesheet.LookLog
	}
	s.focus.Set(nil)
	s.lineSheet = &lineSheet{
		sheet: writesheet.NewLine(s.theme, kind.Label+" · "+made.param.Label, made.field.Field.Value(), writesheet.Line{
			Bytes: messageBytes, Limit: flow.MaxMessageText, Look: look, Breaks: keepsBreaks(node.Kind),
			Reads: s.lineReads(ctx.Docs, node.ID),
		}),
		node: node.ID,
	}

	return true
}

// lineReads reads a line written in the sheet as its node would say it,
// each value as it starts, markup kept; or says what keeps it from being
// read, the line then shown as written.
func (s *State) lineReads(docs frame.Docs, nodeID ids.ULID) func(string) (string, string) {
	return func(text string) (string, string) {
		node, found := s.currentNode(docs, nodeID)
		if !found {
			return text, ""
		}
		saved, _ := node.Message() //nolint:errcheck // A message that can't be read keeps nothing.
		segments, problem := s.parseMessage(docs, text, saved)
		if segments == nil && problem != "" {
			return show.Escape(text), problem
		}

		return s.readSegments(docs, segments), ""
	}
}

// updateLineSheet gives the writing sheet a frame. What Done, Enter or
// Escape hands back goes into the line's field, as a change that waits
// for Save as typing does.
func (s *State) updateLineSheet(input *ui.Input, ctx *frame.Context) {
	saving, open := s.lineSheet.sheet.Update(input, s.screen(), ctx.Clipboard)
	if made, has := s.messageControl(); saving != nil && has && s.form.node == s.lineSheet.node {
		made.field.Field.SetDraft(saving.Text)
		s.focus.Set(made.field)
	}
	if !open {
		s.lineSheet = nil
	}
}

// drawLineSheet draws the writing sheet over the window while it is open.
func (s *State) drawLineSheet(screen *ebiten.Image) {
	if s.lineSheet != nil {
		s.lineSheet.sheet.Draw(screen, s.screen())
	}
}

// hasSheetState answers "sheet", whether the writing sheet is open, and the
// sheet's own states.
func (s *State) hasSheetState(state string, _ frame.Docs) (bool, bool) {
	if state == "sheet" {
		return s.lineSheet != nil, true
	}
	if s.lineSheet == nil {
		return false, false
	}

	return s.lineSheet.sheet.HasState(state)
}
