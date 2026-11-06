package browser

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/art"
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
	sectionLoose     = sectionPrefix + "loose"
)

// sectionNew returns what a section's [+] creates, or "" for a section
// without one.
func sectionNew(section string) string {
	return map[string]string{
		sectionZones: newZone, sectionWorkflows: newWorkflow, sectionNPCs: newNPC,
		sectionAudio: newSound, sectionSprites: newSprite, sectionPalettes: newPalette,
		sectionTemplates: newTemplate,
	}[section]
}

// newSection returns the section whose [+] creates an item, or "".
func newSection(item string) string {
	for _, section := range []string{
		sectionZones, sectionWorkflows, sectionNPCs, sectionAudio, sectionSprites, sectionPalettes,
		sectionTemplates,
	} {
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
		sectionSprites: art.SpriteFormatName, sectionPalettes: art.PaletteFormatName,
		sectionTemplates: artplugin.TemplateFormatName,
	}[section]
}

// formatSection returns the section listing a document type, or "".
func formatSection(format string) string {
	for _, section := range []string{
		sectionZones, sectionWorkflows, sectionNPCs, sectionSprites, sectionPalettes, sectionTemplates,
	} {
		if sectionFormat(section) == format {
			return section
		}
	}

	return ""
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
				Kind: "", Changed: time.Time{},
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

// workflowRow is a workflow's row: its kind's icon, and how many NPCs
// follow it, or zones run it as their loop.
func workflowRow(entry project.Entry, count, depth int) ui.Row {
	icon, detail := ui.IconWorkflow, ""
	loop := entry.Kind == string(flow.PurposeZoneLoop)
	if loop {
		icon = ui.IconZoneLoop
	}
	switch {
	case count > 0 && loop:
		detail = fmt.Sprintf("loop of %d %s", count, plural(count, "zone", "zones"))
	case count > 0:
		detail = fmt.Sprintf("used by %d %s", count, plural(count, "NPC", "NPCs"))
	}

	return item(entry, icon, detail, depth)
}

// uses counts the NPCs following each behaviour and the zones running each
// zone loop. NPCs are read one at a time.
func (s *State) uses(docs frame.Docs, index *project.Index) map[ids.ULID]int {
	counts := map[ids.ULID]int{}
	for _, entry := range index.OfFormat(npc.FormatName) {
		if found, loaded := s.npcOf(docs, entry.ID); loaded && !found.Behaviour.IsZero() {
			counts[found.Behaviour]++
		}
	}
	for _, entry := range index.OfFormat(zone.FormatName) {
		docs.View(entry.ID, func(document client.Document) {
			if zoneDoc, ok := document.Model.(*zoneplugin.Document); ok && !zoneDoc.Zone.Loop.IsZero() {
				counts[zoneDoc.Zone.Loop]++
			}
		})
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
