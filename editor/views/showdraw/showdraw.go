// Package showdraw draws what workflows show the player in the editor's
// views: a speaker's line in a bubble over its head, and the message box at
// the foot of the view. The engine's player draws them, as it does for a
// game (increment 62); this package gives it the editor's faces and the
// scale the window draws at.
package showdraw

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/pen"
	"github.com/bradhannah/berad-engine/engine/player"
	"github.com/bradhannah/berad-engine/engine/show"
)

const (
	// CloseHint says how to close a box that waits for the player.
	CloseHint = player.CloseHint
	// BoxLines is how many lines one box holds before it would need paging,
	// which is not built yet.
	BoxLines = player.BoxLines
)

// Writer writes what a game shows in the editor's faces, at the scale the
// window draws at now.
func Writer(theme *ui.Theme) player.Writer {
	return player.Writer{Type: theme.Shown, Scale: pen.Scale()}
}

// BoxOf is the box a stage shows now, as a preview's state carries it.
func BoxOf(stage *show.Stage) *protocol.PreviewBox {
	box := player.BoxOf(stage)
	if box == nil {
		return nil
	}

	return &protocol.PreviewBox{Text: box.Text, Name: box.Name, Left: box.Left, Queued: box.Queued, Closing: box.Closing}
}

// boxOf is a preview's box as the player draws it.
func boxOf(box *protocol.PreviewBox) *player.Box {
	if box == nil {
		return nil
	}

	return &player.Box{Text: box.Text, Name: box.Name, Left: box.Left, Queued: box.Queued, Closing: box.Closing}
}

// BubbleRect is where a line's bubble goes over a head, its tail's tip at
// the head, and the line wrapped to fit.
func BubbleRect(theme *ui.Theme, head image.Point, text string) (image.Rectangle, []player.StyledLine) {
	return Writer(theme).BubbleRect(head, text)
}

// DrawBubble draws a line in a bubble over a head, fading as the share of
// its time left comes to an end.
func DrawBubble(screen *ebiten.Image, theme *ui.Theme, head image.Point, text string, left float64) {
	Writer(theme).DrawBubble(screen, head, text, left)
}

// BoxRect is where a box goes, at the foot of an area, and its text wrapped
// to fit.
func BoxRect(theme *ui.Theme, area image.Rectangle, box *protocol.PreviewBox) (image.Rectangle, []player.StyledLine) {
	return Writer(theme).BoxRect(area, boxOf(box))
}

// DrawBox draws a box at the foot of an area: its name, its text, how many
// wait with it, and the bar draining as its time runs, or how to close it.
func DrawBox(screen *ebiten.Image, theme *ui.Theme, area image.Rectangle, box *protocol.PreviewBox) {
	Writer(theme).DrawBox(screen, area, boxOf(box))
}
