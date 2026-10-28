package pixelview

import (
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/tiles"
)

// "Shows" chooses the stand-in a sprite is drawn for on every map, until the
// definition editor gives definitions sprites of their own. A sprite shows at
// most one stand-in, and each stand-in has at most one sprite.

// showsItem prefixes a stand-in's ID in the Shows menu; the item for nothing
// is the prefix alone.
const showsItem = "standin:"

// standInSprites returns which sprite shows each stand-in.
func standInSprites(docs frame.Docs) map[string]ids.ULID {
	record := map[string]ids.ULID{}
	docs.View(art.StandInsID(), func(document client.Document) {
		if standIns, ok := document.Model.(*artplugin.StandInsDocument); ok {
			for _, standIn := range standIns.Sprites.StandIns() {
				record[standIn], _ = standIns.Sprites.Sprite(standIn)
			}
		}
	})

	return record
}

// shownStandIn returns the stand-in the sprite shows, or "" for none.
func (s *State) shownStandIn(docs frame.Docs) string {
	record := standInSprites(docs)
	for _, definition := range tiles.StandIns() {
		if record[definition.ID] == s.sprite {
			return definition.ID
		}
	}

	return ""
}

// standInName names a stand-in as authors know it.
func standInName(standIn string) string {
	if definition, ok := tiles.Lookup(standIn); ok {
		return definition.Name
	}

	return standIn
}

// spriteName names a sprite from the project index.
func spriteName(docs frame.Docs, sprite ids.ULID) string {
	name := "another sprite"
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			name = index.Name(sprite, name)
		}
	})

	return name
}

// layoutShows places the Shows heading and its button just above the save
// bar.
func (s *State) layoutShows(docs frame.Docs, left, width int) {
	top := s.save.Rect.Min.Y - ui.Inset - ui.ButtonH
	s.shows.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
	s.panel.showsTop = top - ui.LineHeight
	s.shows.Label = "Nothing"
	if standIn := s.shownStandIn(docs); standIn != "" {
		s.shows.Label = standInName(standIn)
	}
}

// openShows offers nothing and every stand-in a sprite can show, naming the
// sprite that already shows one.
func (s *State) openShows(docs frame.Docs) {
	record := standInSprites(docs)
	items := []ui.MenuItem{{ID: showsItem, Label: "Nothing", Icon: ui.IconNone, Disabled: false, Danger: false,
		Rule: false}}
	for _, definition := range tiles.StandIns() {
		if !art.CanShowSprite(definition.ID) {
			continue
		}
		label := definition.Name
		if sprite, shown := record[definition.ID]; shown && sprite != s.sprite {
			label += " — " + spriteName(docs, sprite)
		}
		items = append(items, ui.MenuItem{
			ID: showsItem + definition.ID, Label: label, Icon: ui.IconNone, Disabled: false, Danger: false,
			Rule: len(items) == 1,
		})
	}
	window := image.Rectangle{Min: image.Pt(0, 0), Max: s.size}
	s.showsMenu.Items = items
	s.showsMenu.Width = min(ui.MenuWidth(items, s.theme), window.Dx()-ui.Half*ui.Inset)
	s.showsMenu.OpenAt(image.Pt(s.shows.Rect.Min.X, s.shows.Rect.Max.Y+ui.Gap), window)
}

// updateShows handles the open Shows menu, which takes all input.
func (s *State) updateShows(input *ui.Input, ctx *frame.Context) {
	chosen := s.showsMenu.Update(input)
	input.ClearAll()
	if strings.HasPrefix(chosen, showsItem) {
		s.chooseShows(ctx, strings.TrimPrefix(chosen, showsItem))
	}
}

// chooseShows makes the sprite show a stand-in, or nothing, in one undo
// step: the stand-in it showed goes back to its flat colour, and a stand-in
// another sprite showed is taken over.
func (s *State) chooseShows(ctx *frame.Context, standIn string) {
	current := s.shownStandIn(ctx.Docs)
	if standIn == current {
		return
	}
	s.stroke.count++
	gesture := s.gesture()
	if current != "" {
		back := artplugin.StandInArgs{StandIn: current, Sprite: ids.ULID{}}
		s.command(ctx, art.StandInsID(), artplugin.StandInEdit, gesture, back, nil, nil)
	}
	if standIn == "" {
		ctx.Frame.Status("Shows nothing: " + standInName(current) + " is its flat colour again")

		return
	}
	status := "Shows " + standInName(standIn) + " on every map"
	if previous, shown := standInSprites(ctx.Docs)[standIn]; shown {
		status += ", in place of " + spriteName(ctx.Docs, previous)
	}
	set := artplugin.StandInArgs{StandIn: standIn, Sprite: s.sprite}
	s.command(ctx, art.StandInsID(), artplugin.StandInEdit, gesture, set, func() { ctx.Frame.Status(status) }, nil)
}

// drawShows draws the Shows heading and its button.
func (s *State) drawShows(screen *ebiten.Image, left int) {
	if s.shows.Rect.Empty() {
		return
	}
	s.theme.Heading(screen, "SHOWS", left, s.panel.showsTop)
	s.shows.Draw(screen, s.theme)
}
