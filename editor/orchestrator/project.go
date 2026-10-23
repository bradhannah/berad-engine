package orchestrator

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
)

const (
	stateDir     = ".berad"
	journalPath  = ".berad/journal.log"
	backupDir    = ".berad/backups"
	logDir       = ".berad/logs"
	lockPath     = ".berad/orchestrator.lock"
	dirPerm      = 0o750
	filePerm     = 0o600
	shortIDChars = 8
	tempSuffix   = ".tmp"
	stampLayout  = "20060102T150405"
)

// ErrDuplicateID reports two files holding the same document ID.
var ErrDuplicateID = errors.New("two files hold the same document ID")

// project reads and writes a project folder through an os.Root, so no path can
// escape it.
type project struct {
	root   *os.Root
	logger *slog.Logger
}

// storedDocument is a document read from disk, already upgraded if needed.
type storedDocument struct {
	path        string
	envelope    content.Envelope
	fromVersion int
}

func openProject(dir string, logger *slog.Logger) (*project, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open project %s: %w", dir, err)
	}
	for _, sub := range []string{stateDir, backupDir, logDir} {
		if err := root.MkdirAll(sub, dirPerm); err != nil {
			return nil, fmt.Errorf("create %s: %w", sub, err)
		}
	}

	return &project{root: root, logger: logger}, nil
}

func (p *project) close() error {
	if err := p.root.Close(); err != nil {
		return fmt.Errorf("close project: %w", err)
	}

	return nil
}

// loadDocuments reads every document envelope in the project, skipping the
// state folder and hidden folders. Old documents are backed up, then upgraded
// in memory; the caller saves the upgraded form.
func (p *project) loadDocuments(formats *content.Registry, now time.Time) ([]storedDocument, error) {
	var documents []storedDocument
	seen := map[ids.ULID]string{}
	walk := func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != "." && strings.HasPrefix(entry.Name(), ".") {
				return fs.SkipDir
			}

			return nil
		}
		if filepath.Ext(path) != ".json" {
			return nil
		}
		document, ok, err := p.loadOne(formats, path, now)
		if err != nil || !ok {
			return err
		}
		if other, duplicate := seen[document.envelope.ID]; duplicate {
			return fmt.Errorf("%w: %s and %s", ErrDuplicateID, other, path)
		}
		seen[document.envelope.ID] = path
		documents = append(documents, document)

		return nil
	}
	if err := fs.WalkDir(p.root.FS(), ".", walk); err != nil {
		return nil, fmt.Errorf("load documents: %w", err)
	}

	return documents, nil
}

func (p *project) loadOne(formats *content.Registry, path string, now time.Time) (storedDocument, bool, error) {
	raw, err := p.root.ReadFile(path)
	if err != nil {
		return storedDocument{}, false, fmt.Errorf("read %s: %w", path, err)
	}
	envelope, err := content.Decode(raw)
	if err != nil {
		p.logger.Warn("skipping file that is not a document", "path", path, "error", err)

		return noDocument(), false, nil
	}
	upgraded, changed, err := formats.Upgrade(envelope)
	if errors.Is(err, content.ErrUnknownFormat) {
		p.logger.Warn("skipping document with unknown format", "path", path, "format", envelope.Format)

		return noDocument(), false, nil
	}
	if err != nil {
		return storedDocument{}, false, fmt.Errorf("%s: %w", path, err)
	}
	if changed {
		if err := p.backup(path, raw, envelope.Version, now); err != nil {
			return storedDocument{}, false, err
		}
	}

	return storedDocument{path: path, envelope: upgraded, fromVersion: envelope.Version}, true, nil
}

// writeDocument saves an envelope by writing a temporary file, syncing it and
// renaming it into place, so a crash never leaves a half-written document.
func (p *project) writeDocument(path string, envelope content.Envelope) error {
	encoded, err := content.Encode(envelope)
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}

	return p.writeAtomic(path, encoded)
}

func (p *project) writeAtomic(path string, data []byte) error {
	if err := p.root.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return fmt.Errorf("create folder for %s: %w", path, err)
	}
	temp := path + tempSuffix
	file, err := p.root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, filePerm)
	if err != nil {
		return fmt.Errorf("create %s: %w", temp, err)
	}
	_, writeErr := file.Write(data)
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return fmt.Errorf("write %s: %w", temp, err)
	}
	if err := p.root.Rename(temp, path); err != nil {
		return fmt.Errorf("rename %s: %w", temp, err)
	}

	return nil
}

func (p *project) backup(path string, original []byte, version int, now time.Time) error {
	name := strings.NewReplacer("/", "_", "\\", "_").Replace(path)
	target := filepath.Join(backupDir, name+".v"+strconv.Itoa(version)+"."+now.UTC().Format(stampLayout)+".bak")
	if err := p.root.WriteFile(target, original, filePerm); err != nil {
		return fmt.Errorf("back up %s before upgrading: %w", path, err)
	}
	p.logger.Info("backed up document before upgrading", "path", path, "backup", target, "fromVersion", version)

	return nil
}

// documentPath returns "<format>s/<name>--<short id>.json".
func documentPath(format, name string, id ids.ULID) string {
	return filepath.Join(format+"s", slug(name)+"--"+id.Short(shortIDChars)+".json")
}

func slug(name string) string {
	var builder strings.Builder
	dash := false
	for _, char := range strings.ToLower(name) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			builder.WriteRune(char)
			dash = false

			continue
		}
		if !dash && builder.Len() > 0 {
			builder.WriteByte('-')
			dash = true
		}
	}
	result := strings.TrimSuffix(builder.String(), "-")
	if result == "" {
		return "document"
	}

	return result
}

func noDocument() storedDocument {
	var none storedDocument

	return none
}
