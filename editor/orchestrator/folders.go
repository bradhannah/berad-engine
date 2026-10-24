package orchestrator

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bradhannah/berad-engine/editor/model"
	projectindex "github.com/bradhannah/berad-engine/editor/plugins/project"
)

// finderFile is the file macOS's Finder leaves in folders it has shown; a
// folder holding nothing else is empty.
const finderFile = ".DS_Store"

// errFolderInUse reports a folder's directory that still holds files.
var errFolderInUse = errors.New("the folder still holds files")

// folderOf returns the folder a document's file lies in within its type's
// folder, "workflows/Town/Guards/guard--ID.json" giving "Town/Guards", or ""
// for a file at the top or outside its type's folder.
func folderOf(format, path string) string {
	dir := filepath.ToSlash(filepath.Dir(path))
	top := format + "s"
	if !strings.HasPrefix(dir, top+projectindex.FolderSeparator) {
		return ""
	}

	return strings.TrimPrefix(dir, top+projectindex.FolderSeparator)
}

// folderDir returns where a folder lives on disk.
func folderDir(folder projectindex.Folder) string {
	return filepath.Join(folder.Format+"s", filepath.FromSlash(folder.Path))
}

// foldersOnDisk turns the project's directories into folders: every
// directory inside a document type's folder, empty or not.
func (o *Orchestrator) foldersOnDisk(dirs []string) map[projectindex.Folder]bool {
	folders := map[projectindex.Folder]bool{}
	for _, dir := range dirs {
		top, rest, inside := strings.Cut(filepath.ToSlash(dir), projectindex.FolderSeparator)
		format := strings.TrimSuffix(top, "s")
		if _, known := o.cfg.Registry.Document(format); !inside || !known || format == projectindex.FormatName {
			continue
		}
		folders[projectindex.Folder{Format: format, Path: rest}] = true
	}

	return folders
}

// indexNow builds a project index from the documents and folders as they
// are now.
func (o *Orchestrator) indexNow() *projectindex.Index {
	entries := make([]projectindex.Entry, 0, len(o.docs))
	for id, entry := range o.docs {
		if !isIndex(id) {
			entries = append(entries, projectindex.Entry{
				ID: id, Name: entry.info.Name, Format: entry.info.Format, Folder: entry.info.Folder,
				Tags: entry.info.Tags,
			})
		}
	}

	return projectindex.NewIndexWithFolders(entries, slices.Collect(maps.Keys(o.folders)))
}

// liveIndex returns the project index windows see, once it is built.
func (o *Orchestrator) liveIndex() (*projectindex.Index, bool) {
	entry, built := o.docs[projectindex.IndexID()]
	if !built {
		return nil, false
	}
	index, isIndex := entry.document.(*projectindex.Index)

	return index, isIndex
}

// syncFromIndex carries a move, folder or tags edit, already applied to an
// index, over to the documents: a moved document's files move at the next
// save, changed tags are saved, and the directories of folders that went
// are removed at the next save once nothing is left in them.
func (o *Orchestrator) syncFromIndex(index *projectindex.Index) {
	for _, listed := range index.Entries() {
		entry, ok := o.docs[listed.ID]
		if !ok {
			continue
		}
		if !slices.Equal(entry.info.Tags, listed.Tags) {
			entry.info.Tags = slices.Clone(listed.Tags)
			entry.dirty = true
		}
		if entry.info.Folder != listed.Folder {
			entry.info.Folder = listed.Folder
			o.repath(entry)
		}
	}
	kept := map[projectindex.Folder]bool{}
	for _, folder := range index.AllFolders() {
		kept[folder] = true
	}
	for folder := range o.folders {
		if !kept[folder] {
			o.removedDirs[folderDir(folder)] = true
		}
	}
	o.folders = kept
}

