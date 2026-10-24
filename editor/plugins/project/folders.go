package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Edit kind names for folders and tags: moving a document into a folder,
// making, renaming, moving or deleting a folder, and setting a document's
// tags.
const (
	MoveEdit   = "project.move"
	FolderEdit = "project.folder"
	TagsEdit   = "project.tags"
)

// Limits on folder names and tags, in bytes and in tags.
const (
	MaxFolderLength = 100
	MaxTagLength    = 40
	MaxTags         = 20
)

// FolderSeparator joins folder names into a folder's path.
const FolderSeparator = "/"

// ErrFolder reports a folder name that cannot be used, or a folder change
// that cannot be made.
var ErrFolder = errors.New("invalid folder")

// ErrTag reports a tag that cannot be used.
var ErrTag = errors.New("invalid tag")

// Folder is one folder of a document type, named by its path: folder names
// joined by "/". Folders hold documents of their type only, and may hold
// folders.
type Folder struct {
	Format string `json:"format"`
	Path   string `json:"path"`
}

// compareFolders orders folders by type, then by path: a folder before what
// it holds, and folders side by side by name, ignoring case.
func compareFolders(a, b Folder) int {
	if a.Format != b.Format {
		return strings.Compare(a.Format, b.Format)
	}

	return ComparePaths(a.Path, b.Path)
}

// ComparePaths orders folder paths: a folder before what it holds, and
// folders side by side by name, ignoring case.
func ComparePaths(a, b string) int {
	return slices.CompareFunc(strings.Split(a, FolderSeparator), strings.Split(b, FolderSeparator),
		func(x, y string) int { return strings.Compare(strings.ToLower(x), strings.ToLower(y)) })
}

// CheckFolderName validates one folder name. It becomes a folder on disk, so
// it may not hold a path separator or a character some file systems refuse,
// start or end with a dot, or start or end with a space.
func CheckFolderName(name string) error {
	switch {
	case name == "", strings.TrimSpace(name) != name, len(name) > MaxFolderLength:
	case strings.HasPrefix(name, "."), strings.HasSuffix(name, "."):
	case strings.ContainsAny(name, `/\:*?"<>|`), strings.ContainsFunc(name, unicode.IsControl):
	default:
		return nil
	}

	return fmt.Errorf("%w: %q", ErrFolder, name)
}

// CheckFolderPath validates a folder's path: one or more valid folder names
// joined by "/".
func CheckFolderPath(path string) error {
	for name := range strings.SplitSeq(path, FolderSeparator) {
		if err := CheckFolderName(name); err != nil {
			return err
		}
	}

	return nil
}

// FolderParent returns the folder holding a folder, or "" at the top.
func FolderParent(path string) string {
	parent, _, found := cutLast(path)
	if !found {
		return ""
	}

	return parent
}

// FolderName returns a folder's own name, the last of its path.
func FolderName(path string) string {
	_, name, found := cutLast(path)
	if !found {
		return path
	}

	return name
}

func cutLast(path string) (string, string, bool) {
	at := strings.LastIndex(path, FolderSeparator)
	if at < 0 {
		return "", path, false
	}

	return path[:at], path[at+len(FolderSeparator):], true
}

// JoinFolder returns the path of a folder named name inside parent, or at
// the top when parent is "".
func JoinFolder(parent, name string) string {
	if parent == "" {
		return name
	}

	return parent + FolderSeparator + name
}

// Within reports whether a folder is the folder at path or inside it,
// ignoring case. Every folder is within the top, "".
func Within(folder, path string) bool {
	if path == "" {
		return true
	}
	if len(folder) < len(path) || !strings.EqualFold(folder[:len(path)], path) {
		return false
	}

	return len(folder) == len(path) || strings.HasPrefix(folder[len(path):], FolderSeparator)
}

// rebase moves a folder within from to the same place within target.
func rebase(folder, from, target string) string { return target + folder[len(from):] }

// HasFolder reports whether a type has a folder, ignoring case. The top, "",
// always exists.
func (x *Index) HasFolder(format, path string) bool {
	_, found := x.folderPath(format, path)

	return found
}

// folderPath returns a folder's path as it was made, found ignoring case.
func (x *Index) folderPath(format, path string) (string, bool) {
	if path == "" {
		return "", true
	}
	for _, folder := range x.folders {
		if folder.Format == format && strings.EqualFold(folder.Path, path) {
			return folder.Path, true
		}
	}

	return "", false
}

// Folders returns a type's folders, each before what it holds and folders
// side by side by name.
func (x *Index) Folders(format string) []string {
	var paths []string
	for _, folder := range x.folders {
		if folder.Format == format {
			paths = append(paths, folder.Path)
		}
	}

	return paths
}

