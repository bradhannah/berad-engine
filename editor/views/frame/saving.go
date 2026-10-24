package frame

import (
	"image"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/ui"
)

// Saver is a view whose typed changes wait for Save. Cmd+S saves them all,
// and closing the window with any unsaved asks first.
type Saver interface {
	// Unsaved counts typed changes not saved yet, and UnsavedName names what
	// they belong to, such as an NPC or "Wait in Workflow 1".
	Unsaved(docs Docs) int
	UnsavedName(docs Docs) string
	// Save saves them, and reports false when one cannot be saved; it says
	// why in the status bar.
	Save(ctx *Context) bool
	// Leave runs then once the author has decided about unsaved changes: at
	// once when there are none, after Save or Don't save, never after Cancel.
	Leave(ctx *Context, then func())
}

// Guard dialog button IDs, for input scripts.
const (
	GuardSave    = "save-changes"
	GuardDiscard = "discard-changes"
)

// Guard asks what to do with unsaved changes before something that would
// lose them, such as selecting something else or closing the window: Save,
// Don't save or Cancel.
type Guard struct {
	dialog *ui.Dialog
	then   func()
}

// Ask runs then at once when nothing is unsaved; otherwise it asks about the
// unsaved changes to name, and keeps then for the answer.
func (g *Guard) Ask(unsaved int, name string, then func()) {
	if unsaved == 0 {
		then()

		return
	}
	what := "1 unsaved change"
	if unsaved > 1 {
		what = strconv.Itoa(unsaved) + " unsaved changes"
	}
	g.dialog = ui.NewDialog("Save changes to “"+name+"”?", []string{"It has " + what + "."},
		ui.DialogButton(GuardSave, "Save"), ui.DialogButton(GuardDiscard, "Don't save"),
		ui.DialogButton(ui.DialogCancel, "Cancel"))
	g.then = then
}

// Active reports whether the guard is asking; while it is, it takes all
// input.
func (g *Guard) Active() bool { return g.dialog != nil }

// Update handles a frame of the dialog. Save calls save, and goes on only if
// it could; Don't save calls discard and goes on; Cancel stays.
func (g *Guard) Update(input *ui.Input, size image.Point, save func() bool, discard func()) {
	if g.dialog == nil {
		return
	}
	choice := g.dialog.Update(input, image.Rectangle{Min: image.Pt(0, 0), Max: size})
	if choice == "" {
		return
	}
	then := g.then
	g.dialog, g.then = nil, nil
	switch choice {
	case GuardSave:
		if save() {
			then()
		}
	case GuardDiscard:
		discard()
		then()
	}
}

// Draw renders the dialog while asking.
func (g *Guard) Draw(screen *ebiten.Image, theme *ui.Theme) {
	if g.dialog != nil {
		g.dialog.Draw(screen, theme)
	}
}

// Locate finds a dialog button while asking, for input scripts.
func (g *Guard) Locate(id string, size image.Point) (image.Point, bool) {
	if g.dialog == nil {
		return image.Pt(0, 0), false
	}

	return g.dialog.Locate(id, image.Rectangle{Min: image.Pt(0, 0), Max: size})
}

// UnsavedFields counts the fields with unsaved changes.
func UnsavedFields(fields ...*ui.TextField) int {
	count := 0
	for _, field := range fields {
		if field != nil && field.Field.Unsaved() {
			count++
		}
	}

	return count
}
