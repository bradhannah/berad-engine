package browser

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/workflowview"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// The facets (increment 30). Every property things in the project have is a
// facet, each value with how many things have it. Ticking values narrows
// what is listed to what has them: any value ticked within a facet, and
// every facet ticked. A value nothing left has is not offered, so a tick
// never leads nowhere. Increment 32 offers them from the bar.

// How the facets are offered.
const (
	// facetShown is how many values a facet lists before "N more…".
	facetShown = 6
	// checksPerFrame is how many workflows' problems are looked at again
	// each frame, in turn.
	checksPerFrame = 8
)

// Facets, by the keys saved views keep.
const (
	facetKind     = "kind"
	facetZone     = "zone"
	facetRuns     = "runs"
	facetFolder   = "folder"
	facetTags     = "tags"
	facetChanged  = "changed"
	facetProblems = "problems"
)

// Row IDs: a facet's heading, a value as "tick:<facet>:<value>", a facet's
// "N more…", and a saved view.
const (
	facetPrefix  = "facet:"
	tickPrefix   = "tick:"
	morePrefix   = "more:"
	viewPrefix   = "view:"
	sectionViews = facetPrefix + "views"
)

// The Changed facet's values. Each holds the ones before it: a thing changed
// today changed this week too.
const (
	changedToday = "today"
	changedWeek  = "week"
	changedMonth = "month"
	changedOlder = "older"
	weekDays     = 6
	monthDays    = 29
)

// runsCalled is the Runs as value of a workflow other workflows call.
const runsCalled = "called"

// kindSound is the Kind value of a sound, which is a cue in the audio
// library rather than a document.
const kindSound = "sound"

// facetDef is a facet: its key and its heading.
type facetDef struct {
	id, label string
}

// facetDefs lists the facets in the order the bar shows them.
func facetDefs() []facetDef {
	return []facetDef{
		{facetKind, "Kind"}, {facetZone, zoneWord}, {facetRuns, "Runs as"}, {facetFolder, "Folder"},
		{facetTags, "Tags"}, {facetChanged, "Changed"}, {facetProblems, "Problems"},
	}
}

// kindInfo is how a kind of thing is named and shown.
type kindInfo struct {
	many, one string
	icon      ui.Icon
}

// kindOrder lists the kinds the table lists, in the Kind facet's order.
func kindOrder() []string {
	return []string{
		zone.FormatName, floor.FormatName, flow.FormatName, npc.FormatName, art.SpriteFormatName,
		art.PaletteFormatName, artplugin.TemplateFormatName, kindSound,
	}
}

// zoneWord names a zone, alone or as a column or a facet.
const zoneWord = "Zone"

// kindInfoOf names a kind the table lists, as the inspector names it, and
// whether the table lists that kind.
func kindInfoOf(format string) (kindInfo, bool) {
	many, listed := map[string]string{
		zone.FormatName: "Zones", floor.FormatName: floorsLabel, flow.FormatName: "Workflows", npc.FormatName: "NPCs",
		art.SpriteFormatName: "Sprites", art.PaletteFormatName: "Palettes",
		artplugin.TemplateFormatName: "Templates", kindSound: "Sounds",
	}[format]
	if format == kindSound {
		return kindInfo{many: many, one: "Sound", icon: ui.IconSound}, true
	}
	about := kindOf(format)

	return kindInfo{many: many, one: about.label, icon: about.icon}, listed
}

// Table columns, as indexes into a thing's cells.
const (
	columnKind = iota
	columnZone
	columnFolder
	columnUsedBy
	columnChanged
	columnCount
)

// thing is one thing the table can list: a document, a sound, or a row
// such as Settings that only a search finds.
type thing struct {
	row     ui.Row
	matches bool
	// extra is true for a row only a search finds, which no facet holds.
	extra  bool
	values map[string][]string
	cells  [columnCount]string
}

// facetValue is how a facet's value is shown: its label, its icon, and,
// in a facet whose order is fixed, its place.
type facetValue struct {
	label string
	icon  ui.Icon
	rank  int
}

// facetLabels holds every facet's values' labels, by facet and value.
type facetLabels map[string]map[string]facetValue

func (l facetLabels) add(facet, value, label string, icon ui.Icon, rank int) {
	if l[facet] == nil {
		l[facet] = map[string]facetValue{}
	}
	if _, known := l[facet][value]; !known {
		l[facet][value] = facetValue{label: label, icon: icon, rank: rank}
	}
}

