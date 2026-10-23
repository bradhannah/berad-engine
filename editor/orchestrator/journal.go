package orchestrator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/bradhannah/berad-engine/engine/ids"
)

const journalVersion = 1

// ErrJournalVersion reports a journal written by a newer program.
var ErrJournalVersion = errors.New("journal is newer than this program supports")

// ErrJournalCorrupt reports an unreadable journal entry that is not the last.
var ErrJournalCorrupt = errors.New("journal entry is corrupt")

// journalEntry records one applied edit. Every entry carries a format version.
type journalEntry struct {
	Version  int             `json:"v"`
	Seq      uint64          `json:"seq"`
	Document ids.ULID        `json:"document"`
	Revision uint64          `json:"revision"`
	Edit     string          `json:"edit"`
	Args     json.RawMessage `json:"args"`
}

// journal is an append-only log of applied edits. Appends reach the operating
// system before an edit is acknowledged, so killing the orchestrator loses
// nothing; sync makes them durable against power loss and runs periodically.
type journal struct {
	root    *os.Root
	file    *os.File
	pending bool
}

func openJournal(root *os.Root) (*journal, error) {
	file, err := root.OpenFile(journalPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, filePerm)
	if err != nil {
		return nil, fmt.Errorf("open journal: %w", err)
	}

	return &journal{root: root, file: file, pending: false}, nil
}

func (j *journal) append(entry journalEntry) error {
	entry.Version = journalVersion
	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode journal entry: %w", err)
	}
	if _, err := j.file.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("append journal: %w", err)
	}
	j.pending = true

	return nil
}

func (j *journal) sync() error {
	if !j.pending {
		return nil
	}
	if err := j.file.Sync(); err != nil {
		return fmt.Errorf("sync journal: %w", err)
	}
	j.pending = false

	return nil
}

// entries reads the journal. A final line cut short by a crash is ignored.
func (j *journal) entries() ([]journalEntry, error) {
	raw, err := j.root.ReadFile(journalPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read journal: %w", err)
	}
	lines := bytes.Split(raw, []byte{'\n'})
	entries := make([]journalEntry, 0, len(lines))
	for index, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var entry journalEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			if index == len(lines)-1 {
				break
			}

			return nil, fmt.Errorf("%w: line %d: %w", ErrJournalCorrupt, index+1, err)
		}
		if entry.Version > journalVersion {
			return nil, fmt.Errorf("%w: version %d", ErrJournalVersion, entry.Version)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// reset empties the journal after every document has been saved to its file.
func (j *journal) reset() error {
	if err := j.file.Close(); err != nil {
		return fmt.Errorf("close journal: %w", err)
	}
	temp := journalPath + tempSuffix
	if err := j.root.WriteFile(temp, nil, filePerm); err != nil {
		return fmt.Errorf("reset journal: %w", err)
	}
	if err := j.root.Rename(temp, journalPath); err != nil {
		return fmt.Errorf("reset journal: %w", err)
	}
	file, err := j.root.OpenFile(journalPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, filePerm)
	if err != nil {
		return fmt.Errorf("reopen journal: %w", err)
	}
	j.file = file
	j.pending = false

	return nil
}

func (j *journal) close() error {
	syncErr := j.sync()
	closeErr := j.file.Close()
	if err := errors.Join(syncErr, closeErr); err != nil {
		return fmt.Errorf("close journal: %w", err)
	}

	return nil
}
