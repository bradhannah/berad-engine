// Package picker runs pick on map for a window that needs an anchor (D13). It
// asks which Map windows could answer: with none open it asks which zone to
// open, with one it uses it, and with several it asks which, marking the maps
// that hold anchors. It then starts the pick and hands the answer back.
package picker

import (
	"image"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/zone"
)

const shortID = 6

// reasonTakenOver is why a pick ends when another control starts one.
const reasonTakenOver = "another pick started"

type choice int

const (
	choosingNothing choice = iota
	choosingMap
	choosingZone
)

// Picker runs one window's picks, one at a time. Each pick has an owner,
// the control that started it, so only that control offers to cancel it.
type Picker struct {
	window   ids.ULID
	picking  string
	owner    string
	count    int
	dialog   *ui.Dialog
	choosing choice
	label    string
	now      protocol.PickNow
	done     func(protocol.PickAnswer, *frame.Context)
}

// New returns the picker for a window.
func New(window ids.ULID) *Picker {
	return &Picker{
		window: window, picking: "", owner: "", count: 0, dialog: nil, choosing: choosingNothing, label: "",
		now: protocol.PickNow{Floor: ids.ULID{}, Item: "", Name: ""}, done: nil,
	}
}

// Picking reports whether a pick is in progress.
func (p *Picker) Picking() bool { return p.picking != "" }

// PickingFor reports whether the pick in progress is the owner's.
func (p *Picker) PickingFor(owner string) bool { return p.picking != "" && p.owner == owner }

// Label is an owner's pick button: Cancel pick while its pick runs, Pick on
// map otherwise.
func (p *Picker) Label(owner string) string {
	if p.PickingFor(owner) {
		return "Cancel pick"
	}

	return "Pick on map"
}

// Toggle cancels the owner's pick in progress, or starts one for it, taking
// over any other pick, with reason saying where a cancel came from.
func (p *Picker) Toggle(ctx *frame.Context, owner, reason, label string, now protocol.PickNow,
	done func(protocol.PickAnswer, *frame.Context),
) {
	if p.PickingFor(owner) {
		p.Cancel(ctx, reason)

		return
	}
	p.Start(ctx, owner, label, now, done)
}

// Dialog returns the dialog choosing a map or zone, or nil.
func (p *Picker) Dialog() *ui.Dialog { return p.dialog }

// Start begins picking an anchor for an owner, taking over a pick in
// progress. label says what it is for and now what it would replace, both
// shown in the Map window; done receives the answer.
func (p *Picker) Start(ctx *frame.Context, owner, label string, now protocol.PickNow,
	done func(protocol.PickAnswer, *frame.Context),
) {
	if p.picking != "" {
		ctx.Requests.PickCancel(protocol.PickCancel{Pick: p.picking, Reason: reasonTakenOver})
		p.picking = ""
	}
	p.owner, p.label, p.now, p.done = owner, label, now, done
	ctx.Requests.PickAsk(protocol.PickAnchor, func(maps []protocol.PickMap, err error) {
		if err != nil {
			ctx.Frame.Status(err.Error())

			return
		}
		switch len(maps) {
		case 0:
			p.chooseZone(ctx)
		case 1:
			p.begin(ctx, protocol.PickStart{
				Pick: "", What: "", Label: "", Window: maps[0].Window, Zone: maps[0].Zone, From: ids.ULID{},
				Now: p.now,
			})
		default:
			p.chooseMap(ctx, maps)
		}
	})
}

// Cancel ends the pick in progress.
func (p *Picker) Cancel(ctx *frame.Context, reason string) {
	ctx.Requests.PickCancel(protocol.PickCancel{Pick: p.picking, Reason: reason})
	p.picking = ""
	ctx.Frame.Status("Pick cancelled")
}

// Changed takes this picker's answers and cancellations, and reports whether
// it used the update.
func (p *Picker) Changed(update client.Update, ctx *frame.Context) bool {
	switch {
	case update.PickAnswer != nil && update.PickAnswer.Pick == p.picking && p.picking != "":
		p.picking = ""
		if p.done != nil {
			p.done(*update.PickAnswer, ctx)
		}

		return true
	case update.PickCancel != nil && update.PickCancel.Pick == p.picking && p.picking != "":
		p.picking = ""
		ctx.Frame.Status("Pick cancelled: " + update.PickCancel.Reason)

		return true
	}

	return false
}

