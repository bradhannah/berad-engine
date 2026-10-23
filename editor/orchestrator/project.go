package orchestrator

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
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
	removedDir   = ".berad/backups/removed"
	logDir       = ".berad/logs"
	lockPath     = ".berad/orchestrator.lock"
	dirPerm      = 0o750
	filePerm     = 0o600
	shortIDChars = 8
	tempSuffix   = ".tmp"
	stampLayout  = "20060102T150405"
)

// project reads and writes a project folder through an os.Root, so no path can
// escape it.
type project struct {
	root   *os.Root
	logger *slog.Logger
}

// storedDocument is a document read from disk, already upgraded if needed,
// with the layout file beside it when it has one (D17).
type storedDocument struct {
	path        string
	envelope    content.Envelope
	fromVersion int
	layout      *storedDocument
}

// upgraded reports whether the document or its layout was upgraded on load,
// so it must be saved in the current format.
func (d storedDocument) upgraded() bool {
	return d.fromVersion != d.envelope.Version || (d.layout != nil && d.layout.upgraded())
}

// layoutSuffix ends the name of a document's layout file, which sits beside
// the document's own file.
const layoutSuffix = ".layout.json"

// layoutPath returns where a document's layout file lives.
func layoutPath(path string) string {
	return strings.TrimSuffix(path, ".json") + layoutSuffix
}

func openProject(dir string, logger *slog.Logger) (*project, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open project %s: %w", dir, err)
	}
	for _, sub := range []string{stateDir, backupDir, removedDir, logDir} {
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
// in memory; the caller saves the upgraded form. Two files holding the same
// document (left by a crash during a rename) resolve to the newer one.
func (p *project) loadDocuments(formats *content.Registry, now time.Time) ([]storedDocument, error) {
	var documents []storedDocument
	layouts := map[string]storedDocument{}
	seen := map[ids.ULID]int{}
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
		if strings.HasSuffix(path, layoutSuffix) {
			layouts[path] = document

			return nil
		}
		if index, duplicate := seen[document.envelope.ID]; duplicate {
			kept, retired := p.newer(documents[index], document)
			documents[index] = kept
			p.logger.Warn("two files hold one document; keeping the newer", "kept", kept.path, "moved", retired.path)

			return errors.Join(p.retire(retired.path, now), p.retire(layoutPath(retired.path), now))
		}
		seen[document.envelope.ID] = len(documents)
		documents = append(documents, document)

		return nil
	}
	if err := fs.WalkDir(p.root.FS(), ".", walk); err != nil {
		return nil, fmt.Errorf("load documents: %w", err)
	}

	return documents, p.attachLayouts(documents, layouts, now)
}

// attachLayouts gives each document the layout file beside it. A layout file
// whose document is gone moves to the backups.
func (p *project) attachLayouts(documents []storedDocument, layouts map[string]storedDocument, now time.Time) error {
	for index := range documents {
		path := layoutPath(documents[index].path)
		if layout, ok := layouts[path]; ok && layout.envelope.ID == documents[index].envelope.ID {
			documents[index].layout = &layout
			delete(layouts, path)
		}
	}
	errs := make([]error, 0, len(layouts))
	for _, path := range slices.Sorted(maps.Keys(layouts)) {
		p.logger.Warn("a layout file has no document; moving it to the backups", "path", path)
		errs = append(errs, p.retire(path, now))
	}

	return errors.Join(errs...)
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

	return storedDocument{path: path, envelope: upgraded, fromVersion: envelope.Version, layout: nil}, true, nil
}

// newer returns the copy of a document to keep and the one to retire: the
// higher revision, or the more recently written file.
func (p *project) newer(first, second storedDocument) (storedDocument, storedDocument) {
	if first.envelope.Revision != second.envelope.Revision {
		if first.envelope.Revision > second.envelope.Revision {
			return first, second
		}

		return second, first
	}
	firstInfo, firstErr := p.root.Stat(first.path)
	secondInfo, secondErr := p.root.Stat(second.path)
	if firstErr == nil && secondErr == nil && secondInfo.ModTime().After(firstInfo.ModTime()) {
		return second, first
	}

	return first, second
}

// retire moves a file that no longer holds a live document into the backups,
// rather than deleting it.
func (p *project) retire(path string, now time.Time) error {
	name := strings.NewReplacer("/", "_", "\\", "_").Replace(path)
	target := filepath.Join(removedDir, name+"."+now.UTC().Format(stampLayout)+".bak")
	if err := p.root.Rename(path, target); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("move %s to the backups: %w", path, err)
	}

	return nil
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
