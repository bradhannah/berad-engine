package browser

import (
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/worldstate"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// The places column (increment 32): Home, World state and Settings, then
// each kind with its folders, and the saved views. It holds places, never
// the things in them, which the main pane shows for the place picked.

// Places that are not a kind or a folder.
const (
	placeHome    = "place:home"
	placeMatches = "place:matches"
	// treePrefix keeps the column's folding apart from the main pane's.
	treePrefix = "tree:"
	// toggleKind starts what a script finds by its disclosure triangle.
	toggleKind = "toggle"
	// placeWidth is the column's width, at most half the window's.
	placeWidth = 240
)

// placeKinds lists the kinds the column shows, in its order.
func placeKinds() []string {
	return []string{
		sectionZones, sectionWorkflows, sectionNPCs, sectionAudio, sectionSprites, sectionPalettes, sectionTemplates,
	}
}

// placeCounts says how many things each place holds, counting what its
// folders hold: all of them, and those the search and ticks keep.
type placeCounts struct {
	all, kept map[string]int
}

// countUnit is what a place counts as one: a thing, or a zone with its
// floors, which is kept when the zone or any of its floors is.
type countUnit struct {
	keys []string
	kept bool
}

// countPlaces counts each kind's and each folder's things, a zone and its
// floors as one; Matches counts every thing kept.
func (s *State) countPlaces(docs frame.Docs, things []thing) placeCounts {
	counted := placeCounts{all: map[string]int{}, kept: map[string]int{}}
	loose := s.looseFloors(docs)
	zoneOfFloor := map[string]string{}
	for _, entry := range s.entries(docs, zone.FormatName) {
		for _, floorID := range s.floorsOf(docs, entry.ID) {
			zoneOfFloor[floorID.String()] = entry.ID.String()
		}
	}
	units := map[string]*countUnit{}
	for _, found := range things {
		if found.extra {
			continue
		}
		kept := s.kept(found, "")
		if kept {
			counted.kept[placeMatches]++
		}
		unitID, keys := found.row.ID, placesOf(found, loose)
		if zoneID, inZone := zoneOfFloor[found.row.ID]; inZone {
			unitID, keys = zoneID, nil
		}
		unit := units[unitID]
		if unit == nil {
			unit = &countUnit{keys: nil, kept: false}
			units[unitID] = unit
		}
		if keys != nil {
			unit.keys = keys
		}
		unit.kept = unit.kept || kept
	}
	for _, unit := range units {
		for _, key := range unit.keys {
			counted.all[key]++
			if unit.kept {
				counted.kept[key]++
			}
		}
	}

	return counted
}

// placesOf lists the places a thing is counted in: its kind, and each
// folder that holds it.
func placesOf(found thing, loose map[ids.ULID]bool) []string {
	kinds := found.values[facetKind]
	if len(kinds) == 0 {
		return nil
	}
	keys := []string{kindPlace(kinds[0])}
	if kinds[0] == floor.FormatName {
		if floorID, err := ids.Parse(found.row.ID); err == nil && loose[floorID] {
			keys = append(keys, sectionLoose)
		}
	}
	for _, key := range found.values[facetFolder] {
		format, path, _ := strings.Cut(key, ":")
		if format == kindSound {
			keys = append(keys, audioCategoryPrefix+path)

			continue
		}
		keys = append(keys, folderRowID(format, path))
	}

	return keys
}

// kindPlace returns the place of a kind of thing: floors are with zones.
func kindPlace(kind string) string {
	switch kind {
	case floor.FormatName:
		return sectionZones
	case kindSound:
		return sectionAudio
	}

	return formatSection(kind)
}

// looseFloors returns the floors no zone lists.
func (s *State) looseFloors(docs frame.Docs) map[ids.ULID]bool {
	listed := map[ids.ULID]bool{}
	for _, entry := range s.entries(docs, zone.FormatName) {
		for _, floorID := range s.floorsOf(docs, entry.ID) {
			listed[floorID] = true
		}
	}
	loose := map[ids.ULID]bool{}
	for _, entry := range s.entries(docs, floor.FormatName) {
		if !listed[entry.ID] {
			loose[entry.ID] = true
		}
	}

	return loose
}

// placeRows lists the column: Home, Matches while narrowed, World state
// and Settings, each kind with its folders, then the saved views. While
// narrowed, only kinds and folders holding something kept are listed, each
// saying how many of all it holds.
func (s *State) placeRows(docs frame.Docs, index *project.Index, counted placeCounts) []ui.Row {
	narrowed := s.narrowed()
	rows := []ui.Row{placeRow(placeHome, "Home", ui.IconHome, "")}
	if narrowed {
		rows = append(rows, placeRow(placeMatches, "Matches", ui.IconFolder, strconv.Itoa(counted.kept[placeMatches])))
	}
	rows = append(rows, placeRow(sectionWorldState, worldstate.Name, ui.IconSettings, ""),
		placeRow(settingsRow, "Settings", ui.IconSettings, ""))
	for _, kind := range placeKinds() {
		rows = append(rows, s.kindPlaces(index, kind, counted)...)
	}

	return append(rows, s.viewRows(docs)...)
}

// placeRow is a place's row.
func placeRow(id, label string, icon ui.Icon, detail string) ui.Row {
	row := ui.NewRow(id, label)
	row.Icon, row.Detail = icon, detail

	return row
}

// countLabel says how many a place holds, or how many of them are kept
// while narrowed.
func (s *State) countLabel(counted placeCounts, key string) string {
	if s.narrowed() {
		return strconv.Itoa(counted.kept[key]) + " of " + strconv.Itoa(counted.all[key])
	}

	return strconv.Itoa(counted.all[key])
}

// shownPlace reports whether a place is listed: always, or while narrowed
// only when it holds something kept.
func (s *State) shownPlace(counted placeCounts, key string) bool {
	return !s.narrowed() || counted.kept[key] > 0
}

// kindPlaces lists a kind and, when it is open, its folders: the audio
// library's categories for sounds, and the floors in no zone for zones.
func (s *State) kindPlaces(index *project.Index, kind string, counted placeCounts) []ui.Row {
	if !s.shownPlace(counted, kind) {
		return nil
	}
	label, icon := kindLabel(kind)
	row := placeRow(kind, label, icon, s.countLabel(counted, kind))
	row.Strong, row.Add = true, sectionNew(kind) != ""
	children := s.folderPlaces(index, kind, "", 1, counted)
	if len(children) == 0 {
		return []ui.Row{row}
	}
	row.Toggle = ui.DisclosureOpen
	if s.closed[treePrefix+kind] && !s.narrowed() {
		row.Toggle = ui.DisclosureClosed

		return []ui.Row{row}
	}

	return append([]ui.Row{row}, children...)
}

// kindLabel names a kind's place and gives its icon.
func kindLabel(kind string) (string, ui.Icon) {
	switch kind {
	case sectionAudio:
		return "Sounds", ui.IconSound
	case sectionZones:
		return "Zones", ui.IconZone
	}
	info, _ := kindInfoOf(sectionFormat(kind))

	return info.many, info.icon
}

// folderPlaces lists the folders inside one, each before the folders it
// holds, at a depth; a kind's top lists its own folders.
func (s *State) folderPlaces(
	index *project.Index, kind, parent string, depth int, counted placeCounts,
) []ui.Row {
	switch kind {
	case sectionAudio:
		return s.categoryPlaces(counted)
	case sectionZones:
		rows := s.formatFolderPlaces(index, zone.FormatName, parent, depth, counted)
		if parent == "" && counted.all[sectionLoose] > 0 && s.shownPlace(counted, sectionLoose) {
			loose := placeRow(sectionLoose, "Floors in no zone", ui.IconFolder, s.countLabel(counted, sectionLoose))
			loose.Indent = depth
			rows = append(rows, loose)
		}

		return rows
	}

	return s.formatFolderPlaces(index, sectionFormat(kind), parent, depth, counted)
}

// formatFolderPlaces lists a document type's folders inside one, nested.
func (s *State) formatFolderPlaces(
	index *project.Index, format, parent string, depth int, counted placeCounts,
) []ui.Row {
	var rows []ui.Row
	folders := index.Folders(format)
	for _, path := range folders {
		if project.FolderParent(path) != parent {
			continue
		}
		rowID := folderRowID(format, path)
		if !s.shownPlace(counted, rowID) && !s.holdsPlace(path, folders, format, counted) {
			continue
		}
		row := placeRow(rowID, project.FolderName(path), ui.IconFolder, s.countLabel(counted, rowID))
		row.Indent = depth
		inner := s.formatFolderPlaces(index, format, path, depth+1, counted)
		if len(inner) > 0 {
			row.Toggle = ui.DisclosureOpen
			if s.closed[treePrefix+rowID] && !s.narrowed() {
				row.Toggle, inner = ui.DisclosureClosed, nil
			}
		}
		rows = append(rows, row)
		rows = append(rows, inner...)
	}

	return rows
}

// holdsPlace reports whether a folder holds a folder still listed while
// narrowed, so the path to it stays.
func (s *State) holdsPlace(path string, folders []string, format string, counted placeCounts) bool {
	return slices.ContainsFunc(folders, func(other string) bool {
		return other != path && project.Within(other, path) && counted.kept[folderRowID(format, other)] > 0
	})
}

// categoryPlaces lists the audio library's categories, the sounds' folders.
func (s *State) categoryPlaces(counted placeCounts) []ui.Row {
	var rows []ui.Row
	for _, category := range audioCategories() {
		id := audioCategoryPrefix + category[0]
		if counted.all[id] == 0 || !s.shownPlace(counted, id) {
			continue
		}
		row := placeRow(id, category[1], ui.IconFolder, s.countLabel(counted, id))
		row.Indent = 1
		rows = append(rows, row)
	}

	return rows
}

// layoutPlaces places the column and lists it.
func (s *State) layoutPlaces(docs frame.Docs, index *project.Index, counted placeCounts, content image.Rectangle) int {
	width := min(placeWidth, content.Dx()/ui.Half)
	s.places.Rect = image.Rect(0, toolbarHeight, width, content.Max.Y)
	s.places.Rows = s.placeRows(docs, index, counted)
	s.places.Selected = s.place

	return width
}

// updatePlaces handles the column: a click picks a place or applies a
// saved view, a triangle folds a kind or a folder, a [+] makes a thing of
// its kind, and a right-click opens a place's menu.
func (s *State) updatePlaces(input *ui.Input, ctx *frame.Context) {
	result := s.places.Update(input)
	if result.Toggled != "" {
		s.closed[treePrefix+result.Toggled] = !s.closed[treePrefix+result.Toggled]
	}
	if result.Added != "" {
		if kindOfPlace(s.place) != result.Added {
			s.show(result.Added)
		}
		s.create(ctx, sectionNew(result.Added))
	}
	if result.Context {
		s.pick(result.ContextRow)
		s.rightClicked(result.ContextRow, input.Cursor)
	}
	chosen := s.places.Selected
	s.places.Selected = s.place
	if !result.Changed || chosen == s.place {
		return
	}
	s.focus.Set(nil)
	switch {
	case strings.HasPrefix(chosen, viewPrefix):
		s.applyView(ctx.Docs, strings.TrimPrefix(chosen, viewPrefix))
	case chosen == sectionViews:
	case s.named != "" && s.Unsaved(ctx.Docs) > 0:
		s.guard.Ask(s.Unsaved(ctx.Docs), s.UnsavedName(ctx.Docs), func() { s.pick(chosen) })
	default:
		s.pick(chosen)
	}
}

// show shows a place in the main pane, from its top.
func (s *State) show(place string) {
	if place != "" {
		s.place = place
		s.list.ScrollToTop()
	}
}

// pick shows a place in the main pane. A folder, a kind or Settings is
// selected too, so the inspector shows it and new things land in it.
func (s *State) pick(place string) {
	if place == "" {
		return
	}
	s.show(place)
	s.list.Selected = ""
	_, isFolder := parseFolderRow(place)
	if isFolder || place == settingsRow || strings.HasPrefix(place, sectionPrefix) ||
		strings.HasPrefix(place, audioCategoryPrefix) {
		s.list.Selected = place
	}
}

// placeFor returns the place holding a document: its folder, its type's
// top, or for a floor its zone's place.
func (s *State) placeFor(docs frame.Docs, index *project.Index, entry project.Entry) (string, bool) {
	if entry.Format != floor.FormatName {
		return folderPlace(entry.Format, entry.Folder), formatSection(entry.Format) != ""
	}
	zoneID, inZone := s.zoneOf(docs, entry.ID)
	if !inZone {
		return sectionLoose, true
	}
	delete(s.closed, zoneID.String())
	zoneEntry, _ := index.Entry(zoneID)

	return folderPlace(zone.FormatName, zoneEntry.Folder), true
}

// folderPlace returns the place of a folder of a type, or of the type's
// top for "".
func folderPlace(format, path string) string {
	if path == "" {
		return formatSection(format)
	}

	return folderRowID(format, path)
}

// placeLabel names the place picked, for the main pane's heading and
// scripts.
func (s *State) placeLabel() string {
	for _, row := range s.places.Rows {
		if row.ID == s.place {
			return row.Label
		}
	}

	return ""
}

// placeParent returns the place that holds the one picked: a folder's
// parent, or its kind; "" at the top.
func placeParent(place string) string {
	if folder, isFolder := parseFolderRow(place); isFolder {
		return folderPlace(folder.Format, project.FolderParent(folder.Path))
	}
	if strings.HasPrefix(place, audioCategoryPrefix) {
		return sectionAudio
	}
	if place == sectionLoose {
		return sectionZones
	}

	return ""
}

// reveal shows the place that holds a document and selects it, as Home's
// rows do; narrowing that would hide it ends.
func (s *State) reveal(docs frame.Docs, documentID ids.ULID) bool {
	index := s.index(docs)
	entry, found := index.Entry(documentID)
	if !found {
		return false
	}
	place, known := s.placeFor(docs, index, entry)
	if !known {
		return false
	}
	if s.narrowed() {
		s.clearNarrowing()
		s.wasNarrowed = false
	}
	s.show(place)
	s.list.Selected = entry.ID.String()
	s.list.Reveal(s.list.Selected)

	return true
}

// locatePlace finds a place by its name, or its triangle.
func (s *State) locatePlace(kind, label string) (image.Point, bool) {
	for _, row := range s.places.Rows {
		if row.Label != label || row.Heading {
			continue
		}
		s.places.Reveal(row.ID)
		if kind == toggleKind {
			return s.places.LocateToggle(row.ID)
		}

		return s.places.Locate(row.ID)
	}

	return image.Pt(0, 0), false
}

// revealNamed shows, for a script, the place holding a thing named label:
// a document, a sound, or World state's Values, Signals or Constants,
// leaving the selection as it was. It reports whether it showed another
// place.
func (s *State) revealNamed(docs frame.Docs, label string) bool {
	index := s.index(docs)
	for _, entry := range index.Entries() {
		if entry.Name != label {
			continue
		}
		if place, known := s.placeFor(docs, index, entry); known && place != s.place {
			s.show(place)

			return true
		}
	}
	if s.audio.library != nil {
		for _, cue := range s.audio.library.Cues {
			if cue.Name == label && s.place != audioCategoryPrefix+cue.Category {
				s.show(audioCategoryPrefix + cue.Category)

				return true
			}
		}
	}
	if (label == "Values" || label == "Signals" || label == constantsLabel) && s.place != sectionWorldState {
		s.show(sectionWorldState)

		return true
	}

	return false
}

// kindOfPlace returns the kind a place belongs to: a kind, or a folder's or
// a category's kind; "" for the others.
func kindOfPlace(place string) string {
	if folder, isFolder := parseFolderRow(place); isFolder {
		return formatSection(folder.Format)
	}
	if strings.HasPrefix(place, audioCategoryPrefix) {
		return sectionAudio
	}
	if place == sectionLoose {
		return sectionZones
	}
	if slices.Contains(placeKinds(), place) {
		return place
	}

	return ""
}

// dropOnPlace finds the place a thing dragged out of the main pane would
// drop on: a folder, or a kind's top.
func (s *State) dropOnPlace(index *project.Index, dragged string, at image.Point) string {
	if !at.In(s.places.Rect) {
		return ""
	}
	for position, row := range s.places.Rows {
		if at.In(s.places.RowRect(position)) && row.ID != dragged && s.canDrop(index, dragged, row.ID) {
			return row.ID
		}
	}

	return ""
}
