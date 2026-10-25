package browser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// Sections of the list, by row ID: "section:" and the section's name.
const (
	sectionPrefix    = "section:"
	sectionZones     = sectionPrefix + "zones"
	sectionWorkflows = sectionPrefix + "workflows"
	sectionNPCs      = sectionPrefix + "npcs"
	sectionText      = sectionPrefix + "text"
	sectionLoose     = sectionPrefix + "loose"
)

// sectionNew returns what a section's [+] creates.
func sectionNew(section string) string {
	return map[string]string{
		sectionZones: newZone, sectionWorkflows: newWorkflow, sectionNPCs: newNPC, sectionText: newText,
		sectionAudio: newSound,
	}[section]
}

// newSection returns the section whose [+] creates an item, or "".
func newSection(item string) string {
	for _, section := range []string{sectionZones, sectionWorkflows, sectionNPCs, sectionText, sectionAudio} {
		if sectionNew(section) == item {
			return section
		}
	}

	return ""
}

// sectionFormat returns the document type a section lists, which its
// folders hold, or "" for the floors in no zone.
func sectionFormat(section string) string {
	return map[string]string{
		sectionZones: zone.FormatName, sectionWorkflows: flow.FormatName, sectionNPCs: npc.FormatName,
		sectionText: textdoc.FormatName,
	}[section]
}

// formatSection returns the section listing a document type, or "".
func formatSection(format string) string {
	for _, section := range []string{sectionZones, sectionWorkflows, sectionNPCs, sectionText} {
		if sectionFormat(section) == format {
			return section
		}
	}

	return ""
}

// rows lists the project as a tree: a section for each type with a count
// and a [+], its folders and documents, zones with their floors (top floor
// first), and floors in no zone. Each document row has its type's icon, a
// short detail and its tags. A search shows only what matches, with every
// section and folder open.
func (s *State) rows(docs frame.Docs) []ui.Row {
	index := s.index(docs)
	match := newMatcher(s.search.Field.Value())
	listed := map[ids.ULID]bool{}
	floors := map[ids.ULID][]ids.ULID{}
	for _, entry := range index.OfFormat(zone.FormatName) {
		floors[entry.ID] = s.floorsOf(docs, entry.ID)
		for _, floorID := range floors[entry.ID] {
			listed[floorID] = true
		}
	}
	zoneMatch := func(entry project.Entry) bool {
		if match.entry(entry) {
			return true
		}
		for _, floorID := range floors[entry.ID] {
			if found, ok := index.Entry(floorID); ok && match.entry(found) {
				return true
			}
		}

		return false
	}
	rows := s.typeRows(sectionZones, "Zones", index, zoneMatch, func(entry project.Entry, depth int) []ui.Row {
		return s.zoneRows(index, entry, floors[entry.ID], depth, match.active())
	})
	uses := s.uses(docs, index)
	rows = append(rows, s.typeRows(sectionWorkflows, "Workflows", index, match.entry,
		func(entry project.Entry, depth int) []ui.Row {
			detail := ""
			if count := uses[entry.ID]; count > 0 {
				detail = fmt.Sprintf("used by %d %s", count, plural(count, "NPC", "NPCs"))
			}

			return []ui.Row{item(entry, ui.IconWorkflow, detail, depth)}
		})...)
	rows = append(rows, s.typeRows(sectionNPCs, "NPCs", index, match.entry, func(entry project.Entry, depth int) []ui.Row {
		return []ui.Row{item(entry, ui.IconNPC, s.follows(docs, index, entry.ID), depth)}
	})...)
	rows = append(rows, s.typeRows(sectionText, "Text", index, match.entry, func(entry project.Entry, depth int) []ui.Row {
		return []ui.Row{item(entry, ui.IconText, "", depth)}
	})...)
	rows = append(rows, s.audioRows(match)...)

	return append(rows, s.looseRows(index, listed, match)...)
}

// looseRows lists the floors in no zone, if there are any.
func (s *State) looseRows(index *project.Index, listed map[ids.ULID]bool, match matcher) []ui.Row {
	var loose []project.Entry
	for _, entry := range index.OfFormat(floor.FormatName) {
		if !listed[entry.ID] && match.entry(entry) {
			loose = append(loose, entry)
		}
	}
	if len(loose) == 0 {
		return nil
	}
	rows := []ui.Row{s.section(sectionLoose, "Floors in no zone", len(loose), false, match.active())}
	if s.closed[sectionLoose] && !match.active() {
		return rows
	}
	for _, entry := range loose {
		rows = append(rows, item(entry, ui.IconFloor, "", 0))
	}

	return rows
}