// things lists what the table can show, by name, with the facet values each
// has, and every value's label.
func (s *State) things(docs frame.Docs) ([]thing, facetLabels) {
	index := s.index(docs)
	labels := facetLabels{}
	match := newMatcher(s.search.Field.Value())
	now := time.Now()
	var found []thing
	for _, entry := range index.Entries() {
		if _, listed := kindInfoOf(entry.Format); listed {
			found = append(found, s.documentThing(docs, index, entry, match, now, labels))
		}
	}
	found = append(found, s.soundThings(match, labels)...)
	found = append(found, s.searchedThings(docs, match)...)
	found = append(found, s.constantThings(docs, match)...)
	found = append(found, s.lineThings(docs, match)...)
	slices.SortStableFunc(found, func(a, b thing) int {
		return strings.Compare(strings.ToLower(a.row.Label), strings.ToLower(b.row.Label))
	})

	return found, labels
}

// documentThing is a document's thing.
func (s *State) documentThing(
	docs frame.Docs, index *project.Index, entry project.Entry, match matcher, now time.Time, labels facetLabels,
) thing {
	info, _ := kindInfoOf(entry.Format)
	row := ui.NewRow(entry.ID.String(), entry.Name)
	row.Icon, row.Tree = info.icon, true
	if entry.Format == flow.FormatName && entry.Kind == string(flow.PurposeZoneLoop) {
		row.Icon = ui.IconZoneLoop
	}
	found := thing{
		row: row, matches: match.entry(entry), extra: false, values: map[string][]string{}, cells: [columnCount]string{},
	}
	found.values[facetKind] = []string{entry.Format}
	labels.add(facetKind, entry.Format, info.many, info.icon, slices.Index(kindOrder(), entry.Format))
	found.cells[columnKind] = info.one
	zones := s.zonesOf(docs, entry)
	zoneNames := make([]string, 0, len(zones))
	for _, zoneID := range zones {
		name := index.Name(zoneID, "a deleted zone")
		found.values[facetZone] = append(found.values[facetZone], zoneID.String())
		labels.add(facetZone, zoneID.String(), name, ui.IconZone, 0)
		zoneNames = append(zoneNames, name)
	}
	found.cells[columnZone] = strings.Join(zoneNames, ", ")
	if entry.Format == flow.FormatName {
		s.workflowFacets(entry, &found, labels)
	}
	folderFacets(entry, info, &found, labels)
	for _, tag := range entry.Tags {
		key := strings.ToLower(tag)
		found.values[facetTags] = append(found.values[facetTags], key)
		labels.add(facetTags, key, "#"+tag, ui.IconNone, 0)
	}
	found.values[facetChanged] = changedValues(entry.Changed, now)
	for rank, named := range changedLabels() {
		labels.add(facetChanged, named[0], named[1], ui.IconNone, rank)
	}
	found.cells[columnChanged] = changedCell(entry.Changed, now)

	return found
}

// workflowFacets adds a workflow's kind, whether others call it, its
// problems and what uses it.
func (s *State) workflowFacets(entry project.Entry, found *thing, labels facetLabels) {
	purpose := flow.Purpose(entry.Kind)
	if !purpose.Valid() {
		purpose = flow.PurposeBehaviour
	}
	icon := ui.IconWorkflow
	if purpose == flow.PurposeZoneLoop {
		icon = ui.IconZoneLoop
	}
	found.values[facetRuns] = []string{string(purpose)}
	labels.add(facetRuns, string(purpose), purpose.Label(), icon, slices.Index(flow.Purposes(), purpose))
	found.cells[columnKind] = purpose.Label()
	check, checked := s.checks[entry.ID]
	if !checked {
		return
	}
	if check.Called() {
		found.values[facetRuns] = append(found.values[facetRuns], runsCalled)
		labels.add(facetRuns, runsCalled, "Called by workflows", ui.IconWorkflow, len(flow.Purposes()))
	}
	for _, family := range check.Families() {
		found.values[facetProblems] = append(found.values[facetProblems], family)
		labels.add(facetProblems, family, family, ui.IconNone, 0)
	}
	found.cells[columnUsedBy] = check.UsedBy()
}

// folderFacets adds the folder a document is in and every folder holding
// that one, so ticking a folder keeps what its folders hold too.
func folderFacets(entry project.Entry, info kindInfo, found *thing, labels facetLabels) {
	found.cells[columnFolder] = strings.ReplaceAll(entry.Folder, project.FolderSeparator, " / ")
	for path := entry.Folder; path != ""; path = project.FolderParent(path) {
		key := entry.Format + ":" + path
		found.values[facetFolder] = append(found.values[facetFolder], key)
		labels.add(facetFolder, key, strings.ReplaceAll(path, project.FolderSeparator, " / "), info.icon, 0)
	}
}