// AllFolders returns every folder of every type.
func (x *Index) AllFolders() []Folder { return slices.Clone(x.folders) }

// emptyFolder reports whether a folder holds no document and no folder.
func (x *Index) emptyFolder(format, path string) bool {
	for _, entry := range x.entries {
		if entry.Format == format && Within(entry.Folder, path) {
			return false
		}
	}
	for _, folder := range x.folders {
		if folder.Format == format && Within(folder.Path, path) && !strings.EqualFold(folder.Path, path) {
			return false
		}
	}

	return true
}

// NormalizeTags tidies tags as typed: trimmed, without a leading "#", with
// empty and repeated tags (ignoring case) left out, in the order given.
func NormalizeTags(tags []string) []string {
	var kept []string
	for _, tag := range tags {
		tag = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(tag), "#"))
		if tag != "" && !slices.ContainsFunc(kept, func(other string) bool { return strings.EqualFold(other, tag) }) {
			kept = append(kept, tag)
		}
	}

	return kept
}

// CheckTags validates tidied tags: at most MaxTags, each at most
// MaxTagLength bytes, with no comma, which separates tags as typed.
func CheckTags(tags []string) error {
	if len(tags) > MaxTags {
		return fmt.Errorf("%w: at most %d tags", ErrTag, MaxTags)
	}
	for _, tag := range tags {
		if len(tag) > MaxTagLength || strings.ContainsAny(tag, ",") || strings.ContainsFunc(tag, unicode.IsControl) {
			return fmt.Errorf("%w: %q", ErrTag, tag)
		}
	}

	return nil
}

// MoveArgs moves a document into a folder of its type, or to the top when
// Folder is "".
type MoveArgs struct {
	ID     ids.ULID `json:"id"`
	Folder string   `json:"folder"`
}

