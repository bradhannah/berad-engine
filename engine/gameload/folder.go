package gameload

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// What a project folder holds beside its documents: the editor's own folder
// and any other whose name starts with a dot, the audio folder, which keeps
// berad-audio's formats, and the files that hold a document's layout.
const (
	audioDir     = "audio"
	documentExt  = ".json"
	layoutSuffix = ".layout.json"
)

// Unread is a file of a project folder that looked like a document and
// could not be read, and why.
type Unread struct {
	Path   string
	Reason string
}

// Folder is a project folder as a Source: every document a game reads, read
// once. Nothing in it waits. A file in an older format is upgraded as it is
// read, and nothing is ever written.
type Folder struct {
	documents map[ids.ULID]folderDocument
	unread    []Unread
}

// folderDocument is one document of a folder: where it was read from, and
// its revision, which tells two files holding one document apart.
type folderDocument struct {
	path     string
	name     string
	format   string
	revision uint64
	value    any
}

// decoder reads a format's data as the engine holds it.
type decoder func(data json.RawMessage) (any, error)

// reads is a format a game reads, and how.
type reads struct {
	format content.Format
	decode decoder
}

// decoding wraps one of the engine's decoders.
func decoding[T any](decode func(json.RawMessage) (T, error)) decoder {
	return func(data json.RawMessage) (any, error) { return decode(data) }
}

// formats lists every format a game reads. A document of any other format,
// such as the views an author saved, is the editor's and is passed over.
func formats() []reads {
	return []reads{
		{format: game.SettingsFormat(), decode: decoding(game.DecodeSettings)},
		{format: zone.Format(), decode: decoding(zone.Decode)},
		{format: tilefloor.Format(), decode: decoding(tilefloor.Decode)},
		{format: npc.Format(), decode: decoding(npc.Decode)},
		{format: flow.Format(), decode: decoding(flow.Decode)},
		{format: worldstate.Format(), decode: decoding(worldstate.Decode)},
		{format: art.SettingsFormat(), decode: decoding(art.DecodeSettings)},
		{format: art.StandInsFormat(), decode: decoding(art.DecodeStandInSprites)},
		{format: art.SpriteFormat(), decode: decoding(art.DecodeSprite)},
		{format: art.TileFormat(), decode: decoding(art.DecodeTile)},
		{format: art.PaletteFormat(), decode: decoding(art.DecodePalette)},
	}
}

// folderReader reads a folder's files into a Folder.
type folderReader struct {
	files    fs.FS
	registry *content.Registry
	decoders map[string]decoder
	folder   *Folder
}

// ReadFolder reads a project folder. A file that claims to be a document
// and cannot be read as one is left out and listed by Unread; an error is one
// that stops the folder being read at all.
func ReadFolder(files fs.FS) (*Folder, error) {
	reader := &folderReader{
		files: files, registry: content.NewRegistry(), decoders: map[string]decoder{},
		folder: &Folder{documents: map[ids.ULID]folderDocument{}, unread: nil},
	}
	for _, read := range formats() {
		if err := reader.registry.Register(read.format); err != nil {
			return nil, fmt.Errorf("project formats: %w", err)
		}
		reader.decoders[read.format.Name] = read.decode
	}
	if err := fs.WalkDir(files, ".", reader.walk); err != nil {
		return nil, fmt.Errorf("read project folder: %w", err)
	}
	if err := reader.folder.holdWhatEveryProjectHolds(); err != nil {
		return nil, err
	}

	return reader.folder, nil
}

// walk reads one entry of the folder.
func (r *folderReader) walk(filePath string, entry fs.DirEntry, walkErr error) error {
	switch {
	case walkErr != nil && filePath == ".":
		return walkErr
	case walkErr != nil:
		r.leaveOut(filePath, walkErr.Error())
		if entry != nil && entry.IsDir() {
			return fs.SkipDir
		}

		return nil
	case entry.IsDir():
		if filePath != "." && (strings.HasPrefix(entry.Name(), ".") || filePath == audioDir) {
			return fs.SkipDir
		}

		return nil
	case path.Ext(filePath) != documentExt || strings.HasSuffix(filePath, layoutSuffix):
		return nil
	}
	r.read(filePath)

	return nil
}

