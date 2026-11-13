package window

import (
	"encoding/json"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
)

// A project that cannot open as it is (increment 57) asks its author, in
// the Project tab, before anything happens: Quit, which changes nothing, or
// Open without them, which leaves out what cannot be read and keeps the
// journal whole in the backups.

// The opening question's buttons, for input scripts, and the width its
// lines wrap at, inside the dialog's.
const (
	openingQuit  = "opening-quit"
	openingOpen  = "opening-open"
	openingLineW = 380
)

// takeOpening shows the question in the tab that quits the editor when it
// closes, the Project tab, while the project waits for an answer.
func (h *host) takeOpening(value json.RawMessage) {
	if !h.closeQuits {
		return
	}
	var asked protocol.Opening
	if json.Unmarshal(value, &asked) != nil || len(asked.Files) == 0 && asked.Journal == "" {
		h.openingDialog = nil

		return
	}
	h.openingDialog = openingDialog(asked, func(line string) []string { return h.theme.Wrap(line, openingLineW) })
}

// openingDialog names each file that cannot be read and why, and what is
// wrong with the journal, each wrapped to fit.
func openingDialog(asked protocol.Opening, wrap func(string) []string) *ui.Dialog {
	var lines []string
	if len(asked.Files) > 0 {
		lines = append(lines, "These cannot be read:")
		for _, file := range asked.Files {
			lines = append(lines, wrap(file.Path+" · "+file.Reason)...)
		}
	}
	if asked.Journal != "" {
		lines = append(lines, wrap(asked.Journal+".")...)
	}
	open := "Open without them: what cannot be read is left out and never written over, " +
		"so that it can be fixed and the project opened again."
	if asked.Journal != "" {
		open = "Open without them: what cannot be read is left out and never written over, " +
			"and the journal is kept whole in the backups."
	}
	lines = append(append(lines, ""), wrap(open)...)

	return ui.NewDialog(asked.Project+" can't open as it is", lines,
		ui.DialogButton(openingQuit, "Quit"), ui.DialogButton(openingOpen, "Open without them"))
}

// updateOpeningDialog handles the question while it shows; it takes all of
// the frame's input.
func (h *host) updateOpeningDialog(input *ui.Input) {
	if h.openingDialog == nil {
		return
	}
	switch h.openingDialog.Update(input, h.shows()) {
	case "":
		return
	case openingOpen:
		if err := h.conn.NoticeChoice(protocol.OpeningKey, protocol.OpenAnyway); err != nil {
			h.frame.Refuse("Could not open the project: " + err.Error())

			return
		}
	case openingQuit:
		h.quit(false)
	}
	h.openingDialog = nil
}