// FolderArgs changes a folder of a type: with From empty it makes the folder
// To, whose parent must exist; with To empty it deletes From, which must
// hold nothing; with both it renames or moves From, and everything in it,
// to To.
type FolderArgs struct {
	Format string `json:"format"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
}

// TagsArgs sets a document's tags.
type TagsArgs struct {
	ID   ids.ULID `json:"id"`
	Tags []string `json:"tags"`
}

// Items an edit touches, for undo clash detection.
func placeItem(id ids.ULID) string { return "place:" + id.String() }

func tagsItem(id ids.ULID) string { return "tags:" + id.String() }

func folderItem(format, path string) string { return "folder:" + format + ":" + strings.ToLower(path) }

// Move is the edit kind that moves a document into a folder.
type Move struct{}

// Name identifies the edit kind.
func (Move) Name() string { return MoveEdit }

// Format names the document format the edit applies to.
func (Move) Format() string { return FormatName }

// Apply moves the document's entry. Its inverse moves it back.
func (Move) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	index, move, err := decode[MoveArgs](document, args)
	if err != nil {
		return model.Applied{}, err
	}
	position, exists := index.find(move.ID)
	if !exists {
		return model.Applied{}, fmt.Errorf("%w: no document %s", model.ErrArgs, move.ID)
	}
	entry := &index.entries[position]
	folder, found := index.folderPath(entry.Format, move.Folder)
	if !found {
		return model.Applied{}, fmt.Errorf("%w: no folder %q", ErrFolder, move.Folder)
	}
	inverse, err := json.Marshal(MoveArgs{ID: move.ID, Folder: entry.Folder})
	if err != nil {
		return model.Applied{}, fmt.Errorf("encode inverse: %w", err)
	}
	entry.Folder = folder

	return model.Applied{Inverse: inverse, InverseEdit: "", Touched: []string{placeItem(move.ID)}, Covers: nil}, nil
}

// FolderChange is the edit kind that makes, renames, moves or deletes a
// folder.
type FolderChange struct{}

// Name identifies the edit kind.
func (FolderChange) Name() string { return FolderEdit }

// Format names the document format the edit applies to.
func (FolderChange) Format() string { return FormatName }

// Apply changes the folder, and for a rename or move everything in it. Its
// inverse changes it back.
func (FolderChange) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	index, change, err := decode[FolderArgs](document, args)
	if err != nil {
		return model.Applied{}, err
	}
	var undo FolderArgs
	switch {
	case change.Format == "":
		return model.Applied{}, fmt.Errorf("%w: a folder needs a document type", ErrFolder)
	case change.From == "" && change.To != "":
		undo, err = index.makeFolder(change.Format, change.To)
	case change.From != "" && change.To == "":
		undo, err = index.deleteFolder(change.Format, change.From)
	case change.From != "" && change.To != "":
		undo, err = index.moveFolder(change.Format, change.From, change.To)
	default:
		return model.Applied{}, fmt.Errorf("%w: a folder change names a folder", ErrFolder)
	}
	if err != nil {
		return model.Applied{}, err
	}
	slices.SortFunc(index.folders, compareFolders)
	inverse, err := json.Marshal(undo)
	if err != nil {
		return model.Applied{}, fmt.Errorf("encode inverse: %w", err)
	}

	return model.Applied{
		Inverse: inverse, InverseEdit: "", Covers: nil,
		Touched: []string{folderItem(change.Format, change.From), folderItem(change.Format, change.To)},
	}, nil
}

// makeFolder adds a folder inside an existing one, spelling the parent as
// it was made.
func (x *Index) makeFolder(format, path string) (FolderArgs, error) {
	if err := CheckFolderPath(path); err != nil {
		return FolderArgs{}, err
	}
	parent, found := x.folderPath(format, FolderParent(path))
	if !found {
		return FolderArgs{}, fmt.Errorf("%w: no folder %q to make it in", ErrFolder, FolderParent(path))
	}
	path = JoinFolder(parent, FolderName(path))
	if x.HasFolder(format, path) {
		return FolderArgs{}, fmt.Errorf("%w: there is already a folder called %q here", ErrFolder, FolderName(path))
	}
	x.folders = append(x.folders, Folder{Format: format, Path: path})

	return FolderArgs{Format: format, From: path, To: ""}, nil
}

// deleteFolder removes an empty folder.
func (x *Index) deleteFolder(format, path string) (FolderArgs, error) {
	path, found := x.folderPath(format, path)
	if !found {
		return FolderArgs{}, fmt.Errorf("%w: no folder %q", ErrFolder, path)
	}
	if !x.emptyFolder(format, path) {
		return FolderArgs{}, fmt.Errorf("%w: %q is not empty", ErrFolder, path)
	}
	x.folders = slices.DeleteFunc(x.folders, func(folder Folder) bool {
		return folder.Format == format && folder.Path == path
	})

	return FolderArgs{Format: format, From: "", To: path}, nil
}

// moveFolder renames or moves a folder, with the folders and documents in
// it. A folder cannot move into itself, nor onto another folder's name.
func (x *Index) moveFolder(format, from, target string) (FolderArgs, error) {
	from, found := x.folderPath(format, from)
	if !found {
		return FolderArgs{}, fmt.Errorf("%w: no folder %q", ErrFolder, from)
	}
	if err := CheckFolderPath(target); err != nil {
		return FolderArgs{}, err
	}
	parent, found := x.folderPath(format, FolderParent(target))
	if !found {
		return FolderArgs{}, fmt.Errorf("%w: no folder %q to move it to", ErrFolder, FolderParent(target))
	}
	target = JoinFolder(parent, FolderName(target))
	switch {
	case Within(target, from) && !strings.EqualFold(target, from):
		return FolderArgs{}, fmt.Errorf("%w: a folder cannot go inside itself", ErrFolder)
	case x.HasFolder(format, target) && !strings.EqualFold(target, from):
		return FolderArgs{}, fmt.Errorf("%w: there is already a folder called %q there", ErrFolder, FolderName(target))
	}
	for position, folder := range x.folders {
		if folder.Format == format && Within(folder.Path, from) {
			x.folders[position].Path = rebase(folder.Path, from, target)
		}
	}
	for position, entry := range x.entries {
		if entry.Format == format && entry.Folder != "" && Within(entry.Folder, from) {
			x.entries[position].Folder = rebase(entry.Folder, from, target)
		}
	}

	return FolderArgs{Format: format, From: target, To: from}, nil
}

// Retag is the edit kind that sets a document's tags.
type Retag struct{}

// Name identifies the edit kind.
func (Retag) Name() string { return TagsEdit }

// Format names the document format the edit applies to.
func (Retag) Format() string { return FormatName }

// Apply sets the document's tags, tidied. Its inverse sets the old ones.
func (Retag) Apply(document model.Document, args json.RawMessage) (model.Applied, error) {
	index, retag, err := decode[TagsArgs](document, args)
	if err != nil {
		return model.Applied{}, err
	}
	tags := NormalizeTags(retag.Tags)
	if err := CheckTags(tags); err != nil {
		return model.Applied{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	position, exists := index.find(retag.ID)
	if !exists {
		return model.Applied{}, fmt.Errorf("%w: no document %s", model.ErrArgs, retag.ID)
	}
	inverse, err := json.Marshal(TagsArgs{ID: retag.ID, Tags: index.entries[position].Tags})
	if err != nil {
		return model.Applied{}, fmt.Errorf("encode inverse: %w", err)
	}
	index.entries[position].Tags = tags

	return model.Applied{Inverse: inverse, InverseEdit: "", Touched: []string{tagsItem(retag.ID)}, Covers: nil}, nil
}
