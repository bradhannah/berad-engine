package frame

import (
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
)

// Lasting notices (increment 57): a notice can carry buttons, such as Keep
// mine and Take the file's for a file changed outside the editor, and when
// several last at once the newest shows with how many there are, a button
// that opens History, where each is kept.

// Sizes of a notice's buttons, in screen pixels: each is as wide as its
// label at about this much a letter, and its padding.
const (
	noticeLetter  = 7
	noticePadding = 24
)

// noticeState is the notice's buttons, how many other notices last beside
// it, and the button last pressed until the window takes it.
type noticeState struct {
	key     string
	buttons []*ui.Button
	actions []string
	others  *ui.Button
	more    int
	chosen  string
}

func newNoticeState() noticeState {
	return noticeState{key: "", buttons: nil, actions: nil, others: ui.NewButton("notices", ""), more: 0, chosen: ""}
}

// ShowNotice shows a lasting notice, with its buttons and how many others
// last beside it, in place of the one before. A notice with no text ends
// the one shown.
func (f *Frame) ShowNotice(notice protocol.Notice) {
	f.Notice(notice.Text)
	f.notices.key, f.notices.more, f.notices.chosen = notice.Key, notice.More, ""
	f.notices.buttons, f.notices.actions = nil, nil
	for _, action := range notice.Actions {
		f.notices.buttons = append(f.notices.buttons, ui.NewButton("notice-"+action.ID, action.Label))
		f.notices.actions = append(f.notices.actions, action.ID)
	}
	f.notices.others.Label = strconv.Itoa(notice.More+1) + " notices"
}

// TakeNoticeChoice is the button last pressed on the notice, once: the
// notice's key and the button's action.
func (f *Frame) TakeNoticeChoice() (string, string, bool) {
	chosen := f.notices.chosen
	f.notices.chosen = ""

	return f.notices.key, chosen, chosen != ""
}

// layoutNotice places the notice's buttons, right to left, ending at right,
// while the status bar shows the notice, and returns where they start.
func (f *Frame) layoutNotice(top, right int) int {
	for _, button := range f.notices.buttons {
		button.Rect = ui.Zero()
	}
	f.notices.others.Rect = ui.Zero()
	if _, kind := f.shown(); kind != shownNotice {
		return right
	}
	place := func(button *ui.Button) {
		width := len([]rune(button.Label))*noticeLetter + noticePadding
		button.Rect = image.Rect(right-width, top, right, top+ui.ButtonH)
		right = button.Rect.Min.X - ui.Gap
	}
	if f.notices.more > 0 {
		place(f.notices.others)
	}
	for index := len(f.notices.buttons) - 1; index >= 0; index-- {
		place(f.notices.buttons[index])
	}

	return right + ui.Gap
}

// noticeLeft is where the notice's buttons start, or ok false with none
// shown.
func (f *Frame) noticeLeft() (int, bool) {
	left, shows := 0, false
	for _, button := range append([]*ui.Button{f.notices.others}, f.notices.buttons...) {
		if !button.Rect.Empty() && (!shows || button.Rect.Min.X < left) {
			left, shows = button.Rect.Min.X, true
		}
	}

	return left, shows
}

// updateNotice handles the notice's buttons: one of its own is kept for
// the window to take, and the count opens History.
func (f *Frame) updateNotice(input *ui.Input) {
	for index, button := range f.notices.buttons {
		if button.Update(input) {
			f.notices.chosen = f.notices.actions[index]
		}
	}
	if f.notices.others.Update(input) {
		f.history.open = true
		f.history.list.Selected = ""
	}
}

// drawNotice draws the notice's buttons where they show.
func (f *Frame) drawNotice(screen *ebiten.Image, theme *ui.Theme) {
	f.notices.others.Draw(screen, theme)
	for _, button := range f.notices.buttons {
		button.Draw(screen, theme)
	}
}

// locateNotice finds a notice's buttons for input scripts, "ui:notice-<id>"
// and "ui:notices", and answers "state:notices=<n>", how many last.
func (f *Frame) locateNotice(name string) (image.Point, bool, bool) {
	if name == "ui:notices" {
		return ui.Center(f.notices.others.Rect), !f.notices.others.Rect.Empty(), true
	}
	if count, ok := strings.CutPrefix(name, "state:notices="); ok {
		lasting := f.notices.more + 1
		if f.notice == "" {
			lasting = 0
		}

		return image.Pt(0, 0), strconv.Itoa(lasting) == count, true
	}
	for _, button := range f.notices.buttons {
		if name == "ui:"+button.ID {
			return ui.Center(button.Rect), !button.Rect.Empty(), true
		}
	}

	return image.Pt(0, 0), false, false
}
