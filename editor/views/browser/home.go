package browser

import (
	"cmp"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// Home (increment 32): what the Project window opens on. What this person
// pinned, what needs attention, what they opened and changed lately by day,
// and the saved views. The orchestrator keeps what was opened and the pins
// for this person alone.

// Days Home groups recent things by.
const (
	dayToday     = "Today"
	dayYesterday = "Yesterday"
)

// What Home shows, and its rows.
const (
	homeRecentMost = 20
	homePrefix     = "home:"
	contextPin     = "context-pin"
	contextReveal  = "context-reveal"
)

// homeState is Home's memory, as the orchestrator shares it.
type homeState struct {
	opened []protocol.HomeOpen
	kept   protocol.HomeKept
}

func newHomeState() homeState {
	return homeState{
		opened: nil, kept: protocol.HomeKept{Pins: []ids.ULID{}, Folders: map[string]protocol.FolderChoice{}},
	}
}

// heardHome takes Home's memory as the orchestrator shares it.
func (s *State) heardHome(session *protocol.Session) {
	switch session.Topic {
	case protocol.TopicHomeOpened:
		var opened protocol.HomeOpened
		if json.Unmarshal(session.Value, &opened) == nil {
			s.home.opened = opened.Opened
		}
	case protocol.TopicHomeKept:
		var kept protocol.HomeKept
		if json.Unmarshal(session.Value, &kept) == nil {
			if kept.Pins == nil {
				kept.Pins = []ids.ULID{}
			}
			if kept.Folders == nil {
				kept.Folders = map[string]protocol.FolderChoice{}
			}
			s.home.kept = kept
		}
	}
}

// keepHome shares what this person keeps, for the orchestrator to save.
func (s *State) keepHome(ctx *frame.Context) {
	ctx.Requests.Publish(protocol.TopicHomeKept, s.home.kept)
}

// homeRows lists Home. A thing is listed once: pinned, then needing
// attention, then recent.
func (s *State) homeRows(docs frame.Docs, index *project.Index, now time.Time) []ui.Row {
	shown := map[ids.ULID]bool{}
	rows := s.pinnedRows(docs, index, shown)
	rows = append(rows, s.attentionRows(docs, index, shown)...)
	rows = append(rows, s.recentRows(docs, index, shown, now)...)
	rows = append(rows, s.homeViewRows(docs)...)
	if len(rows) == 0 {
		row := ui.NewRow(homePrefix+"empty", "Nothing opened or changed yet. Pick a place on the left.")
		row.Muted = true
		rows = append(rows, row)
	}

	return rows
}

// homeHeading is a heading on Home.
func homeHeading(id, label string, count int) ui.Row {
	row := ui.NewHeading(homePrefix+id, label)
	if count > 0 {
		row.Detail = strconv.Itoa(count)
	}

	return row
}

// homeRow is a document's row on Home: its icon and name, where it lives,
// and what is said of it.
func (s *State) homeRow(docs frame.Docs, index *project.Index, entry project.Entry, detail string) ui.Row {
	info, _ := kindInfoOf(entry.Format)
	row := ui.NewRow(entry.ID.String(), entry.Name)
	row.Icon, row.Detail, row.Aside = info.icon, detail, s.wherePath(docs, index, entry)
	if entry.Format == flow.FormatName && entry.Kind == string(flow.PurposeZoneLoop) {
		row.Icon = ui.IconZoneLoop
	}

	return row
}

// wherePath says where a document lives, as "Workflows / Harbour / Shops".
func (s *State) wherePath(docs frame.Docs, index *project.Index, entry project.Entry) string {
	if entry.Format == floor.FormatName {
		if zoneID, inZone := s.zoneOf(docs, entry.ID); inZone {
			return "Zones / " + index.Name(zoneID, "a zone")
		}

		return "Floors in no zone"
	}
	label, _ := kindLabel(formatSection(entry.Format))
	if entry.Folder == "" {
		return label
	}

	return label + " / " + strings.ReplaceAll(entry.Folder, project.FolderSeparator, " / ")
}

// listedOnHome reports whether a document can be listed on Home: one the
// places list, not a tile or a document every project holds.
func listedOnHome(entry project.Entry) bool {
	_, listed := kindInfoOf(entry.Format)

	return listed && (formatSection(entry.Format) != "" || entry.Format == floor.FormatName)
}

// pinnedRows lists the pinned things, in the order pinned.
func (s *State) pinnedRows(docs frame.Docs, index *project.Index, shown map[ids.ULID]bool) []ui.Row {
	var rows []ui.Row
	for _, pinned := range s.home.kept.Pins {
		entry, found := index.Entry(pinned)
		if !found || shown[pinned] {
			continue
		}
		shown[pinned] = true
		rows = append(rows, s.homeRow(docs, index, entry, "pinned"))
	}
	if len(rows) == 0 {
		return nil
	}

	return append([]ui.Row{homeHeading("pinned", "Pinned", len(rows))}, rows...)
}

// attentionRows lists the workflows the checks found problems in, each
// saying what kind.
func (s *State) attentionRows(docs frame.Docs, index *project.Index, shown map[ids.ULID]bool) []ui.Row {
	var rows []ui.Row
	for _, entry := range index.OfFormat(flow.FormatName) {
		check, checked := s.checks[entry.ID]
		if !checked || shown[entry.ID] || len(check.Families()) == 0 {
			continue
		}
		shown[entry.ID] = true
		rows = append(rows, s.homeRow(docs, index, entry, strings.Join(check.Families(), ", ")))
	}
	if len(rows) == 0 {
		return nil
	}

	return append([]ui.Row{homeHeading("attention", "Needs attention", len(rows))}, rows...)
}

// recentThing is a document opened or changed lately.
type recentThing struct {
	entry project.Entry
	at    time.Time
	verb  string
}

// recentRows lists what was opened or changed lately, newest first, under a
// heading for each day.
func (s *State) recentRows(docs frame.Docs, index *project.Index, shown map[ids.ULID]bool, now time.Time) []ui.Row {
	latest := map[ids.ULID]recentThing{}
	note := func(entry project.Entry, at time.Time, verb string) {
		if at.IsZero() || shown[entry.ID] || !listedOnHome(entry) {
			return
		}
		if before, known := latest[entry.ID]; !known || at.After(before.at) {
			latest[entry.ID] = recentThing{entry: entry, at: at, verb: verb}
		}
	}
	for _, open := range s.home.opened {
		if entry, found := index.Entry(open.Document); found {
			note(entry, open.At, "opened")
		}
	}
	for _, entry := range index.Entries() {
		note(entry, entry.Changed, "changed")
	}
	recent := slices.SortedFunc(func(yield func(recentThing) bool) {
		for _, each := range latest {
			if !yield(each) {
				return
			}
		}
	}, func(a, b recentThing) int {
		return cmp.Or(b.at.Compare(a.at), strings.Compare(a.entry.Name, b.entry.Name))
	})
	recent = recent[:min(len(recent), homeRecentMost)]
	var rows []ui.Row
	day := ""
	for _, each := range recent {
		if heading := dayHeading(each.at, now); heading != day {
			day = heading
			rows = append(rows, homeHeading("day:"+heading, heading, 0))
		}
		rows = append(rows, s.homeRow(docs, index, each.entry, each.verb+" "+whenCell(each.at, now)))
	}

	return rows
}

// dayHeading names the day a time falls in, as Home groups recent things.
func dayHeading(when, now time.Time) string {
	when = when.In(now.Location())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch {
	case !when.Before(today):
		return dayToday
	case !when.Before(today.AddDate(0, 0, -1)):
		return dayYesterday
	case !when.Before(today.AddDate(0, 0, -weekDays)):
		return "Earlier this week"
	}

	return "Earlier"
}

// whenCell says when, on Home: the time today and yesterday, else the day
// or the date.
func whenCell(when, now time.Time) string {
	switch dayHeading(when, now) {
	case dayToday, dayYesterday:
		return when.In(now.Location()).Format("15:04")
	}

	return changedCell(when, now)
}

// homeViewRows lists the saved views, which a click applies.
func (s *State) homeViewRows(docs frame.Docs) []ui.Row {
	views := savedViews(docs)
	if len(views) == 0 {
		return nil
	}
	rows := []ui.Row{homeHeading("views", "Views", len(views))}
	for _, view := range views {
		row := ui.NewRow(viewPrefix+view.Name, view.Name)
		row.Icon = ui.IconFolder
		rows = append(rows, row)
	}

	return rows
}

// pinned reports whether a document is pinned to Home.
func (s *State) pinned(documentID ids.ULID) bool {
	return slices.Contains(s.home.kept.Pins, documentID)
}

// togglePin pins the selected document to Home, or unpins it.
func (s *State) togglePin(ctx *frame.Context) {
	entry, ok := s.selected(ctx.Docs)
	if !ok {
		return
	}
	if s.pinned(entry.ID) {
		s.home.kept.Pins = slices.DeleteFunc(slices.Clone(s.home.kept.Pins), func(id ids.ULID) bool {
			return id == entry.ID
		})
		ctx.Frame.Status("Unpinned “" + entry.Name + "” from Home.")
	} else {
		s.home.kept.Pins = append(slices.Clone(s.home.kept.Pins), entry.ID)
		ctx.Frame.Status("Pinned “" + entry.Name + "” to Home.")
	}
	s.keepHome(ctx)
}

// homeItems are a document's right-click items for Home: pin or unpin it,
// and, on Home and in Matches, show it in its folder.
func (s *State) homeItems(entry project.Entry) []ui.MenuItem {
	pin := ruled(contextItem(contextPin, "Pin to Home", ui.IconHome))
	if s.pinned(entry.ID) {
		pin.Label = "Unpin from Home"
	}
	items := []ui.MenuItem{pin}
	if s.place == placeHome || s.place == placeMatches {
		items = append(items, contextItem(contextReveal, "Show in folder", ui.IconFolder))
	}

	return items
}
