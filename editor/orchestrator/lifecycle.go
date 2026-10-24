package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/bradhannah/berad-engine/editor/model"
	projectindex "github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// ErrNameTaken reports a document name already used in its scope (D4).
var ErrNameTaken = errors.New("name already used")

// ErrNoProjectPlugin reports a plug-in list without the project index.
var ErrNoProjectPlugin = errors.New("the plug-in list must include the project index")

// isIndex reports whether an ID is the project index.
func isIndex(id ids.ULID) bool { return id == projectindex.IndexID() }

// buildIndex creates the in-memory project index from the loaded documents
// and folders.
func (o *Orchestrator) buildIndex() {
	index := o.indexNow()
	o.docs[projectindex.IndexID()] = &documentEntry{
		info: DocumentInfo{
			ID: projectindex.IndexID(), Name: "Project", Format: projectindex.FormatName, Revision: 0, Path: "",
			Folder: "", Tags: nil,
		},
		version:  1,
		document: index,
		dirty:    false,
	}
}

// prepareLifecycle completes a project index edit with what only the
// orchestrator knows: a new document's ID, or everything needed to recreate a
// deleted one. It refuses names already used in their scope.
func (o *Orchestrator) prepareLifecycle(edit string, args json.RawMessage) (json.RawMessage, ids.ULID, error) {
	switch edit {
	case projectindex.CreateEdit:
		return o.prepareCreate(args)
	case projectindex.DeleteEdit:
		prepared, err := o.prepareDelete(args)

		return prepared, ids.ULID{}, err
	case projectindex.RenameEdit:
		var rename projectindex.RenameArgs
		if err := json.Unmarshal(args, &rename); err != nil {
			return nil, ids.ULID{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
		}
		entry, ok := o.docs[rename.ID]
		if !ok || isIndex(rename.ID) {
			return nil, ids.ULID{}, fmt.Errorf("%w: %s", ErrUnknownDocument, rename.ID)
		}
		if err := o.checkName(entry.info.Format, rename.Name, rename.ID, o.containerOf(rename.ID)); err != nil {
			return nil, ids.ULID{}, err
		}

		return args, ids.ULID{}, nil
	case projectindex.MoveEdit, projectindex.FolderEdit:
		return args, ids.ULID{}, o.prepareFolders(edit, args)
	default:
		return args, ids.ULID{}, nil
	}
}

func (o *Orchestrator) prepareCreate(args json.RawMessage) (json.RawMessage, ids.ULID, error) {
	var create projectindex.DocumentArgs
	if err := json.Unmarshal(args, &create); err != nil {
		return nil, ids.ULID{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	documentType, ok := o.cfg.Registry.Document(create.Format)
	if !ok || create.Format == projectindex.FormatName {
		return nil, ids.ULID{}, fmt.Errorf("%w: cannot create a %q document", model.ErrArgs, create.Format)
	}
	if documentType.Prepare != nil {
		data, err := documentType.Prepare(create.Data, o.ids.New)
		if err != nil {
			return nil, ids.ULID{}, fmt.Errorf("prepare %s: %w", create.Format, err)
		}
		create.Data = data
	}
	if _, err := documentType.DecodeWithLayout(create.Data, create.Layout); err != nil {
		return nil, ids.ULID{}, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	var created ids.ULID
	if create.ID.IsZero() {
		var err error
		create.ID, err = o.ids.New()
		if err != nil {
			return nil, ids.ULID{}, fmt.Errorf("document id: %w", err)
		}
		created = create.ID
	}
	if err := o.checkName(create.Format, create.Name, create.ID, create.Scope); err != nil {
		return nil, ids.ULID{}, err
	}
	if create.Folder != "" && !create.Scope.IsZero() {
		return nil, ids.ULID{}, fmt.Errorf("%w: it stays with the document it is part of", projectindex.ErrFolder)
	}
	create.Version = documentType.Format.Current
	prepared, err := json.Marshal(create)
	if err != nil {
		return nil, ids.ULID{}, fmt.Errorf("encode create: %w", err)
	}

	return prepared, created, nil
}

func (o *Orchestrator) prepareDelete(args json.RawMessage) (json.RawMessage, error) {
	var deletion projectindex.DocumentArgs
	if err := json.Unmarshal(args, &deletion); err != nil {
		return nil, fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	entry, ok := o.docs[deletion.ID]
	if !ok || isIndex(deletion.ID) {
		return nil, fmt.Errorf("%w: %s", ErrUnknownDocument, deletion.ID)
	}
	data, err := entry.document.Encode()
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", entry.info.Name, err)
	}
	layout, err := model.LayoutOf(entry.document)
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", entry.info.Name, err)
	}
	deletion = projectindex.DocumentArgs{
		ID: deletion.ID, Format: entry.info.Format, Name: entry.info.Name, Version: entry.version,
		Revision: entry.info.Revision, Scope: o.containerOf(deletion.ID), Folder: entry.info.Folder,
		Tags: entry.info.Tags, Data: data, Layout: layout,
	}
	prepared, err := json.Marshal(deletion)
	if err != nil {
		return nil, fmt.Errorf("encode delete: %w", err)
	}

	return prepared, nil
}

// checkName refuses a name that is malformed or already used by another
// document of the same format in the same scope: the container that lists the
// document, or no container.
func (o *Orchestrator) checkName(format, name string, self, scope ids.ULID) error {
	if err := projectindex.CheckName(name); err != nil {
		return fmt.Errorf("%w: %w", model.ErrArgs, err)
	}
	for id, entry := range o.docs {
		if id == self || isIndex(id) || entry.info.Format != format {
			continue
		}
		if o.containerOf(id) == scope && strings.EqualFold(entry.info.Name, name) {
			return fmt.Errorf("%w: another %s is already called %q", ErrNameTaken, format, entry.info.Name)
		}
	}

	return nil
}

// containerOf returns the document that lists a document as a part, or the
// zero ID.
func (o *Orchestrator) containerOf(documentID ids.ULID) ids.ULID {
	for containerID, entry := range o.docs {
		container, ok := entry.document.(model.Container)
		if !ok {
			continue
		}
		for _, part := range container.Parts() {
			if part == documentID {
				return containerID
			}
		}
	}

	return ids.ULID{}
}

// applyLifecycle carries out a journaled project index edit: it creates,
// removes, renames, moves or tags the document itself, or changes a folder.
// Files change at the next save.
func (o *Orchestrator) applyLifecycle(edit string, args json.RawMessage, live bool) error {
	switch edit {
	case projectindex.MoveEdit, projectindex.FolderEdit, projectindex.TagsEdit:
		if index, built := o.liveIndex(); built {
			o.syncFromIndex(index)
		}
	case projectindex.CreateEdit, projectindex.DeleteEdit:
		var change projectindex.DocumentArgs
		if err := json.Unmarshal(args, &change); err != nil {
			return fmt.Errorf("%w: %w", model.ErrArgs, err)
		}
		if edit == projectindex.CreateEdit {
			return o.createDocument(change)
		}
		o.removeDocument(change.ID, live)
	case projectindex.RenameEdit:
		var rename projectindex.RenameArgs
		if err := json.Unmarshal(args, &rename); err != nil {
			return fmt.Errorf("%w: %w", model.ErrArgs, err)
		}
		o.renameDocument(rename.ID, rename.Name)
	}

	return nil
}

func (o *Orchestrator) createDocument(create projectindex.DocumentArgs) error {
	documentType, ok := o.cfg.Registry.Document(create.Format)
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownDocument, create.Format)
	}
	document, err := documentType.DecodeWithLayout(create.Data, create.Layout)
	if err != nil {
		return fmt.Errorf("recreate %s: %w", create.Name, err)
	}
	path := documentPath(create.Format, create.Folder, create.Name, create.ID)
	o.keepFiles(path)
	o.docs[create.ID] = &documentEntry{
		info: DocumentInfo{
			ID: create.ID, Name: create.Name, Format: create.Format, Revision: create.Revision, Path: path,
			Folder: create.Folder, Tags: create.Tags,
		},
		version:  documentType.Format.Current,
		document: document,
		dirty:    true,
	}

	return nil
}

// removeDocument forgets a document; its file is moved to the backups at the
// next save. Windows showing it close, since there is nothing left to show.
func (o *Orchestrator) removeDocument(documentID ids.ULID, live bool) {
	entry, ok := o.docs[documentID]
	if !ok {
		return
	}
	delete(o.docs, documentID)
	o.retireFiles(entry.info.Path)
	delete(o.subscribed, documentID)
	if !live {
		return
	}
	for _, connected := range o.windows {
		if connected.hello.Target == documentID {
			o.send(connected, mustMessage(protocol.KindShutdown, 0, protocol.Shutdown{
				Save:   false,
				Reason: "“" + entry.info.Name + "” was deleted",
			}))
		}
	}
}

func (o *Orchestrator) renameDocument(documentID ids.ULID, name string) {
	entry, ok := o.docs[documentID]
	if !ok || entry.info.Name == name {
		return
	}
	entry.info.Name = name
	o.repath(entry)
}

// retireFiles marks a document's file, and its layout file if it has one, to
// move to the backups at the next save.
func (o *Orchestrator) retireFiles(path string) {
	o.removed[path] = true
	o.removed[layoutPath(path)] = true
}

// keepFiles cancels retiring a document's files, for a document that is
// back at that path.
func (o *Orchestrator) keepFiles(path string) {
	delete(o.removed, path)
	delete(o.removed, layoutPath(path))
}

// replayLifecycle applies a journaled project index edit during recovery. It
// is idempotent: the documents' files may already reflect the edit if the
// editor stopped between saving them and clearing the journal.
func (o *Orchestrator) replayLifecycle(entry journalEntry) error {
	switch entry.Edit {
	case projectindex.CreateEdit:
		var create projectindex.DocumentArgs
		if err := json.Unmarshal(entry.Args, &create); err != nil {
			return fmt.Errorf("replay create: %w", err)
		}
		if _, exists := o.docs[create.ID]; exists {
			return nil
		}
	case projectindex.DeleteEdit, projectindex.RenameEdit:
	case projectindex.MoveEdit, projectindex.FolderEdit, projectindex.TagsEdit:
		return o.replayFolders(entry)
	default:
		return fmt.Errorf("replay: %w: %s", ErrUnknownEdit, entry.Edit)
	}

	return o.applyLifecycle(entry.Edit, entry.Args, false)
}