// zonesOf returns the zones a document belongs to: a zone itself, a floor's
// zone, the zone an NPC lives in, and the zone a workflow runs in.
func (s *State) zonesOf(docs frame.Docs, entry project.Entry) []ids.ULID {
	var found ids.ULID
	switch entry.Format {
	case zone.FormatName:
		found = entry.ID
	case floor.FormatName:
		found, _ = s.zoneOf(docs, entry.ID)
	case npc.FormatName:
		if person, loaded := s.npcOf(docs, entry.ID); loaded {
			found = person.Zone
		}
	case flow.FormatName:
		docs.View(entry.ID, func(document client.Document) {
			if workflowDoc, ok := document.Model.(*workflowplugin.Document); ok {
				found = workflowDoc.Workflow.RunsIn()
			}
		})
	}
	if found.IsZero() {
		return nil
	}

	return []ids.ULID{found}
}

// soundThings are the audio library's cues, each with its category as its
// folder.
func (s *State) soundThings(match matcher, labels facetLabels) []thing {
	if s.audio.library == nil {
		return nil
	}
	info, _ := kindInfoOf(kindSound)
	found := make([]thing, 0, len(s.audio.library.Cues))
	for _, cue := range s.audio.library.Cues {
		row := ui.NewRow(audioCuePrefix+cue.ID, cue.Name)
		row.Icon, row.Tree = ui.IconSound, true
		each := thing{
			row: row, matches: match.cue(cue), extra: false, values: map[string][]string{}, cells: [columnCount]string{},
		}
		each.values[facetKind] = []string{kindSound}
		labels.add(facetKind, kindSound, info.many, info.icon, slices.Index(kindOrder(), kindSound))
		each.cells[columnKind], each.cells[columnFolder] = info.one, categoryLabel(cue)
		key := kindSound + ":" + cue.Category
		each.values[facetFolder] = []string{key}
		labels.add(facetFolder, key, categoryLabel(cue), ui.IconSound, 0)
		found = append(found, each)
	}

	return found
}

// categoryLabel names a cue's category.
func categoryLabel(cue protocol.AudioCue) string {
	for _, category := range audioCategories() {
		if category[0] == cue.Category {
			return category[1]
		}
	}

	return cue.Category
}

// searchedThings are the rows that are not documents, such as Settings and
// World state's parts, which a search finds by their own words.
func (s *State) searchedThings(docs frame.Docs, match matcher) []thing {
	if !match.active() {
		return nil
	}
	var found []thing
	for _, row := range append(s.settingsRows(docs, match), s.worldStateRows(docs, match)...) {
		if row.Section || row.Heading {
			continue
		}
		row.Indent, row.Tree = 0, true
		found = append(found, thing{row: row, matches: true, extra: true, values: nil, cells: [columnCount]string{}})
	}

	return found
}

// changedValues are the Changed values of a time: today, this week, this
// month, each holding the ones before it, or older, which an unknown time is.
func changedValues(changed, now time.Time) []string {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch {
	case changed.IsZero() || changed.Before(today.AddDate(0, 0, -monthDays)):
		return []string{changedOlder}
	case changed.Before(today.AddDate(0, 0, -weekDays)):
		return []string{changedMonth}
	case changed.Before(today):
		return []string{changedWeek, changedMonth}
	default:
		return []string{changedToday, changedWeek, changedMonth}
	}
}

// changedLabels names the Changed values, in order.
func changedLabels() [][2]string {
	return [][2]string{
		{changedToday, "Today"}, {changedWeek, "This week"}, {changedMonth, "This month"}, {changedOlder, "Older"},
	}
}

// changedCell says when a thing changed: today, yesterday, a day of this
// week, or the date; "" when not known.
func changedCell(changed, now time.Time) string {
	if changed.IsZero() {
		return ""
	}
	changed = changed.In(now.Location())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch {
	case !changed.Before(today):
		return "today"
	case !changed.Before(today.AddDate(0, 0, -1)):
		return "yesterday"
	case !changed.Before(today.AddDate(0, 0, -weekDays)):
		return changed.Weekday().String()
	case changed.Year() == now.Year():
		return changed.Format("2 Jan")
	default:
		return changed.Format("2 Jan 2006")
	}
}

// kept reports whether a thing is listed: the search finds it, and it has a
// ticked value in every facet ticked, but the one left out.
func (s *State) kept(found thing, except string) bool {
	if !found.matches {
		return false
	}
	for facet, ticked := range s.ticks {
		if facet == except || len(ticked) == 0 {
			continue
		}
		if !slices.ContainsFunc(found.values[facet], func(value string) bool { return ticked[value] }) {
			return false
		}
	}

	return true
}