// repath moves a document's files to where its folder and name put them, at
// the next save.
func (o *Orchestrator) repath(entry *documentEntry) {
	path := documentPath(entry.info.Format, entry.info.Folder, entry.info.Name, entry.info.ID)
	if path != entry.info.Path {
		o.retireFiles(entry.info.Path)
		o.keepFiles(path)
		entry.info.Path = path
	}
	entry.dirty = true
}

// prepareFolders checks a move or a folder change before it is applied: a
// document that is part of another, such as a floor of a zone, stays with
// it, and folders belong to a document type.
func (o *Orchestrator) prepareFolders(edit string, args json.RawMessage) error {
	switch edit {
	case projectindex.MoveEdit:
		var move projectindex.MoveArgs
		if err := json.Unmarshal(args, &move); err != nil {
			return fmt.Errorf("%w: %w", model.ErrArgs, err)
		}
		if _, ok := o.docs[move.ID]; !ok || isIndex(move.ID) {
			return fmt.Errorf("%w: %s", ErrUnknownDocument, move.ID)
		}
		if !o.containerOf(move.ID).IsZero() {
			return fmt.Errorf("%w: it stays with the document it is part of", projectindex.ErrFolder)
		}
	case projectindex.FolderEdit:
		var change projectindex.FolderArgs
		if err := json.Unmarshal(args, &change); err != nil {
			return fmt.Errorf("%w: %w", model.ErrArgs, err)
		}
		if _, known := o.cfg.Registry.Document(change.Format); !known || change.Format == projectindex.FormatName {
			return fmt.Errorf("%w: no document type %q", projectindex.ErrFolder, change.Format)
		}
	}

	return nil
}

// replayFolders applies a journaled move, folder or tags edit during
// recovery. The files may already reflect it, so an edit that no longer
// applies is skipped.
func (o *Orchestrator) replayFolders(entry journalEntry) error {
	kind, ok := o.cfg.Registry.Edit(entry.Edit)
	if !ok {
		return fmt.Errorf("replay: %w: %s", ErrUnknownEdit, entry.Edit)
	}
	index := o.indexNow()
	if _, err := kind.Apply(index, entry.Args); err != nil {
		o.cfg.Logger.Info("skipping a journaled change the files already hold", "edit", entry.Edit, "error", err)

		return nil
	}
	o.syncFromIndex(index)

	return nil
}

// flushFolders makes every folder's directory, and removes the directories
// of folders that went, deepest first, when nothing but Finder's file is
// left in them. A directory still holding other files is left alone.
func (o *Orchestrator) flushFolders() error {
	var errs []error
	live := map[string]bool{}
	for folder := range o.folders {
		dir := folderDir(folder)
		live[dir] = true
		if err := o.project.root.MkdirAll(dir, dirPerm); err != nil {
			errs = append(errs, fmt.Errorf("create folder %s: %w", dir, err))
		}
	}
	gone := slices.SortedFunc(maps.Keys(o.removedDirs), func(a, b string) int { return cmp.Compare(len(b), len(a)) })
	for _, dir := range gone {
		delete(o.removedDirs, dir)
		if live[dir] {
			continue
		}
		if err := o.removeEmptyDir(dir); err != nil {
			o.cfg.Logger.Info("left a folder that still holds files", "folder", dir, "error", err)
		}
	}

	return errors.Join(errs...)
}

// removeEmptyDir removes a directory holding nothing, or nothing but
// Finder's file.
func (o *Orchestrator) removeEmptyDir(dir string) error {
	entries, err := fs.ReadDir(o.project.root.FS(), filepath.ToSlash(dir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.Name() != finderFile {
			return fmt.Errorf("%w: %s holds %s", errFolderInUse, dir, entry.Name())
		}
	}
	if len(entries) > 0 {
		if err := o.project.root.Remove(filepath.Join(dir, finderFile)); err != nil {
			return fmt.Errorf("remove %s: %w", finderFile, err)
		}
	}
	if err := o.project.root.Remove(dir); err != nil {
		return fmt.Errorf("remove %s: %w", dir, err)
	}

	return nil
}
