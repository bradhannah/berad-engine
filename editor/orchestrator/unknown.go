package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// What the editor does not know (increment 57): a field an agent or a hand
// added to a document's file, which the editor would drop at the next save.
// When the project opens, a lasting notice names each file and field, and
// each such file is copied to the backups before the save that drops what
// it holds.

// Limits on what is said: the fields named for each file and the files
// named in the notice.
const (
	unknownPerFile = 4
	unknownFiles   = 3
)

// backupUnknown is the folder in the backups that keeps files as they were
// before a save dropped what the editor does not know.
const backupUnknown = "unknown-fields"

// envelopeKeys are the keys a document's envelope holds.
func envelopeKeys() []string {
	return []string{"format", "version", "id", "name", "revision", "tags", "changed", "data"}
}

// unknownIn lists what a document's file holds that the editor would not
// write back: keys of the envelope it does not know, and keys of its data
// and its layout's that the decoded document does not encode. A key whose
// value is empty is left out, since the editor may leave out what is empty.
func unknownIn(stored storedDocument, decoded model.Document) []string {
	var found []string
	var envelope map[string]any
	if json.Unmarshal(stored.raw, &envelope) == nil {
		for _, key := range slices.Sorted(maps.Keys(envelope)) {
			if !slices.Contains(envelopeKeys(), key) && !emptyJSON(envelope[key]) {
				found = append(found, key)
			}
		}
	}
	if encoded, err := decoded.Encode(); err == nil {
		found = append(found, droppedKeys(stored.envelope.Data, encoded, "")...)
	}
	if stored.layout != nil {
		if layout, err := model.LayoutOf(decoded); err == nil && layout != nil {
			found = append(found, droppedKeys(stored.layout.envelope.Data, layout, "layout.")...)
		}
	}

	return found
}

// droppedKeys lists the keys in held that encoded lacks, by their path.
func droppedKeys(held, encoded []byte, prefix string) []string {
	var heldTree, encodedTree any
	if json.Unmarshal(held, &heldTree) != nil || json.Unmarshal(encoded, &encodedTree) != nil {
		return nil
	}
	seen := map[string]bool{}
	var found []string
	walkDropped(heldTree, encodedTree, strings.TrimSuffix(prefix, "."), func(path string) {
		if !seen[path] {
			seen[path] = true
			found = append(found, path)
		}
	})

	return found
}

// walkDropped calls found for each key, below under, that held has and encoded
// lacks; arrays are compared item by item, their paths written with [].
func walkDropped(held, encoded any, under string, found func(string)) {
	switch tree := held.(type) {
	case map[string]any:
		other, isMap := encoded.(map[string]any)
		if !isMap {
			other = map[string]any{}
		}
		for _, key := range slices.Sorted(maps.Keys(tree)) {
			path := key
			if under != "" {
				path = under + "." + key
			}
			value, has := other[key]
			if !has {
				if !emptyJSON(tree[key]) {
					found(path)
				}

				continue
			}
			walkDropped(tree[key], value, path, found)
		}
	case []any:
		other, isList := encoded.([]any)
		if !isList {
			return
		}
		for index := range min(len(tree), len(other)) {
			walkDropped(tree[index], other[index], under+"[]", found)
		}
	}
}

// emptyJSON reports whether a JSON value is empty: null, false, 0, "", []
// or {}.
func emptyJSON(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case bool:
		return !typed
	case float64:
		return typed == 0
	case string:
		return typed == ""
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	}

	return false
}

// tellUnknown leaves a notice naming the files that hold what the editor
// does not know, and what in each.
func (o *Orchestrator) tellUnknown() {
	var named []string
	count := 0
	for _, documentID := range sortedIDs(o.docs) {
		entry := o.docs[documentID]
		if len(entry.unknown) == 0 {
			continue
		}
		count++
		if len(named) < unknownFiles {
			fields := entry.unknown[:min(len(entry.unknown), unknownPerFile)]
			said := filepath.Base(entry.info.Path) + " (" + strings.Join(fields, ", ")
			if len(entry.unknown) > unknownPerFile {
				said += ", and " + strconv.Itoa(len(entry.unknown)-unknownPerFile) + " more"
			}
			named = append(named, said+")")
		}
	}
	if count == 0 {
		return
	}
	text := "The next save drops what the editor does not know: " + strings.Join(named, "; ")
	if count > len(named) {
		text += ", and " + strconv.Itoa(count-len(named)) + " more files"
	}
	o.addNotice(protocol.Notice{
		Kind: protocol.NoticeUnknownFields, Window: ids.ULID{}, Report: "", Key: protocol.NoticeUnknownFields,
		Text:    text + ". Each file is copied to the backups first.",
		Actions: []protocol.NoticeAction{dismissButton()}, More: 0,
	})
}

// keepUnknown copies a document's files to the backups before a save drops
// what the editor does not know in them.
func (o *Orchestrator) keepUnknown(entry *documentEntry) error {
	for _, path := range []string{entry.info.Path, layoutPath(entry.info.Path)} {
		data, err := o.project.root.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("keep %s before saving: %w", path, err)
		}
		if _, err := o.project.keepCopy(backupUnknown, path, data, o.cfg.Clock()); err != nil {
			return err
		}
	}

	return nil
}
