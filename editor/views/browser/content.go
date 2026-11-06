package browser

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// The main pane (increment 32): what the place picked holds, its folders
// first, then its things. While narrowed it lists only what is kept, and
// Matches lists everything kept, grouped by the folder each is in.

// groupPrefix starts a Matches group's heading.
const groupPrefix = "group:"

// contentRows lists what the place picked holds.
func (s *State) contentRows(docs frame.Docs, index *project.Index, things []thing, counted placeCounts) []ui.Row {
	kept := s.keptIDs(things)
	switch s.place {
	case placeHome:
		return s.homeRows(docs, index, time.Now())
	case placeMatches:
		return s.matchRows(docs, index, things)
	case sectionWorldState:
		return s.worldParts(docs)
	case settingsRow:
		return s.settingsRows(docs, newMatcher(""))
	case sectionLoose:
		return s.looseContent(docs, index, kept)
	case sectionAudio:
		return s.categoryContent(counted)
	}
	if category, isCategory := strings.CutPrefix(s.place, audioCategoryPrefix); isCategory {
		return s.cueRows(category, kept)
	}
	format, path, isFolder := placeFolderOf(s.place)
	if !isFolder {
		return nil
	}

	return s.folderContent(docs, index, format, path, kept, counted)
}

// placeFolderOf reads the document type and folder of a kind's or a
// folder's place.
func placeFolderOf(place string) (string, string, bool) {
	if folder, isFolder := parseFolderRow(place); isFolder {
		return folder.Format, folder.Path, true
	}
	if format := sectionFormat(place); format != "" {
		return format, "", true
	}

	return "", "", false
}

// keptIDs returns the rows of what the search and ticks keep, or nil when
// nothing narrows.
func (s *State) keptIDs(things []thing) map[string]bool {
	if !s.narrowed() {
		return nil
	}
	kept := map[string]bool{}
	for _, found := range things {
		if s.kept(found, "") {
			kept[found.row.ID] = true
		}
	}

	return kept
}

// folderContent lists a folder: its folders, each with how many things it
// holds, then the things directly in it.
func (s *State) folderContent(
	docs frame.Docs, index *project.Index, format, path string, kept map[string]bool, counted placeCounts,
) []ui.Row {
	var rows []ui.Row
	for _, folder := range index.Folders(format) {
		id := folderRowID(format, folder)
		if project.FolderParent(folder) != path || kept != nil && counted.kept[id] == 0 {
			continue
		}
		row := ui.NewRow(id, project.FolderName(folder))
		row.Icon, row.Detail = ui.IconFolder, s.countLabel(counted, id)
		rows = append(rows, row)
	}
	var uses map[ids.ULID]int
	if format == flow.FormatName {
		uses = s.uses(docs, index)
	}
	for _, entry := range index.OfFormat(format) {
		if entry.Folder == path && s.keptDocument(docs, entry, kept) {
			rows = append(rows, s.documentRows(docs, index, entry, uses)...)
		}
	}

	return rows
}

// keptDocument reports whether a document is listed: always, or while
// narrowed when it is kept, or for a zone when one of its floors is.
func (s *State) keptDocument(docs frame.Docs, entry project.Entry, kept map[string]bool) bool {
	if kept == nil || kept[entry.ID.String()] {
		return true
	}
	if entry.Format != zone.FormatName {
		return false
	}

	return slices.ContainsFunc(s.floorsOf(docs, entry.ID), func(floorID ids.ULID) bool { return kept[floorID.String()] })
}

// documentRows returns a document's row: a zone with its floors, a
// workflow with how many use it, an NPC with what it follows, and art with
// its detail.
func (s *State) documentRows(
	docs frame.Docs, index *project.Index, entry project.Entry, uses map[ids.ULID]int,
) []ui.Row {
	switch entry.Format {
	case zone.FormatName:
		return s.zoneRows(index, entry, s.floorsOf(docs, entry.ID), 0, false)
	case flow.FormatName:
		return []ui.Row{workflowRow(entry, uses[entry.ID], 0)}
	case npc.FormatName:
		return []ui.Row{item(entry, ui.IconNPC, s.follows(docs, index, entry.ID), 0)}
	case floor.FormatName:
		return []ui.Row{item(entry, ui.IconFloor, "", 0)}
	}
	info, _ := kindInfoOf(entry.Format)

	return []ui.Row{item(entry, info.icon, paletteDetail(entry), 0)}
}

