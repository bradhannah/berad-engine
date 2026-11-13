package orchestrator

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strings"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// The files are shared (increment 57): with git, agents and the author's own
// hands. Before each save the editor looks at every document's files. One
// changed outside the editor, while nothing for it waits in the editor, is
// taken in, and the windows showing it say so. One changed in both places is
// held, not written, until its author chooses in a notice: Keep mine, or
// Take the file's. Whichever is chosen, the other version goes to the
// project's backups.

// changedKeyPrefix begins the key of the notice about a document whose file
// changed outside the editor while it had changes of its own.
const changedKeyPrefix = "changed:"

// The buttons on that notice.
const (
	keepMine = "keep-mine"
	takeFile = "take-file"
)

// backupChanged is the folder in the backups that keeps the version not
// kept when a file changed in both places.
const backupChanged = "changed"

// errAnotherDocument reports a file that now holds another document.
var errAnotherDocument = errors.New("it now holds another document")

// checkOutside looks at every document's files for changes made outside the
// editor, before a save writes any of them.
func (o *Orchestrator) checkOutside() {
	for _, documentID := range sortedIDs(o.docs) {
		entry := o.docs[documentID]
		if entry.info.Path == "" || entry.standIn || o.held[documentID] {
			continue
		}
		changed, gone := o.changedOutside(entry)
		switch {
		case !changed:
		case o.waiting(entry):
			o.holdChanged(entry, gone, "")
		case !gone:
			o.takeInSaying(entry)
		}
	}
}

// changedOutside reports whether a document's file, or its layout's, changed
// outside the editor, and whether the document's file is gone.
func (o *Orchestrator) changedOutside(entry *documentEntry) (bool, bool) {
	changed, gone := o.project.outside(entry.info.Path)
	if layoutChanged, _ := o.project.outside(layoutPath(entry.info.Path)); layoutChanged && !gone {
		changed = true
	}

	return changed, gone
}

// waiting reports whether something for a document waits in the editor and
// is not in its file: its own changes, or a window's typed into a field.
func (o *Orchestrator) waiting(entry *documentEntry) bool {
	if entry.dirty {
		return true
	}
	for _, open := range o.windows {
		if open.hello.Target == entry.info.ID && open.unsaved.Count > 0 {
			return true
		}
	}

	return false
}

// holdChanged holds a document changed in both places, so nothing writes
// it, and asks its author which to keep. why, when set, is why the file's
// version could not be taken in.
func (o *Orchestrator) holdChanged(entry *documentEntry, gone bool, why string) {
	o.held[entry.info.ID] = true
	text := entry.info.Name + " was changed on disk while it had changes here. " +
		"Whichever you keep, the other version goes to the backups."
	actions := []protocol.NoticeAction{{ID: keepMine, Label: "Keep mine"}, {ID: takeFile, Label: "Take the file's"}}
	switch {
	case gone:
		text = entry.info.Name + "'s file was removed outside the editor while it had changes here. " +
			"Keep mine writes it back; to remove it here too, delete it in the Project window."
		actions = actions[:1]
	case why != "":
		text = entry.info.Name + " was changed on disk, and its file cannot be read: " + why +
			". Keep mine writes over it; its version goes to the backups."
		actions = actions[:1]
	}
	o.addNotice(protocol.Notice{
		Kind: protocol.NoticeChangedOnDisk, Text: text, Window: ids.ULID{}, Report: "",
		Key: changedKeyPrefix + entry.info.ID.String(), Actions: actions, More: 0,
	})
}

// settleChanged carries out its author's choice for a document changed in
// both places, keeps the other version in the backups, and saves.
func (o *Orchestrator) settleChanged(key, action string) {
	documentID, err := ids.Parse(key)
	entry, found := o.docs[documentID]
	if err != nil || !found {
		o.endNotice(changedKeyPrefix + key)

		return
	}
	switch action {
	case keepMine:
		err = o.keepMine(entry)
	case takeFile:
		err = o.takeFile(entry)
	default:
		return
	}
	if err != nil {
		o.cfg.Logger.Error("could not settle a file changed on disk", "document", entry.info.Name, "error", err)
		o.tellSaving(err)

		return
	}
	o.endNotice(changedKeyPrefix + key)
	o.flushLogged()
}

// keepMine keeps the file's version in the backups and lets the editor's be
// written over it.
func (o *Orchestrator) keepMine(entry *documentEntry) error {
	for _, path := range []string{entry.info.Path, layoutPath(entry.info.Path)} {
		data, err := o.project.root.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("keep %s's file: %w", entry.info.Name, err)
		}
		if _, err := o.project.keepCopy(backupChanged, path, data, o.cfg.Clock()); err != nil {
			return err
		}
		o.project.note(path, data)
	}
	delete(o.held, entry.info.ID)
	entry.dirty = true

	return nil
}

// takeFile keeps the editor's version in the backups and takes in the
// file's. It is written again, so the journal's changes to the editor's
// version are never laid over the file's.
func (o *Orchestrator) takeFile(entry *documentEntry) error {
	data, err := entry.document.Encode()
	if err != nil {
		return fmt.Errorf("keep %s: %w", entry.info.Name, err)
	}
	if _, err := o.project.keepCopy(backupChanged, entry.info.Path, data, o.cfg.Clock()); err != nil {
		return err
	}
	if err := o.takeIn(entry); err != nil {
		return err
	}
	delete(o.held, entry.info.ID)
	entry.dirty = true
	o.sendSnapshot(entry, "Took in "+entry.info.Name+" as changed on disk; your version is in the backups")

	return nil
}