// read reads one file as a document, keeping the newer of two files that
// hold the same one.
func (r *folderReader) read(filePath string) {
	raw, err := fs.ReadFile(r.files, filePath)
	if err != nil {
		r.leaveOut(filePath, err.Error())

		return
	}
	envelope, err := content.Decode(raw)
	if err != nil {
		r.leaveOut(filePath, strings.TrimPrefix(err.Error(), content.ErrMalformed.Error()+": "))

		return
	}
	upgraded, _, err := r.registry.Upgrade(envelope)
	if errors.Is(err, content.ErrUnknownFormat) {
		return
	}
	if err != nil {
		r.leaveOut(filePath, err.Error())

		return
	}
	value, err := r.decoders[upgraded.Format](upgraded.Data)
	if err != nil {
		r.leaveOut(filePath, err.Error())

		return
	}
	read := folderDocument{
		path: filePath, name: upgraded.Name, format: upgraded.Format, revision: upgraded.Revision, value: value,
	}
	if other, twice := r.folder.documents[upgraded.ID]; twice && !r.newer(read, other) {
		return
	}
	r.folder.documents[upgraded.ID] = read
}

// newer reports whether a file holds a newer copy of a document than
// another: the higher revision, or the file written later.
func (r *folderReader) newer(read, other folderDocument) bool {
	if read.revision != other.revision {
		return read.revision > other.revision
	}
	readInfo, readErr := fs.Stat(r.files, read.path)
	otherInfo, otherErr := fs.Stat(r.files, other.path)

	return readErr == nil && otherErr == nil && readInfo.ModTime().After(otherInfo.ModTime())
}

func (r *folderReader) leaveOut(filePath, reason string) {
	r.folder.unread = append(r.folder.unread, Unread{Path: filePath, Reason: reason})
}

// holdWhatEveryProjectHolds gives the folder the documents every project
// holds that it lacks, as the editor adds them when it opens a project: the
// game settings, the art settings, the record of stand-in sprites and the
// world state, each as a new project has it. Default, the engine's palette,
// is always the engine's, whatever a file says.
func (f *Folder) holdWhatEveryProjectHolds() error {
	palette, err := art.DefaultPalette()
	if err != nil {
		return fmt.Errorf("default palette: %w", err)
	}
	f.documents[art.DefaultPaletteID()] = folderDocument{
		path: "", name: art.DefaultPaletteName, format: art.PaletteFormatName, revision: 0, value: palette,
	}
	for _, each := range []struct {
		id    ids.ULID
		added folderDocument
	}{
		{id: game.SettingsID(), added: folderDocument{
			path: "", name: game.SettingsName, format: game.SettingsFormatName, revision: 0, value: game.DefaultSettings(),
		}},
		{id: art.SettingsID(), added: folderDocument{
			path: "", name: art.SettingsName, format: art.SettingsFormatName, revision: 0, value: art.DefaultSettings(),
		}},
		{id: art.StandInsID(), added: folderDocument{
			path: "", name: art.StandInsName, format: art.StandInsFormatName, revision: 0, value: art.NewStandInSprites(),
		}},
		{id: worldstate.ID(), added: folderDocument{
			path: "", name: worldstate.Name, format: worldstate.FormatName, revision: 0, value: worldstate.NewDocument(),
		}},
	} {
		if _, held := f.documents[each.id]; !held {
			f.documents[each.id] = each.added
		}
	}

	return nil
}

// Unread lists the files that looked like documents and could not be read,
// in the order they were found.
func (f *Folder) Unread() []Unread { return slices.Clone(f.unread) }

// View shows a document the folder holds.
func (f *Folder) View(documentID ids.ULID, show func(Document)) State {
	document, held := f.documents[documentID]
	if !held {
		return Missing
	}
	show(Document{Name: document.name, Value: document.value})

	return Held
}

// Name names a document, "" for one the folder lacks.
func (f *Folder) Name(documentID ids.ULID) string { return f.documents[documentID].name }

// List lists the documents of a format, ordered by name.
func (f *Folder) List(format string) []ids.ULID {
	var listed []ids.ULID
	for documentID, document := range f.documents {
		if document.format == format {
			listed = append(listed, documentID)
		}
	}
	slices.SortFunc(listed, func(a, b ids.ULID) int {
		return cmp.Or(
			strings.Compare(strings.ToLower(f.documents[a].name), strings.ToLower(f.documents[b].name)), a.Compare(b),
		)
	})

	return listed
}
