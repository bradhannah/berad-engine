package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// ErrRequired reports an attempt to delete or rename a document every project
// holds.
var ErrRequired = errors.New("every project holds this document")

// ErrInUse reports an edit that would remove something other documents still
// refer to.
var ErrInUse = errors.New("still in use")

// ErrReadOnly reports an edit to a document the engine makes, which nothing
// changes.
var ErrReadOnly = errors.New("read only")

// addRequired adds the documents every project holds that this one lacks.
// They are saved at the next save, and since their data is the same every
// time, a project reopened before then adds the same documents again. A
// read-only document is made again every time, so it is always the engine's.
func (o *Orchestrator) addRequired() error {
	for _, required := range o.cfg.Registry.Required() {
		held, holds := o.docs[required.ID]
		if holds && !required.ReadOnly {
			continue
		}
		documentType, ok := o.cfg.Registry.Document(required.Format)
		if !ok {
			return fmt.Errorf("%w: %s", ErrUnknownDocument, required.Format)
		}
		data, err := required.Data()
		if err != nil {
			return fmt.Errorf("required %s: %w", required.Name, err)
		}
		document, err := documentType.Decode(data)
		if err != nil {
			return fmt.Errorf("required %s: %w", required.Name, err)
		}
		if holds {
			o.restoreReadOnly(held, document, data)

			continue
		}
		o.docs[required.ID] = &documentEntry{
			info: DocumentInfo{
				ID: required.ID, Name: required.Name, Format: required.Format, Revision: 0,
				Path: documentPath(required.Format, "", required.Name, required.ID), Folder: "", Tags: nil,
			},
			version:  documentType.Format.Current,
			document: document,
			dirty:    true,
		}
		o.cfg.Logger.Info("added a document every project holds", "name", required.Name)
	}

	return nil
}

// restoreReadOnly puts back the engine's data for a read-only document whose
// saved copy differs, keeping its folder and tags.
func (o *Orchestrator) restoreReadOnly(held *documentEntry, document model.Document, data json.RawMessage) {
	saved, err := held.document.Encode()
	if err == nil && string(saved) == string(data) {
		return
	}
	held.document = document
	held.dirty = true
	o.cfg.Logger.Info("restored a document the engine makes", "name", held.info.Name)
}

// refuseReadOnly refuses any edit to a document the engine makes.
func (o *Orchestrator) refuseReadOnly(documentID ids.ULID) error {
	if !o.cfg.Registry.IsReadOnly(documentID) {
		return nil
	}
	name := documentID.String()
	if entry, ok := o.docs[documentID]; ok {
		name = entry.info.Name
	}

	return fmt.Errorf("%w: %s is the engine's and never changes", ErrReadOnly, name)
}

// refuseRequired refuses deleting or renaming a document every project holds.
func (o *Orchestrator) refuseRequired(documentID ids.ULID) error {
	if !o.cfg.Registry.IsRequired(documentID) {
		return nil
	}
	name := documentID.String()
	if entry, ok := o.docs[documentID]; ok {
		name = entry.info.Name
	}

	return fmt.Errorf("%w: %s", ErrRequired, name)
}

// checkRemoval refuses an edit that would remove items other documents still
// refer to, naming the documents that use them. A part, such as a sprite's
// tile, is named by the document it is part of.
func (o *Orchestrator) checkRemoval(
	documentID ids.ULID, document model.Document, remover model.Remover, args json.RawMessage,
) error {
	items, err := remover.Removes(document, args)
	if err != nil {
		return fmt.Errorf("removes: %w", err)
	}
	var users []string
	for _, item := range items {
		users = o.users(documentID, item, users)
	}
	if len(users) > 0 {
		return fmt.Errorf("%w: used by %s", ErrInUse, strings.Join(users, ", "))
	}

	return nil
}

// refuseKept refuses deleting a document that is kept while used, naming
// the documents that still refer to it.
func (o *Orchestrator) refuseKept(documentID ids.ULID, document model.Document) error {
	kept, ok := document.(model.KeptWhileUsed)
	if !ok || !kept.KeptWhileUsed() {
		return nil
	}
	if users := o.users(documentID, "", nil); len(users) > 0 {
		return fmt.Errorf("%w: used by %s", ErrInUse, strings.Join(users, ", "))
	}

	return nil
}

// users adds the names of the documents that refer to a document, or to one
// item in it, to a list. A part is named by the document it is part of.
func (o *Orchestrator) users(documentID ids.ULID, item string, users []string) []string {
	question := protocol.Query{Question: protocol.QuestionWhereUsed, Document: documentID, Item: item}
	for _, use := range o.whereUsed(question) {
		name := use.Name
		if container := o.containerOf(use.Document); !container.IsZero() {
			if entry, ok := o.docs[container]; ok {
				name = entry.info.Name
			}
		}
		if !slices.Contains(users, name) {
			users = append(users, name)
		}
	}

	return users
}

// refusedRemoval reports a step whose undo or redo would remove something
// still in use. Undo and redo check before changing anything, so a refused
// step stays where it was and can be tried again once nothing uses it.
func (o *Orchestrator) refusedRemoval(actions []action, undoing bool) error {
	for _, step := range actions {
		editName, args := step.edit, step.forward
		if undoing {
			editName, args = step.inverseEdit(), step.back
		}
		kind, known := o.cfg.Registry.Edit(editName)
		remover, removes := kind.(model.Remover)
		entry, found := o.docs[step.document]
		if !known || !removes || !found {
			continue
		}
		if err := o.checkRemoval(step.document, entry.document, remover, args); err != nil {
			return fmt.Errorf("%s refused: %w", editName, err)
		}
	}

	return nil
}
