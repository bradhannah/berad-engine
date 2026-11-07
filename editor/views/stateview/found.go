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