// worldParts lists World state's Values and Signals.
func (s *State) worldParts(docs frame.Docs) []ui.Row {
	var rows []ui.Row
	for _, row := range s.worldStateRows(docs, newMatcher("")) {
		if !row.Section {
			row.Indent = 0
			rows = append(rows, row)
		}
	}

	return rows
}

// looseContent lists the floors in no zone.
func (s *State) looseContent(docs frame.Docs, index *project.Index, kept map[string]bool) []ui.Row {
	loose := s.looseFloors(docs)
	var rows []ui.Row
	for _, entry := range index.OfFormat(floor.FormatName) {
		if loose[entry.ID] && (kept == nil || kept[entry.ID.String()]) {
			rows = append(rows, item(entry, ui.IconFloor, "", 0))
		}
	}

	return rows
}

// categoryContent lists the audio library's categories, the sounds'
// folders.
func (s *State) categoryContent(counted placeCounts) []ui.Row {
	rows := s.categoryPlaces(counted)
	for index := range rows {
		rows[index].Indent = 0
	}

	return rows
}

// cueRows lists a category's sounds, each with its ID beside its name.
func (s *State) cueRows(category string, kept map[string]bool) []ui.Row {
	if s.audio.library == nil {
		return nil
	}
	var rows []ui.Row
	for _, cue := range s.audio.library.Cues {
		id := audioCuePrefix + cue.ID
		if cue.Category != category || kept != nil && !kept[id] {
			continue
		}
		row := ui.NewRow(id, cue.Name)
		row.Icon, row.Aside, row.Detail = ui.IconSound, cue.ID, audioDetail(cue)
		rows = append(rows, row)
	}

	return rows
}

// matchGroup is a group of Matches: the place its things are in, its
// heading, and its things.
type matchGroup struct {
	key, label string
	rank       int
	things     []thing
}

// matchRows lists everything kept, grouped by the folder each is in, the
// groups in the column's order.
func (s *State) matchRows(docs frame.Docs, index *project.Index, things []thing) []ui.Row {
	groups := map[string]*matchGroup{}
	for _, found := range things {
		if !s.kept(found, "") {
			continue
		}
		key, label, rank := groupOf(found)
		if groups[key] == nil {
			groups[key] = &matchGroup{key: key, label: label, rank: rank, things: nil}
		}
		groups[key].things = append(groups[key].things, found)
	}
	ordered := slices.SortedFunc(func(yield func(*matchGroup) bool) {
		for _, group := range groups {
			if !yield(group) {
				return
			}
		}
	}, func(a, b *matchGroup) int { return cmp.Or(cmp.Compare(a.rank, b.rank), strings.Compare(a.key, b.key)) })
	uses := s.uses(docs, index)
	var rows []ui.Row
	for _, group := range ordered {
		heading := ui.NewHeading(groupPrefix+group.key, group.label)
		heading.Detail = strconv.Itoa(len(group.things))
		rows = append(rows, heading)
		for _, found := range group.things {
			rows = append(rows, s.matchRow(docs, index, found, uses)...)
		}
	}

	return rows
}

// matchRow is a kept thing's row in Matches: a document as its folder
// lists it, or the thing's own row.
func (s *State) matchRow(docs frame.Docs, index *project.Index, found thing, uses map[ids.ULID]int) []ui.Row {
	if documentID, err := ids.Parse(found.row.ID); err == nil {
		if entry, ok := index.Entry(documentID); ok {
			return s.documentRows(docs, index, entry, uses)
		}
	}
	row := found.row
	row.Tree, row.Indent = false, 0

	return []ui.Row{row}
}

// groupOf returns the group a kept thing is in: its folder, or its kind's
// top; its heading, as "Workflows / Harbour / Shops"; and its rank, the
// kind's place in the column.
func groupOf(found thing) (string, string, int) {
	kinds := found.values[facetKind]
	if len(kinds) == 0 {
		return "other", "Settings and World state", len(placeKinds())
	}
	place := kindPlace(kinds[0])
	label, _ := kindLabel(place)
	if kinds[0] == floor.FormatName {
		label = "Floors"
	}
	rank := slices.Index(placeKinds(), place)
	folders := found.values[facetFolder]
	if len(folders) == 0 {
		return kinds[0], label, rank
	}
	_, path, _ := strings.Cut(folders[0], ":")

	return folders[0], label + " / " + strings.ReplaceAll(path, project.FolderSeparator, " / "), rank
}
