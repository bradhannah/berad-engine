package browser

import (
	"image"
	"strings"

	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// The right-click menu's own choices. Making something shares New ▾'s item
// IDs.
const (
	contextOpen   = "context-open"
	contextRename = "context-rename"
	contextMove   = "context-move"
	contextDelete = "context-delete"
)

// typeLabel names a type in the right-click menu's items, as "New zone".
func typeLabel(format string) string {
	return map[string]string{
		zone.FormatName: "zone", flow.FormatName: "workflow", npc.FormatName: npcLabel,
		textdoc.FormatName: "text document",
	}[format]
}

// typeIcon is a type's icon in the right-click menu.
func typeIcon(format string) ui.Icon {
	return map[string]ui.Icon{
		zone.FormatName: ui.IconZone, flow.FormatName: ui.IconWorkflow, npc.FormatName: ui.IconNPC,
		textdoc.FormatName: ui.IconText,
	}[format]
}

// newItem is the item that makes a document of a type.
func newItem(format string) ui.MenuItem {
	return contextItem(sectionNew(formatSection(format)), "New "+typeLabel(format), typeIcon(format))
}

// contextClick is a right-click waiting for the selection to settle: the row
// it landed on, or "" below the rows, and where the pointer was.
type contextClick struct {
	row     string
	at      image.Point
	pending bool
}

func noContextClick() contextClick { return contextClick{row: "", at: image.Pt(0, 0), pending: false} }

func contextItem(id, label string, icon ui.Icon) ui.MenuItem {
	return ui.MenuItem{ID: id, Label: label, Icon: icon, Disabled: false, Danger: false, Rule: false}
}

// ruled starts a group with an item.
func ruled(item ui.MenuItem) ui.MenuItem {
	item.Rule = true

	return item
}

// rightClicked selects what a right-click landed on, so the menu acts on
// it: a document or a folder. A section heading or the space below the rows
// selects nothing, so what the menu makes goes at the top of its section.
func (s *State) rightClicked(row string, point image.Point) {
	selected := row
	if row == "" || strings.HasPrefix(row, sectionPrefix) {
		selected = ""
	}
	if s.list.Selected != selected {
		s.list.Selected = selected
		s.focus.Set(nil)
	}
	s.context = contextClick{row: row, at: point, pending: true}
}

// openContextMenu opens the right-click menu once the selection has settled.
// When the inspector's unsaved changes held the selection back, it does not
// open.
func (s *State) openContextMenu(docs frame.Docs) {
	click := s.context
	s.context = noContextClick()
	if !click.pending || s.guard.Active() {
		return
	}
	if row := click.row; row != "" && !strings.HasPrefix(row, sectionPrefix) && s.list.Selected != row {
		return
	}
	items := s.contextItems(docs, click.row)
	if len(items) == 0 {
		return
	}
	s.contextMenu.Items, s.contextMenu.Width = items, ui.MenuWidth(items, s.theme)
	s.contextMenu.OpenAt(click.at, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
	s.contextAt = click.at
}

// contextItems lists what a right-click offers where it landed. On a section
// heading it makes a document or a folder of that type; on a folder it makes
// a document or a folder inside it, and renames, moves or deletes it; on a
// document it opens it, makes another beside it, and renames, moves or
// deletes it. Below the rows it offers everything New ▾ makes.
func (s *State) contextItems(docs frame.Docs, row string) []ui.MenuItem {
	if row == "" {
		var items []ui.MenuItem
		for _, item := range menuItems() {
			if item.ID != newFloor {
				items = append(items, item)
			}
		}

		return items
	}
	if format := sectionFormat(row); strings.HasPrefix(row, sectionPrefix) {
		if format == "" {
			return nil
		}

		return []ui.MenuItem{
			newItem(format), contextItem(newFolderPrefix+format, "New "+typeLabel(format)+" folder", ui.IconFolder),
		}
	}
	if folder, isFolder := parseFolderRow(row); isFolder {
		index := s.index(docs)
		remove := ruled(dangerous(contextItem(contextDelete, "Delete folder", ui.IconNone)))
		remove.Disabled = hasChildren(index.Folders(folder.Format), index.OfFormat(folder.Format), folder.Path)

		return []ui.MenuItem{
			newItem(folder.Format), contextItem(newFolderPrefix+folder.Format, "New folder", ui.IconFolder),
			ruled(contextItem(contextRename, "Rename", ui.IconNone)),
			contextItem(contextMove, "Move to…", ui.IconNone),
			remove,
		}
	}
	entry, ok := s.selected(docs)
	if !ok {
		return nil
	}
	if entry.Format == floor.FormatName {
		return s.floorItems(docs, entry)
	}

	return s.documentItems(docs, entry)
}

func dangerous(item ui.MenuItem) ui.MenuItem {
	item.Danger = true

	return item
}

// documentItems lists a zone's, workflow's, NPC's or text document's
// right-click menu. A zone can gain a floor; an NPC opens its behaviour.
func (s *State) documentItems(docs frame.Docs, entry project.Entry) []ui.MenuItem {
	open := contextItem(contextOpen, "Open", ui.IconNone)
	if entry.Format == npc.FormatName {
		open.Label = "Open behaviour"
		found, loaded := s.npcOf(docs, entry.ID)
		open.Disabled = !loaded || found.Behaviour.IsZero()
	}
	items := []ui.MenuItem{
		open, ruled(newItem(entry.Format)), contextItem(newFolderPrefix+entry.Format, "New folder", ui.IconFolder),
	}
	if entry.Format == zone.FormatName {
		items = append(items, contextItem(newFloor, "Add floor", ui.IconFloor))
	}

	return append(items,
		ruled(contextItem(contextRename, "Rename", ui.IconNone)),
		contextItem(contextMove, "Move to…", ui.IconNone),
		ruled(dangerous(contextItem(contextDelete, "Delete", ui.IconNone))),
	)
}

// floorItems lists a floor's right-click menu. A floor stays with its zone,
// so it cannot move into a folder.
func (s *State) floorItems(docs frame.Docs, entry project.Entry) []ui.MenuItem {
	_, inZone := s.zoneOf(docs, entry.ID)
	open := contextItem(contextOpen, "Open map", ui.IconNone)
	open.Disabled = !inZone
	addFloor := ruled(contextItem(newFloor, "Add floor", ui.IconFloor))
	addFloor.Disabled = !inZone

	return []ui.MenuItem{
		open, addFloor,
		ruled(contextItem(contextRename, "Rename", ui.IconNone)),
		ruled(dangerous(contextItem(contextDelete, "Delete", ui.IconNone))),
	}
}

// chooseFromContext carries out a right-click menu choice on the selection.
func (s *State) chooseFromContext(ctx *frame.Context, chosen string) {
	switch chosen {
	case "":
	case contextOpen:
		s.openSelected(ctx)
	case contextRename:
		s.startRename(ctx.Docs)
	case contextMove:
		if s.moveItems(ctx.Docs) {
			s.moveMenu.OpenAt(s.contextAt, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
		}
	case contextDelete:
		s.deleteSelected(ctx)
	default:
		s.create(ctx, chosen)
	}
}

// startRename focuses the selection's name with all of it selected, ready
// to type over.
func (s *State) startRename(docs frame.Docs) {
	s.syncName(docs)
	s.layout(docs)
	s.focus.Set(s.name)
	if buffer := s.name.Field.Buffer(); buffer != nil {
		buffer.SelectAll()
	}
}