// UpdateDialog runs the dialog for a frame; the window gives it all input
// while it is open.
func (p *Picker) UpdateDialog(input *ui.Input, ctx *frame.Context, window image.Rectangle) {
	chosen := p.dialog.Update(input, window)
	if chosen == "" {
		return
	}
	dialog, kind := p.dialog, p.choosing
	p.dialog, p.choosing = nil, choosingNothing
	if chosen == ui.DialogCancel || dialog.List == nil {
		return
	}
	target, err := ids.Parse(dialog.List.Selected)
	if err != nil {
		return
	}
	start := protocol.PickStart{
		Pick: "", What: "", Label: "", Window: ids.ULID{}, Zone: ids.ULID{}, From: ids.ULID{}, Now: p.now,
	}
	if kind == choosingZone {
		start.Zone = target
	} else {
		start.Window = target
	}
	p.begin(ctx, start)
}

// Locate finds a dialog button by ID, or its nth row as "row<n>", for input
// scripts.
func (p *Picker) Locate(value string, window image.Rectangle) (image.Point, bool) {
	if p.dialog == nil {
		return image.Pt(0, 0), false
	}
	if number, err := strconv.Atoi(strings.TrimPrefix(value, "row")); err == nil && p.dialog.List != nil &&
		number >= 1 && number <= len(p.dialog.List.Rows) {
		p.dialog.Locate("", window)

		return p.dialog.List.Locate(p.dialog.List.Rows[number-1].ID)
	}

	return p.dialog.Locate(value, window)
}

func (p *Picker) begin(ctx *frame.Context, start protocol.PickStart) {
	p.count++
	start.Pick = p.window.String() + "-" + strconv.Itoa(p.count)
	start.What = protocol.PickAnchor
	start.Label, start.Now = p.label, p.now
	p.picking = start.Pick
	ctx.Requests.PickStart(start, func(result protocol.Result, err error) {
		if err != nil {
			p.picking = ""
			ctx.Frame.Report("")(result, err)

			return
		}
		ctx.Frame.Status("Click an anchor in the map. Escape here or there cancels.")
	})
}

func (p *Picker) chooseZone(ctx *frame.Context) {
	zones := names(ctx.Docs).OfFormat(zone.FormatName)
	if len(zones) == 0 {
		ctx.Frame.Refuse("There are no zones to pick from. Create one in the project browser.")

		return
	}
	list := ui.NewList("zones", zones[0].ID.String())
	for _, entry := range zones {
		row := ui.NewRow(entry.ID.String(), entry.Name)
		row.Icon = ui.IconZone
		list.Rows = append(list.Rows, row)
	}
	p.dialog = ui.NewDialog("No map is open. Open which zone to pick from?", nil,
		ui.DialogButton("open", "Open map"), ui.DialogButton(ui.DialogCancel, "Cancel"))
	p.dialog.List = list
	p.choosing = choosingZone
}

func (p *Picker) chooseMap(ctx *frame.Context, maps []protocol.PickMap) {
	index := names(ctx.Docs)
	list := ui.NewList("maps", "")
	for number, option := range maps {
		detail := "view " + option.Window.Short(shortID)
		if !option.Shows {
			detail += ", no anchors"
		}
		row := ui.NewRow(option.Window.String(), strconv.Itoa(number+1)+". "+index.Name(option.Zone, "zone"))
		row.Detail, row.Marked = detail, option.Shows
		list.Rows = append(list.Rows, row)
		if list.Selected == "" && option.Shows {
			list.Selected = option.Window.String()
		}
	}
	if list.Selected == "" {
		list.Selected = list.Rows[0].ID
	}
	p.dialog = ui.NewDialog("Pick in which map?", []string{"● marks maps that have anchors."},
		ui.DialogButton("use", "Pick there"), ui.DialogButton(ui.DialogCancel, "Cancel"))
	p.dialog.List = list
	p.choosing = choosingMap
}

// names returns a copy of the project index, for zone and floor names.
func names(docs frame.Docs) *project.Index {
	found := project.NewIndex(nil)
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			found = project.NewIndex(index.Entries())
		}
	})

	return found
}
