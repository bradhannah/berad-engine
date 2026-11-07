package stateview

import (
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/valuetree"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/show"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// ConstantItem starts the item a Constants window is opened at: a
// constant, as "constant:<id>", or an NPC's or zone's own value of a
// template's constant, as "constant:<id>:<holder>".
const ConstantItem = "constant:"

// Found is a constant a search finds: the item that opens the Constants
// window at it, its path, whose own value matched if one did, and the text
// that matched, or for a path that matched, its value. Text is true when
// the words were found in its text, not its path.
type Found struct {
	Item   string
	Path   string
	Holder string
	Said   string
	Text   bool
}

// FindConstants lists the constants whose path, default text or an NPC's
// or zone's own text has some words, as Find a constant finds them, with
// markup's tags left out of the text. A constant found by its path is
// listed once; otherwise each text that has the words is.
func FindConstants(docs frame.Docs, words string) []Found {
	query := strings.ToLower(strings.TrimSpace(words))
	catalog, ready := valuetree.Catalog(docs)
	world, loaded := valuetree.World(docs)
	if query == "" || !ready || !loaded {
		return nil
	}
	finding := &finder{query: query, found: nil}
	walk(world.Values, worldstate.ScopeWorld, ids.ULID{}, nil, func(declared worldstate.Declaration, _ []ids.ULID) {
		if declared.IsConstant() {
			finding.look(declared, world.Values.Path(declared.ID), nil)
		}
	})
	for _, scope := range []worldstate.Scope{worldstate.ScopeZone, worldstate.ScopeNPC} {
		finding.templates(world, catalog, scope)
		finding.owned(docs, catalog, scope)
	}

	return finding.found
}

// finder gathers the constants a search finds.
type finder struct {
	query string
	found []Found
}

// look adds a constant if its path, its default text or one of its
// holders' own texts has the words.
func (f *finder) look(declared worldstate.Declaration, path string, holders []holderText) {
	item := ConstantItem + declared.ID.String()
	if strings.Contains(strings.ToLower(path), f.query) {
		f.found = append(f.found, Found{Item: item, Path: path, Holder: "", Said: valueText(declared), Text: false})

		return
	}
	if text := plainText(declared); text != "" && strings.Contains(strings.ToLower(text), f.query) {
		f.found = append(f.found, Found{Item: item, Path: path, Holder: "", Said: text, Text: true})
	}
	for _, holder := range holders {
		if strings.Contains(strings.ToLower(holder.text), f.query) {
			f.found = append(f.found, Found{
				Item: item + ":" + holder.id.String(), Path: path, Holder: holder.name, Said: holder.text, Text: true,
			})
		}
	}
}

// templates looks at the constants of a scope's templates, with each
// holder's own text.
func (f *finder) templates(world valuetree.List, catalog *worldstate.Catalog, scope worldstate.Scope) {
	for _, template := range world.Values.Children(scope, ids.ULID{}) {
		if template.Type != worldstate.TypeTemplate {
			continue
		}
		walk(world.Values, scope, template.ID, nil, func(declared worldstate.Declaration, _ []ids.ULID) {
			if declared.IsConstant() {
				f.look(declared, world.Values.Path(declared.ID), holdersOf(catalog, template.ID, declared))
			}
		})
	}
}

// owned looks at each NPC's or zone's own constants.
func (f *finder) owned(docs frame.Docs, catalog *worldstate.Catalog, scope worldstate.Scope) {
	for _, ownerID := range catalog.Owners(scope) {
		owner, _ := catalog.Owner(ownerID)
		own, held := valuetree.Owned(docs, ownerID)
		if !held {
			continue
		}
		walk(own.Values, scope, ids.ULID{}, nil, func(declared worldstate.Declaration, _ []ids.ULID) {
			if declared.IsConstant() {
				f.look(declared, worldstate.NamedPath(own.Values.Path(declared.ID), ownerName(owner, ownerID)), nil)
			}
		})
	}
}

// holderText is an NPC's or zone's own text for a template's constant.
type holderText struct {
	id         ids.ULID
	name, text string
}

// holdersOf lists the NPCs' or zones' own texts of a template's text
// constant, markup's tags left out.
func holdersOf(catalog *worldstate.Catalog, template ids.ULID, declared worldstate.Declaration) []holderText {
	if declared.Type != worldstate.TypeText {
		return nil
	}
	var texts []holderText
	for _, holder := range catalog.Holders(template) {
		owner, _ := catalog.Owner(holder)
		if value, own := owner.Owned.ConstantValue(declared); own {
			texts = append(texts, holderText{id: holder, name: owner.Name, text: show.Plain(value.Text)})
		}
	}

	return texts
}

// plainText is a text constant's default without markup's tags; "" for a
// number.
func plainText(declared worldstate.Declaration) string {
	if declared.Type != worldstate.TypeText {
		return ""
	}

	return show.Plain(declared.Zero().Text)
}

// valueText is a constant's default as a search shows it: its text without
// tags, or its number.
func valueText(declared worldstate.Declaration) string {
	if declared.Type == worldstate.TypeText {
		return plainText(declared)
	}

	return strconv.Itoa(declared.Zero().Integer)
}

// showFound shows a constant a search asked for: selected in the tree, or
// for an NPC's or zone's own value, that holder's row of it.
func (s *State) showFound(ctx *frame.Context) {
	if s.showing == "" {
		return
	}
	idText, holderText, _ := strings.Cut(strings.TrimPrefix(s.showing, ConstantItem), ":")
	s.showing = ""
	constant, err := ids.Parse(idText)
	if err != nil {
		return
	}
	shown, found := placeOfConstant(ctx.Docs, constant)
	if !found {
		return
	}
	holder, err := ids.Parse(holderText)
	if err != nil || shown.template.IsZero() {
		s.showPlace(ctx, shown)

		return
	}
	s.whenLeft(ctx, func() {
		s.toggled[sectionPrefix+string(shown.scope)] = false
		s.toggled[ownerPrefix+holder.String()] = true
		s.toggled[appliedPrefix+holder.String()+":"+shown.template.String()] = false
		s.revealID = heldPrefix + holder.String() + ":" + constant.String()
	})
}

// revealHeld selects the row showFound waits for, once the tree has it,
// scrolled into view.
func (s *State) revealHeld() {
	meaning, found := s.shown.items[s.revealID]
	if !found {
		return
	}
	s.list.Selected = s.revealID
	s.list.Reveal(s.revealID)
	s.inspect(meaning)
	s.revealID = ""
}

// placeOfConstant is where a constant sits in the tree: the world's or a
// template's, or an NPC's or zone's own.
func placeOfConstant(docs frame.Docs, constant ids.ULID) (place, bool) {
	world, loaded := valuetree.World(docs)
	catalog, ready := valuetree.Catalog(docs)
	if !loaded || !ready {
		return noPlace(), false
	}
	if declared, found := world.Values.Value(constant); found {
		template, _ := world.Values.Template(constant)

		return place{
			scope: declared.Scope, owner: ids.ULID{}, template: template, groups: groupsOf(world.Values, declared),
			path: world.Values.Path(constant),
		}, true
	}
	for _, scope := range []worldstate.Scope{worldstate.ScopeZone, worldstate.ScopeNPC} {
		for _, ownerID := range catalog.Owners(scope) {
			owner, _ := catalog.Owner(ownerID)
			own, held := valuetree.Owned(docs, ownerID)
			if declared, found := own.Values.Value(constant); held && found {
				return place{
					scope: scope, owner: ownerID, template: ids.ULID{}, groups: groupsOf(own.Values, declared),
					path: worldstate.NamedPath(own.Values.Path(constant), ownerName(owner, ownerID)),
				}, true
			}
		}
	}

	return noPlace(), false
}

// groupsOf lists the groups holding a declaration, outermost first.
func groupsOf(list *worldstate.Declarations, declared worldstate.Declaration) []ids.ULID {
	var groups []ids.ULID
	parent, ok := list.Value(declared.Parent)
	for ok && parent.Type == worldstate.TypeGroup {
		groups = append([]ids.ULID{parent.ID}, groups...)
		parent, ok = list.Value(parent.Parent)
	}

	return groups
}

// noPlace is no place in the tree.
func noPlace() place {
	return place{scope: "", owner: ids.ULID{}, template: ids.ULID{}, groups: nil, path: ""}
}