// counts returns how many things each of a facet's values would list:
// those the search and every other facet keep.
func (s *State) counts(things []thing, facet string) map[string]int {
	counted := map[string]int{}
	for _, found := range things {
		if s.kept(found, facet) {
			for _, value := range found.values[facet] {
				counted[value]++
			}
		}
	}

	return counted
}

// ticked reports whether any value is ticked.
func (s *State) ticked() bool {
	for _, values := range s.ticks {
		if len(values) > 0 {
			return true
		}
	}

	return false
}

// tick ticks a facet's value, or takes the tick away.
func (s *State) tick(facet, value string, on bool) {
	if s.ticks[facet] == nil {
		s.ticks[facet] = map[string]bool{}
	}
	if on {
		s.ticks[facet][value] = true
	} else {
		delete(s.ticks[facet], value)
	}
}

// facetGroup lists one facet: its heading, then its values, most common
// first or in the facet's own order, the first few unless it shows all.
func (s *State) facetGroup(def facetDef, counts map[string]int, labels map[string]facetValue) []ui.Row {
	var values []string
	for value, count := range counts {
		if count > 0 {
			values = append(values, value)
		}
	}
	for value := range s.ticks[def.id] {
		if counts[value] == 0 {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return nil
	}
	heading := s.section(facetPrefix+def.id, def.label, 0, false, false)
	heading.Detail = ""
	rows := []ui.Row{heading}
	if s.closed[heading.ID] {
		return rows
	}
	s.sortValues(def.id, values, counts, labels)
	shown, hidden := values, 0
	if !s.more[def.id] && len(values) > facetShown {
		shown = values[:facetShown]
		for _, value := range values[facetShown:] {
			if s.ticks[def.id][value] {
				shown = append(shown, value)
			}
		}
		hidden = len(values) - len(shown)
	}
	for _, value := range shown {
		rows = append(rows, s.valueRow(def.id, value, counts[value], labels))
	}

	return append(rows, moreRow(def.id, hidden, s.more[def.id] && len(values) > facetShown)...)
}

// sortValues orders a facet's values: Kind and Changed in their own order,
// others most common first, then by name.
func (s *State) sortValues(facet string, values []string, counts map[string]int, labels map[string]facetValue) {
	fixed := facet == facetKind || facet == facetChanged || facet == facetRuns
	slices.SortFunc(values, func(first, second string) int {
		if fixed {
			return cmp.Compare(labels[first].rank, labels[second].rank)
		}

		return cmp.Or(cmp.Compare(counts[second], counts[first]),
			strings.Compare(strings.ToLower(labels[first].label), strings.ToLower(labels[second].label)))
	})
}

// valueRow is a facet value's row: its icon, its label, how many things it
// would list, and a tick when ticked.
func (s *State) valueRow(facet, value string, count int, labels map[string]facetValue) ui.Row {
	shown, known := labels[value]
	if !known {
		shown = facetValue{label: value, icon: ui.IconNone, rank: 0}
	}
	row := ui.NewRow(tickPrefix+facet+":"+value, shown.label)
	row.Icon, row.Tree, row.Detail, row.Ticked = shown.icon, true, strconv.Itoa(count), s.ticks[facet][value]

	return row
}

// moreRow offers a facet's hidden values, or to show fewer again.
func moreRow(facet string, hidden int, fewer bool) []ui.Row {
	switch {
	case hidden > 0:
		row := ui.NewRow(morePrefix+facet, strconv.Itoa(hidden)+" more…")
		row.Tree, row.Muted = true, true

		return []ui.Row{row}
	case fewer:
		row := ui.NewRow(morePrefix+facet, "Fewer")
		row.Tree, row.Muted = true, true

		return []ui.Row{row}
	}

	return nil
}

// updateChecks keeps a problem check for each workflow, and looks at a few
// again each frame, in turn.
func (s *State) updateChecks(ctx *frame.Context) {
	workflows := s.entries(ctx.Docs, flow.FormatName)
	listed := map[ids.ULID]bool{}
	for _, entry := range workflows {
		listed[entry.ID] = true
		if _, checked := s.checks[entry.ID]; !checked {
			s.checks[entry.ID] = workflowview.NewCheck(entry.ID, s.theme)
		}
	}
	for workflowID := range s.checks {
		if !listed[workflowID] {
			delete(s.checks, workflowID)
		}
	}
	for range min(checksPerFrame, len(workflows)) {
		s.checkAt = (s.checkAt + 1) % len(workflows)
		s.checks[workflows[s.checkAt].ID].Update(ctx)
	}
}
