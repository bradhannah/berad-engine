package orchestrator

import (
	"crypto/sha256"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

// What the editor has seen of each file (increment 57): its size, when it
// was written and a hash of its bytes, as the editor read it or last wrote
// it, so a file changed outside the editor is never written over unseen.

// fileStamp is a file as the editor last saw it.
type fileStamp struct {
	size int64
	mod  time.Time
	sum  [sha256.Size]byte
}

// note records a file as the editor has just read or written it.
func (p *project) note(path string, data []byte) {
	info, err := p.root.Stat(path)
	if err != nil {
		delete(p.seen, path)

		return
	}
	p.seen[path] = fileStamp{size: info.Size(), mod: info.ModTime(), sum: sha256.Sum256(data)}
}

// forget forgets a file the editor no longer reads or writes.
func (p *project) forget(path string) { delete(p.seen, path) }

// outside reports whether a file was changed outside the editor since the
// editor last read or wrote it, and whether it is gone. A file the editor
// never saw counts as changed only once something is there. Only the bytes
// count: a file written again with the same bytes, as a checkout can, is
// not changed.
func (p *project) outside(path string) (bool, bool) {
	stamp, seen := p.seen[path]
	info, err := p.root.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return seen, seen
	}
	if err != nil {
		return false, false
	}
	if !seen {
		return true, false
	}
	if info.Size() == stamp.size && info.ModTime().Equal(stamp.mod) {
		return false, false
	}
	data, err := p.root.ReadFile(path)
	if err != nil {
		return true, false
	}
	if sha256.Sum256(data) != stamp.sum {
		return true, false
	}
	stamp.size, stamp.mod = info.Size(), info.ModTime()
	p.seen[path] = stamp

	return false, false
}

// keepCopy puts a copy of a version of a file in the project's backups,
// under a kind of reason, as "changed".
func (p *project) keepCopy(kind, path string, data []byte, now time.Time) (string, error) {
	folder := filepath.Join(backupDir, kind)
	if err := p.root.MkdirAll(folder, dirPerm); err != nil {
		return "", errors.Join(errBackup, err)
	}
	name := strings.NewReplacer("/", "_", "\\", "_").Replace(path)
	target := filepath.Join(folder, name+"."+now.UTC().Format(stampLayout)+".bak")
	if err := p.root.WriteFile(target, data, filePerm); err != nil {
		return "", errors.Join(errBackup, err)
	}

	return target, nil
}

// errBackup reports a copy that could not be put in the backups.
var errBackup = errors.New("could not keep a copy in the backups")