// takeInSaying takes in a document's file as changed outside the editor, and
// tells the windows showing it, by the name in the file. A file that cannot
// be read is asked about instead.
func (o *Orchestrator) takeInSaying(entry *documentEntry) {
	if err := o.takeIn(entry); err != nil {
		o.cfg.Logger.Warn("a file changed on disk cannot be read", "path", entry.info.Path, "error", err)
		o.holdChanged(entry, false, plainReason(err))

		return
	}
	o.sendSnapshot(entry, "Took in "+entry.info.Name+" as changed on disk")
}

// takeIn reads a document's files as they are now on disk, in place of the
// editor's version. Its undo steps go, since they were for the version it
// replaces.
func (o *Orchestrator) takeIn(entry *documentEntry) error {
	raw, err := o.project.root.ReadFile(entry.info.Path)
	if err != nil {
		return fmt.Errorf("read %s: %w", entry.info.Path, err)
	}
	upgraded, changed, err := o.readEnvelope(raw)
	if err != nil {
		return err
	}
	if upgraded.ID != entry.info.ID {
		return errAnotherDocument
	}
	layoutRaw, layout, err := o.readLayout(entry.info.Path)
	if err != nil {
		return err
	}
	documentType, ok := o.cfg.Registry.Document(upgraded.Format)
	if !ok {
		return fmt.Errorf("%w: %s", content.ErrUnknownFormat, upgraded.Format)
	}
	decoded, err := documentType.DecodeWithLayout(upgraded.Data, layout)
	if err != nil {
		return fmt.Errorf("decode %s: %w", entry.info.Path, err)
	}
	entry.document, entry.version = decoded, upgraded.Version
	entry.info.Name, entry.info.Tags, entry.info.Changed = upgraded.Name, upgraded.Tags, upgraded.Changed
	entry.info.Revision = max(entry.info.Revision, upgraded.Revision)
	entry.dirty = changed
	o.project.note(entry.info.Path, raw)
	if layoutRaw != nil {
		o.project.note(layoutPath(entry.info.Path), layoutRaw)
	}
	o.history.forget(entry.info.ID)
	o.buildIndex()

	return nil
}

// readEnvelope reads a document file's envelope, upgraded to the current
// version, and whether it was upgraded.
func (o *Orchestrator) readEnvelope(raw []byte) (content.Envelope, bool, error) {
	envelope, err := content.Decode(raw)
	if err != nil {
		return content.Envelope{}, false, fmt.Errorf("decode: %w", err)
	}
	upgraded, changed, err := o.cfg.Registry.Formats().Upgrade(envelope)
	if err != nil {
		return content.Envelope{}, false, fmt.Errorf("upgrade: %w", err)
	}

	return upgraded, changed || upgraded.Version != envelope.Version, nil
}

// readLayout reads the layout file beside a document's, when there is one:
// its bytes and its data.
func (o *Orchestrator) readLayout(path string) ([]byte, []byte, error) {
	raw, err := o.project.root.ReadFile(layoutPath(path))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", layoutPath(path), err)
	}
	upgraded, _, err := o.readEnvelope(raw)
	if err != nil {
		return nil, nil, err
	}

	return raw, upgraded.Data, nil
}

// snapshotOf is a document as a window is sent it.
func snapshotOf(entry *documentEntry) (protocol.Snapshot, error) {
	data, err := entry.document.Encode()
	if err != nil {
		return protocol.Snapshot{}, fmt.Errorf("encode %s: %w", entry.info.Name, err)
	}
	layout, err := model.LayoutOf(entry.document)
	if err != nil {
		return protocol.Snapshot{}, fmt.Errorf("layout of %s: %w", entry.info.Name, err)
	}

	return protocol.Snapshot{
		Document: entry.info.ID, Format: entry.info.Format, Name: entry.info.Name, Revision: entry.info.Revision,
		Data: data, Layout: layout, Said: "",
	}, nil
}

// sendSnapshot sends a document again to every window showing it, saying
// why.
func (o *Orchestrator) sendSnapshot(entry *documentEntry, said string) {
	snapshot, err := snapshotOf(entry)
	if err != nil {
		o.cfg.Logger.Error("could not send a document again", "document", entry.info.Name, "error", err)

		return
	}
	snapshot.Said = said
	message := mustMessage(protocol.KindSnapshot, 0, snapshot)
	for _, subscriber := range o.subscribed[entry.info.ID] {
		o.send(subscriber, message)
	}
}

// sortedIDs lists the documents' IDs in order.
func sortedIDs(docs map[ids.ULID]*documentEntry) []ids.ULID {
	return slices.SortedFunc(maps.Keys(docs), compareIDs)
}

// compareIDs orders documents by ID, so they are looked at in the same order
// every time.
func compareIDs(first, second ids.ULID) int { return strings.Compare(first.String(), second.String()) }
