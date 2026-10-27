package browser

import (
	"image"
	"slices"
	"strings"

	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// Folder rows have the ID "folder:<format>:<path>".
const folderPrefix = "folder:"

// Menu item IDs for a new folder of each type, and the Move to… menu's
// items, "move:" and a folder's path ("move:" alone is the top).
const (
	newFolderPrefix = "new-folder-"
	movePrefix      = "move:"
)

// pathMark separates folder names where a folder's path is shown.
const pathMark = " › "

func folderRowID(format, path string) string { return folderPrefix + format + ":" + path }

// parseFolderRow reads a folder row's ID.
func parseFolderRow(id string) (project.Folder, bool) {
	rest, isFolder := strings.CutPrefix(id, folderPrefix)
	format, path, found := strings.Cut(rest, ":")

	return project.Folder{Format: format, Path: path}, isFolder && found
}

// showPath writes a folder's path for people, "Town › Guards", or the top of
// its section.
func showPath(section, path string) string {
	if path == "" {
		return "Top of " + strings.ToLower(strings.TrimPrefix(section, sectionPrefix))
	}

	return strings.ReplaceAll(path, project.FolderSeparator, pathMark)
}

// matcher is what the search box asks for: names, folder names or tags that
// hold the text, or, after "#", tags only. It ignores case.
type matcher struct {
	text string
	tags bool
}

func newMatcher(query string) matcher {
	text := strings.ToLower(strings.TrimSpace(query))
	if rest, isTag := strings.CutPrefix(text, "#"); isTag {
		return matcher{text: strings.TrimSpace(rest), tags: true}
	}

	return matcher{text: text, tags: false}
}

// active reports whether anything is being searched for.
func (m matcher) active() bool { return m.text != "" || m.tags }

// entry reports whether a document matches: by a tag, or by its name or a
// folder it is in unless only tags are searched.
func (m matcher) entry(entry project.Entry) bool {
	if !m.active() {
		return true
	}
	for _, tag := range entry.Tags {
		if strings.Contains(strings.ToLower(tag), m.text) {
			return true
		}
	}
	if m.tags {
		return false
	}
	if strings.Contains(strings.ToLower(entry.Name), m.text) {
		return true
	}

	return entry.Folder != "" && slices.ContainsFunc(strings.Split(entry.Folder, project.FolderSeparator), m.folderName)
}

// folder reports whether a folder's own name matches.
func (m matcher) folder(path string) bool { return !m.tags && m.folderName(project.FolderName(path)) }

func (m matcher) folderName(name string) bool { return strings.Contains(strings.ToLower(name), m.text) }

// words reports whether a row that is not a document, such as Settings,
// matches by its own words. A search for tags finds none.
func (m matcher) words(texts ...string) bool {
	return !m.tags && slices.ContainsFunc(texts, m.folderName)
}

// selectedFolder returns the selected folder.
func (s *State) selectedFolder() (project.Folder, bool) { return parseFolderRow(s.list.Selected) }

// placeOf returns the type and folder something selected is in, for making
// new things beside it: a selected folder, or a document's folder.
func (s *State) placeOf(docs frame.Docs) (string, string, bool) {
	if folder, ok := s.selectedFolder(); ok {
		return folder.Format, folder.Path, true
	}
	if entry, ok := s.selected(docs); ok && formatSection(entry.Format) != "" {
		return entry.Format, entry.Folder, true
	}

	return "", "", false
}

// targetFolder returns where a new document of a type goes: the selected
// folder of that type, or the folder of a selected document of that type,
// or the top.
func (s *State) targetFolder(docs frame.Docs, format string) string {
	if selectedFormat, folder, ok := s.placeOf(docs); ok && selectedFormat == format {
		return folder
	}

	return ""
}

// createFolder makes a folder of a type, inside the selected folder of that
// type when there is one, selects it and focuses its name.
func (s *State) createFolder(ctx *frame.Context, format string) {
	index := s.index(ctx.Docs)
	parent := s.targetFolder(ctx.Docs, format)
	var siblings []string
	for _, path := range index.Folders(format) {
		if strings.EqualFold(project.FolderParent(path), parent) {
			siblings = append(siblings, project.FolderName(path))
		}
	}
	path := project.JoinFolder(parent, uniqueName("Folder", siblings))
	change := project.FolderArgs{Format: format, From: "", To: path}
	s.command(ctx, project.IndexID(), project.FolderEdit, s.gesture(), change, func(protocol.Result, error) {
		s.list.Selected = folderRowID(format, path)
		s.closed[folderRowID(format, parent)] = false
		s.name.Field.Revert()
		s.tags.Field.Revert()
		s.named = s.list.Selected
		s.name.Field.SetSaved(project.FolderName(path))
		s.focus.Set(s.name)
		if buffer := s.name.Field.Buffer(); buffer != nil {
			buffer.SelectAll()
		}
		ctx.Frame.Status("Made a folder. Type its name, then press Enter or Save.")
	}, nil)
}

// deleteFolder deletes the selected folder when it holds nothing.
func (s *State) deleteFolder(ctx *frame.Context, folder project.Folder) {
	index := s.index(ctx.Docs)
	if hasChildren(index.Folders(folder.Format), index.OfFormat(folder.Format), folder.Path) {
		ctx.Frame.Status("Move what is in the folder out first; only an empty folder can be deleted")

		return
	}
	change := project.FolderArgs{Format: folder.Format, From: folder.Path, To: ""}
	s.command(ctx, project.IndexID(), project.FolderEdit, s.gesture(), change, func(protocol.Result, error) {
		s.list.Selected = ""
		ctx.Frame.Status("Deleted the folder. Undo brings it back.")
	}, nil)
}

// openMoveMenu opens Move to… below its button.
func (s *State) openMoveMenu(docs frame.Docs) {
	if s.moveItems(docs) {
		s.moveMenu.Open(s.moveBn.Rect, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
	}
}

// moveItems lists where the selected document or folder can move: the top
// of its section or any folder of its type, but not where it already is,
// nor into a folder it holds. It reports whether anything is selected.
func (s *State) moveItems(docs frame.Docs) bool {
	format, current, ok := s.placeOf(docs)
	if !ok {
		return false
	}
	moving, isFolder := s.selectedFolder()
	if isFolder {
		current = project.FolderParent(moving.Path)
	}
	section := formatSection(format)
	items := []ui.MenuItem{moveItem(section, "", current == "")}
	for _, path := range s.index(docs).Folders(format) {
		if isFolder && project.Within(path, moving.Path) {
			continue
		}
		items = append(items, moveItem(section, path, strings.EqualFold(path, current)))
	}
	s.moveMenu.Items, s.moveMenu.Width = items, ui.MenuWidth(items, s.theme)

	return true
}

func moveItem(section, path string, here bool) ui.MenuItem {
	icon := ui.IconFolder
	if path == "" {
		icon = ui.IconNone
	}

	return ui.MenuItem{
		ID: movePrefix + path, Label: showPath(section, path), Icon: icon, Disabled: here, Danger: false, Rule: false,
	}
}

// moveSelected moves the selected document or folder into a folder, or to
// the top of its section when path is "".
func (s *State) moveSelected(ctx *frame.Context, path string) {
	if folder, ok := s.selectedFolder(); ok {
		s.moveFolder(ctx, folder, path)

		return
	}
	if entry, ok := s.selected(ctx.Docs); ok {
		s.moveDocument(ctx, entry.ID, path)
	}
}

func (s *State) moveDocument(ctx *frame.Context, documentID ids.ULID, path string) {
	move := project.MoveArgs{ID: documentID, Folder: path}
	s.command(ctx, project.IndexID(), project.MoveEdit, s.gesture(), move, func(protocol.Result, error) {
		ctx.Frame.Status("Moved to " + showPath(sectionOf(s, ctx.Docs, documentID), path))
	}, nil)
}

func (s *State) moveFolder(ctx *frame.Context, folder project.Folder, parent string) {
	target := project.JoinFolder(parent, project.FolderName(folder.Path))
	change := project.FolderArgs{Format: folder.Format, From: folder.Path, To: target}
	s.command(ctx, project.IndexID(), project.FolderEdit, s.gesture(), change, func(protocol.Result, error) {
		if s.list.Selected == folderRowID(folder.Format, folder.Path) {
			s.list.Selected = folderRowID(folder.Format, target)
		}
		ctx.Frame.Status("Moved the folder to " + showPath(formatSection(folder.Format), parent))
	}, nil)
}

func sectionOf(s *State, docs frame.Docs, documentID ids.ULID) string {
	entry, _ := s.index(docs).Entry(documentID)

	return formatSection(entry.Format)
}

// dropPlace returns the type and folder a row stands for as somewhere to
// drop: a folder, or the top of a section.
func dropPlace(row string) (string, string, bool) {
	if folder, ok := parseFolderRow(row); ok {
		return folder.Format, folder.Path, true
	}
	if format := sectionFormat(row); format != "" {
		return format, "", true
	}

	return "", "", false
}

// canDrop reports whether a dragged row can be dropped on another: a
// document onto another folder of its type or its section's top, a folder
// onto a folder of its type outside itself or its section's top. A floor
// stays with its zone.
func (s *State) canDrop(index *project.Index, dragged, target string) bool {
	format, path, ok := dropPlace(target)
	if !ok {
		return false
	}
	if folder, isFolder := parseFolderRow(dragged); isFolder {
		return folder.Format == format && !project.Within(path, folder.Path) &&
			!strings.EqualFold(project.FolderParent(folder.Path), path)
	}
	documentID, err := ids.Parse(dragged)
	if err != nil {
		return false
	}
	entry, found := index.Entry(documentID)

	return found && entry.Format == format && entry.Format != floor.FormatName && !strings.EqualFold(entry.Folder, path)
}

// drop carries out a row dropped on a folder or a section heading.
func (s *State) drop(ctx *frame.Context, dragged, target string) {
	_, path, ok := dropPlace(target)
	if !ok {
		return
	}
	if folder, isFolder := parseFolderRow(dragged); isFolder {
		s.moveFolder(ctx, folder, path)

		return
	}
	if documentID, err := ids.Parse(dragged); err == nil {
		s.moveDocument(ctx, documentID, path)
	}
}

// renameFolder saves a folder's new name.
func (s *State) renameFolder(ctx *frame.Context, folder project.Folder, name string) {
	target := project.JoinFolder(project.FolderParent(folder.Path), name)
	change := project.FolderArgs{Format: folder.Format, From: folder.Path, To: target}
	s.command(ctx, project.IndexID(), project.FolderEdit, "", change,
		func(protocol.Result, error) {
			s.focus.Set(nil)
			s.list.Selected = folderRowID(folder.Format, target)
			s.named = s.list.Selected
			ctx.Frame.Status("Renamed the folder to “" + name + "”")
		},
		func() { s.name.Field.Failed() })
}

// commitTags saves the selected document's tags, typed as "town, guards".
func (s *State) commitTags(ctx *frame.Context) {
	typed, changed := s.tags.Field.Commit()
	documentID, err := ids.Parse(s.named)
	if !changed || err != nil {
		s.focus.Set(nil)

		return
	}
	retag := project.TagsArgs{ID: documentID, Tags: project.NormalizeTags(strings.Split(typed, ","))}
	s.command(ctx, project.IndexID(), project.TagsEdit, "", retag,
		func(protocol.Result, error) {
			s.focus.Set(nil)
			ctx.Frame.Status("Tags saved")
		},
		func() { s.tags.Field.Failed() })
}

// searchTag shows every document with a tag.
func (s *State) searchTag(tag string) {
	s.search.Field.SetSaved("#" + tag)
	s.search.Field.Revert()
	s.focus.Set(nil)
}