// typeRows lists a section: its heading with a count, then its folders and
// documents as a tree, each folder before what it holds, folders before
// documents. Only documents shows accepts are listed while searching, in
// folders that hold one or whose own name matches.
func (s *State) typeRows(
	section, label string, index *project.Index, shows func(project.Entry) bool,
	documentRows func(project.Entry, int) []ui.Row,
) []ui.Row {
	format := sectionFormat(section)
	match := newMatcher(s.search.Field.Value())
	var shown []project.Entry
	for _, entry := range index.OfFormat(format) {
		if shows(entry) {
			shown = append(shown, entry)
		}
	}
	rows := []ui.Row{s.section(section, label, len(shown), true, match.active())}
	if s.closed[section] && !match.active() {
		return rows
	}
	folders := index.Folders(format)
	var walk func(parent string, depth int)
	walk = func(parent string, depth int) {
		for _, path := range folders {
			if project.FolderParent(path) != parent {
				continue
			}
			held := countWithin(shown, path)
			if match.active() && held == 0 && !match.folder(path) {
				continue
			}
			row := s.folderRow(format, path, held, depth, hasChildren(folders, index.OfFormat(format), path))
			rows = append(rows, row)
			if row.Toggle != ui.DisclosureClosed {
				walk(path, depth+1)
			}
		}
		for _, entry := range shown {
			if entry.Folder == parent {
				rows = append(rows, documentRows(entry, depth)...)
			}
		}
	}
	walk("", 0)

	return rows
}

// countWithin counts the documents in a folder or the folders it holds.
func countWithin(entries []project.Entry, path string) int {
	count := 0
	for _, entry := range entries {
		if entry.Folder != "" && project.Within(entry.Folder, path) {
			count++
		}
	}

	return count
}

// hasChildren reports whether a folder holds a folder or a document.
func hasChildren(folders []string, entries []project.Entry, path string) bool {
	for _, other := range folders {
		if project.FolderParent(other) == path {
			return true
		}
	}

	return countWithin(entries, path) > 0
}

// folderRow is a folder's row: its icon, its name, how many documents it
// holds, and a triangle when it holds anything. A search opens every folder.
func (s *State) folderRow(format, path string, held, depth int, children bool) ui.Row {
	id := folderRowID(format, path)
	row := ui.NewRow(id, project.FolderName(path))
	row.Icon, row.Indent, row.Detail = ui.IconFolder, depth, strconv.Itoa(held)
	if children {
		row.Toggle = ui.DisclosureOpen
		if s.closed[id] && !newMatcher(s.search.Field.Value()).active() {
			row.Toggle = ui.DisclosureClosed
		}
	}

	return row
}

// section returns a section heading with its count, open or closed; a
// search opens every section.
func (s *State) section(id, label string, count int, add, searching bool) ui.Row {
	row := ui.NewHeading(id, label)
	row.Section, row.Add = true, add
	row.Detail = strconv.Itoa(count)
	row.Toggle = ui.DisclosureOpen
	if s.closed[id] && !searching {
		row.Toggle = ui.DisclosureClosed
	}

	return row
}

// item returns a document's row, with its tags beside its name.
func item(entry project.Entry, icon ui.Icon, detail string, depth int) ui.Row {
	row := ui.NewRow(entry.ID.String(), entry.Name)
	row.Icon, row.Detail, row.Indent = icon, detail, depth
	if len(entry.Tags) > 0 {
		row.Aside = "#" + strings.Join(entry.Tags, " #")
	}

	return row
}

// zoneRows returns a zone's row and, when it is open, its floors' rows with
// tree lines, the top floor first and marked "top". A search opens it.
func (s *State) zoneRows(
	index *project.Index, entry project.Entry, floors []ids.ULID, depth int, searching bool,
) []ui.Row {
	zoneRow := item(entry, ui.IconZone, fmt.Sprintf("%d %s", len(floors), plural(len(floors), "floor", "floors")),
		depth)
	zoneRow.Strong = true
	open := !s.closed[entry.ID.String()] || searching
	if len(floors) > 0 {
		zoneRow.Toggle = ui.DisclosureClosed
		if open {
			zoneRow.Toggle = ui.DisclosureOpen
		}
	}
	rows := []ui.Row{zoneRow}
	if !open {
		return rows
	}
	for position := len(floors) - 1; position >= 0; position-- {
		floorEntry, found := index.Entry(floors[position])
		if !found {
			floorEntry = project.Entry{
				ID: floors[position], Name: "missing floor", Format: floor.FormatName, Folder: "", Tags: nil,
			}
		}
		detail := ""
		if position == len(floors)-1 && len(floors) > 1 {
			detail = "top"
		}
		floorRow := item(floorEntry, ui.IconFloor, detail, depth+1)
		floorRow.Guide = ui.GuideMiddle
		if position == 0 {
			floorRow.Guide = ui.GuideLast
		}
		rows = append(rows, floorRow)
	}

	return rows
}

// uses counts the NPCs following each workflow. NPCs are read one at a time.
func (s *State) uses(docs frame.Docs, index *project.Index) map[ids.ULID]int {
	counts := map[ids.ULID]int{}
	for _, entry := range index.OfFormat(npc.FormatName) {
		if found, loaded := s.npcOf(docs, entry.ID); loaded && !found.Behaviour.IsZero() {
			counts[found.Behaviour]++
		}
	}

	return counts
}

// follows says which workflow an NPC follows, for its row.
func (s *State) follows(docs frame.Docs, index *project.Index, npcID ids.ULID) string {
	found, loaded := s.npcOf(docs, npcID)
	switch {
	case !loaded:
		return ""
	case found.Behaviour.IsZero():
		return "no behaviour"
	default:
		return "follows " + index.Name(found.Behaviour, "a deleted workflow")
	}
}
