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
	}[section]
}

// newSection returns the section whose [+] creates an item, or "".
func newSection(item string) string {
	for _, section := range []string{sectionZones, sectionWorkflows, sectionNPCs, sectionText} {
		if sectionNew(section) == item {
			return section
		}
	}

	return ""
}

// rows lists the project as a tree: a section for each type with a count
// and a [+], zones with their floors (top floor first), and floors in no
// zone. Each document row has its type's icon and a short detail.
func (s *State) rows(docs frame.Docs) []ui.Row {
	index := s.index(docs)
	listed := map[ids.ULID]bool{}
	zones := index.OfFormat(zone.FormatName)
	rows := []ui.Row{s.section(sectionZones, "Zones", len(zones), true)}
	for _, entry := range zones {
		rows = append(rows, s.zoneRows(docs, index, entry, listed)...)
	}
	uses := s.uses(docs, index)
	workflows := index.OfFormat(flow.FormatName)
	rows = append(rows, s.section(sectionWorkflows, "Workflows", len(workflows), true))
	for _, entry := range workflows {
		detail := ""
		if count := uses[entry.ID]; count > 0 {
			detail = fmt.Sprintf("used by %d %s", count, plural(count, "NPC", "NPCs"))
		}
		rows = append(rows, s.item(sectionWorkflows, entry, ui.IconWorkflow, detail))
	}
	npcs := index.OfFormat(npc.FormatName)
	rows = append(rows, s.section(sectionNPCs, "NPCs", len(npcs), true))
	for _, entry := range npcs {
		rows = append(rows, s.item(sectionNPCs, entry, ui.IconNPC, s.follows(docs, index, entry.ID)))
	}
	texts := index.OfFormat(textdoc.FormatName)
	rows = append(rows, s.section(sectionText, "Text", len(texts), true))
	for _, entry := range texts {
		rows = append(rows, s.item(sectionText, entry, ui.IconText, ""))
	}
	var loose []project.Entry
	for _, entry := range index.OfFormat(floor.FormatName) {
		if !listed[entry.ID] {
			loose = append(loose, entry)
		}
	}
	if len(loose) > 0 {
		rows = append(rows, s.section(sectionLoose, "Floors in no zone", len(loose), false))
		for _, entry := range loose {
			rows = append(rows, s.item(sectionLoose, entry, ui.IconFloor, ""))
		}
	}

	return withoutHidden(rows)
}

// hiddenRow marks a row inside a closed section, dropped by withoutHidden.
const hiddenRow = "hidden:"

// withoutHidden drops the rows of closed sections.
func withoutHidden(rows []ui.Row) []ui.Row {
	shown := rows[:0]
	for _, row := range rows {
		if !strings.HasPrefix(row.ID, hiddenRow) {
			shown = append(shown, row)
		}
	}

	return shown
}

// section returns a section heading with its count, open or closed.
func (s *State) section(id, label string, count int, add bool) ui.Row {
	row := ui.NewHeading(id, label)
	row.Section, row.Add = true, add
	row.Detail = strconv.Itoa(count)
	row.Toggle = ui.DisclosureOpen
	if s.closed[id] {
		row.Toggle = ui.DisclosureClosed
	}

	return row
}

// item returns a document's row in a section, hidden when the section is
// closed.
func (s *State) item(section string, entry project.Entry, icon ui.Icon, detail string) ui.Row {
	row := ui.NewRow(entry.ID.String(), entry.Name)
	row.Icon, row.Detail = icon, detail
	if s.closed[section] {
		row.ID = hiddenRow + row.ID
	}

	return row
}

// zoneRows returns a zone's row and, when it is open, its floors' rows with
// tree lines, the top floor first and marked "top".
func (s *State) zoneRows(
	docs frame.Docs, index *project.Index, entry project.Entry, listed map[ids.ULID]bool,
) []ui.Row {
	floors := s.floorsOf(docs, entry.ID)
	zoneRow := s.item(sectionZones, entry, ui.IconZone, fmt.Sprintf("%d %s", len(floors), plural(len(floors), "floor",
		"floors")))
	zoneRow.Strong = true
	open := !s.closed[entry.ID.String()]
	if len(floors) > 0 {
		zoneRow.Toggle = ui.DisclosureClosed
		if open {
			zoneRow.Toggle = ui.DisclosureOpen
		}
	}
	rows := []ui.Row{zoneRow}
	for position := len(floors) - 1; position >= 0; position-- {
		listed[floors[position]] = true
		if !open {
			continue
		}
		floorEntry := project.Entry{ID: floors[position], Name: index.Name(floors[position], "missing floor"),
			Format: floor.FormatName}
		detail := ""
		if position == len(floors)-1 && len(floors) > 1 {
			detail = "top"
		}
		floorRow := s.item(sectionZones, floorEntry, ui.IconFloor, detail)
		floorRow.Indent, floorRow.Guide = 1, ui.GuideMiddle
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
