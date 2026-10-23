// Package orchestrator is the editor's headless core: the only writer of
// project data. Window processes connect over a Unix socket, send commands and
// receive events; they never touch project files.
//
// All project state is owned by one goroutine (Run), so it needs no locks.
// Connection goroutines only frame messages and pass them to that loop.
package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bradhannah/berad-engine/editor/model"
	projectindex "github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
)

const socketPerm = 0o600

// ErrRunning reports a call that is only allowed before Run.
var ErrRunning = errors.New("orchestrator is already running")

// ErrUnknownDocument reports a document ID the project does not hold.
var ErrUnknownDocument = errors.New("unknown document")

// ErrUnknownEdit reports an edit kind no plug-in registered.
var ErrUnknownEdit = errors.New("unknown edit")

// ErrNotSaved reports an edit undone because it could not be journaled.
var ErrNotSaved = errors.New("edit could not be saved and was undone")

// ErrJournalGap reports a journal that skips document revisions.
var ErrJournalGap = errors.New("journal skips a document revision")

// DocumentInfo describes a document in the project.
type DocumentInfo struct {
	ID       ids.ULID
	Name     string
	Format   string
	Revision uint64
	Path     string
}

type documentEntry struct {
	info     DocumentInfo
	version  int
	document model.Document
	dirty    bool
}

type window struct {
	id    ids.ULID
	hello protocol.Hello
	group string
	conn  *protocol.Conn
	out   chan protocol.Message
}

type inbound struct {
	from    *window
	message protocol.Message
}

// Orchestrator owns one open project.
type Orchestrator struct {
	cfg      Config
	project  *project
	journal  *journal
	lock     *projectLock
	listener net.Listener
	ids      *ids.Generator

	docs       map[ids.ULID]*documentEntry
	history    *history
	session    map[string]json.RawMessage
	windows    map[ids.ULID]*window
	subscribed map[ids.ULID]map[ids.ULID]*window
	recent     map[ids.ULID]*ring
	children   map[ids.ULID]*child
	picks      *picks
	preview    *preview
	// removed holds files of deleted or renamed documents, moved to the
	// backups at the next save.
	removed  map[string]bool
	seq      uint64
	stopping bool
	quit     bool

	inbound chan inbound
	joins   chan *window
	leaves  chan *window
	exits   chan childExit
	stopped chan struct{}
	running atomic.Bool
}

// New opens a project: it takes the project lock, loads and upgrades its
// documents, replays the journal and starts listening for windows.
func New(ctx context.Context, cfg Config) (*Orchestrator, error) {
	cfg, err := cfg.withDefaults()
	if err != nil {
		return nil, err
	}
	if _, ok := cfg.Registry.Edit(projectindex.CreateEdit); !ok {
		return nil, fmt.Errorf("%w: %w", ErrConfig, ErrNoProjectPlugin)
	}
	opened, err := openProject(cfg.ProjectDir, cfg.Logger)
	if err != nil {
		return nil, err
	}
	orch := newOrchestrator(cfg, opened)
	if err := orch.open(ctx); err != nil {
		return nil, errors.Join(err, orch.closeResources())
	}

	return orch, nil
}

func newOrchestrator(cfg Config, opened *project) *Orchestrator {
	return &Orchestrator{
		cfg:        cfg,
		project:    opened,
		journal:    nil,
		lock:       nil,
		listener:   nil,
		ids:        ids.NewGenerator(cfg.Clock, cfg.Random),
		docs:       map[ids.ULID]*documentEntry{},
		history:    newHistory(cfg.UndoLimit),
		session:    map[string]json.RawMessage{},
		windows:    map[ids.ULID]*window{},
		subscribed: map[ids.ULID]map[ids.ULID]*window{},
		recent:     map[ids.ULID]*ring{},
		children:   map[ids.ULID]*child{},
		picks:      newPicks(),
		preview:    nil,
		removed:    map[string]bool{},
		seq:        0,
		stopping:   false,
		quit:       false,
		inbound:    make(chan inbound),
		joins:      make(chan *window),
		leaves:     make(chan *window),
		exits:      make(chan childExit),
		stopped:    make(chan struct{}),
		running:    atomic.Bool{},
	}
}

func (o *Orchestrator) open(ctx context.Context) error {
	lock, err := acquireLock(o.project.root, lockInfo{Socket: o.cfg.SocketPath, PID: os.Getpid()})
	if err != nil {
		return err
	}
	o.lock = lock
	o.journal, err = openJournal(o.project.root)
	if err != nil {
		return err
	}
	if err := o.load(); err != nil {
		return err
	}

	return o.listen(ctx)
}

func (o *Orchestrator) load() error {
	stored, err := o.project.loadDocuments(o.cfg.Registry.Formats(), o.cfg.Clock())
	if err != nil {
		return err
	}
	for _, document := range stored {
		documentType, ok := o.cfg.Registry.Document(document.envelope.Format)
		if !ok {
			continue
		}
		var layout json.RawMessage
		if document.layout != nil {
			layout = document.layout.envelope.Data
		}
		decoded, err := documentType.DecodeWithLayout(document.envelope.Data, layout)
		if err != nil {
			return fmt.Errorf("decode %s: %w", document.path, err)
		}
		o.docs[document.envelope.ID] = &documentEntry{
			info: DocumentInfo{
				ID:       document.envelope.ID,
				Name:     document.envelope.Name,
				Format:   document.envelope.Format,
				Revision: document.envelope.Revision,
				Path:     document.path,
			},
			version:  document.envelope.Version,
			document: decoded,
			dirty:    document.upgraded(),
		}
	}

	if err := o.replay(); err != nil {
		return err
	}
	o.buildIndex()

	return nil
}

// replay applies journal entries newer than the saved documents.
func (o *Orchestrator) replay() error {
	entries, err := o.journal.entries()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		o.seq = max(o.seq, entry.Seq)
		if isIndex(entry.Document) {
			if err := o.replayLifecycle(entry); err != nil {
				return err
			}

			continue
		}
		target, found := o.docs[entry.Document]
		if !found {
			o.cfg.Logger.Warn("journal entry for a missing document", "document", entry.Document)

			continue
		}
		if entry.Revision <= target.info.Revision {
			continue
		}
		if entry.Revision != target.info.Revision+1 {
			return fmt.Errorf("%w: %s at %d, entry %d", ErrJournalGap, entry.Document, target.info.Revision, entry.Revision)
		}
		kind, ok := o.cfg.Registry.Edit(entry.Edit)
		if !ok {
			return fmt.Errorf("replay: %w: %s", ErrUnknownEdit, entry.Edit)
		}
		if _, err := kind.Apply(target.document, entry.Args); err != nil {
			return fmt.Errorf("replay %s on %s: %w", entry.Edit, entry.Document, err)
		}
		target.info.Revision = entry.Revision
		target.dirty = true
	}
	if len(entries) > 0 {
		o.cfg.Logger.Info("recovered edits from the journal", "entries", len(entries))
	}

	return nil
}

func (o *Orchestrator) listen(ctx context.Context) error {
	if err := os.Remove(o.cfg.SocketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale socket: %w", err)
	}
	var config net.ListenConfig
	listener, err := config.Listen(ctx, "unix", o.cfg.SocketPath)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", o.cfg.SocketPath, err)
	}
	o.listener = listener
	if err := os.Chmod(o.cfg.SocketPath, socketPerm); err != nil {
		return fmt.Errorf("restrict socket: %w", err)
	}

	return nil
}

// SocketPath returns where windows connect.
func (o *Orchestrator) SocketPath() string {
	return o.cfg.SocketPath
}

// Documents lists the project's documents, sorted by name. Call before Run.
func (o *Orchestrator) Documents() ([]DocumentInfo, error) {
	if o.running.Load() {
		return nil, ErrRunning
	}
	infos := make([]DocumentInfo, 0, len(o.docs))
	for id, entry := range o.docs {
		if !isIndex(id) {
			infos = append(infos, entry.info)
		}
	}
	slices.SortFunc(infos, func(a, b DocumentInfo) int { return strings.Compare(a.Name, b.Name) })

	return infos, nil
}

// CreateDocument adds a new document and saves it immediately. Call before Run.
func (o *Orchestrator) CreateDocument(name string, document model.Document) (ids.ULID, error) {
	if o.running.Load() {
		return ids.ULID{}, ErrRunning
	}
	documentType, ok := o.cfg.Registry.Document(document.Format())
	if !ok {
		return ids.ULID{}, fmt.Errorf("%w: %s", content.ErrUnknownFormat, document.Format())
	}
	documentID, err := o.ids.New()
	if err != nil {
		return ids.ULID{}, fmt.Errorf("document id: %w", err)
	}
	path := documentPath(document.Format(), name, documentID)
	entry := &documentEntry{
		info:     DocumentInfo{ID: documentID, Name: name, Format: document.Format(), Revision: 0, Path: path},
		version:  documentType.Format.Current,
		document: document,
		dirty:    true,
	}
	if err := o.save(entry); err != nil {
		return ids.ULID{}, err
	}
	o.docs[documentID] = entry
	o.buildIndex()

	return documentID, nil
}

// NewID returns a new permanent ID from the orchestrator's generator (D4),
// for content created before Run such as a sample project's anchors.
func (o *Orchestrator) NewID() (ids.ULID, error) {
	id, err := o.ids.New()
	if err != nil {
		return ids.ULID{}, fmt.Errorf("new id: %w", err)
	}

	return id, nil
}

// Close releases an orchestrator that was never run, saving any changes. An
// orchestrator that runs releases everything itself when Run returns.
func (o *Orchestrator) Close() error {
	if !o.running.CompareAndSwap(false, true) {
		return ErrRunning
	}
	close(o.stopped)

	return errors.Join(o.flush(), o.closeResources())
}

// Run serves windows until ctx is cancelled, then shuts down: windows are told
// to close, documents are saved and the project lock is released.
func (o *Orchestrator) Run(ctx context.Context) error {
	if !o.running.CompareAndSwap(false, true) {
		return ErrRunning
	}
	go o.acceptLoop(ctx)
	flush := time.NewTicker(o.cfg.FlushInterval)
	defer flush.Stop()
	sync := time.NewTicker(o.cfg.SyncInterval)
	defer sync.Stop()
	o.flushLogged()
	for {
		select {
		case <-ctx.Done():
			return o.shutdown()
		case joined := <-o.joins:
			o.join(joined)
		case left := <-o.leaves:
			o.leave(left)
		case message := <-o.inbound:
			o.handle(message)
		case exit := <-o.exits:
			o.childExited(exit)
			if o.quit {
				return o.shutdown()
			}
		case <-flush.C:
			o.flushLogged()
		case <-sync.C:
			if err := o.journal.sync(); err != nil {
				o.cfg.Logger.Error("journal sync failed", "error", err)
			}
		}
	}
}

func (o *Orchestrator) shutdown() error {
	o.stopping = true
	for _, connected := range o.windows {
		o.send(connected, mustMessage(protocol.KindShutdown, 0, protocol.Shutdown{Reason: "the editor is closing"}))
	}
	for _, connected := range slices.Collect(maps.Values(o.windows)) {
		o.leave(connected)
	}
	o.waitForChildren(o.cfg.Clock().Add(o.cfg.ShutdownTimeout))
	close(o.stopped)
	flushErr := o.flush()

	return errors.Join(flushErr, o.closeResources())
}

func (o *Orchestrator) closeResources() error {
	var errs []error
	if o.listener != nil {
		if err := o.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			errs = append(errs, fmt.Errorf("close listener: %w", err))
		}
		if err := os.Remove(o.cfg.SocketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("remove socket: %w", err))
		}
	}
	if o.journal != nil {
		errs = append(errs, o.journal.close())
	}
	if o.lock != nil {
		errs = append(errs, o.lock.release())
	}
	errs = append(errs, o.project.close())

	return errors.Join(errs...)
}

// flush saves every changed document to its file, moves the files of deleted
// and renamed documents to the backups, then empties the journal. New files
// are written before old ones move, so a crash in between leaves two copies,
// which loading resolves, rather than none.
func (o *Orchestrator) flush() error {
	var errs []error
	live := map[string]bool{}
	for _, entry := range o.docs {
		live[entry.info.Path] = true
		live[layoutPath(entry.info.Path)] = true
		if !entry.dirty || entry.info.Path == "" {
			continue
		}
		if err := o.save(entry); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	for _, path := range slices.Sorted(maps.Keys(o.removed)) {
		if !live[path] {
			if err := o.project.retire(path, o.cfg.Clock()); err != nil {
				errs = append(errs, err)

				continue
			}
		}
		delete(o.removed, path)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return o.journal.reset()
}

func (o *Orchestrator) flushLogged() {
	if err := o.flush(); err != nil {
		o.cfg.Logger.Error("saving documents failed; edits remain in the journal", "error", err)
	}
}

func (o *Orchestrator) save(entry *documentEntry) error {
	data, err := entry.document.Encode()
	if err != nil {
		return fmt.Errorf("encode %s: %w", entry.info.Name, err)
	}
	envelope := content.Envelope{
		Format:   entry.info.Format,
		Version:  entry.version,
		ID:       entry.info.ID,
		Name:     entry.info.Name,
		Revision: entry.info.Revision,
		Data:     data,
	}
	if err := o.project.writeDocument(entry.info.Path, envelope); err != nil {
		return err
	}
	if err := o.saveLayout(entry); err != nil {
		return err
	}
	entry.dirty = false

	return nil
}

// saveLayout writes a layered document's layout file beside it (D17). Its
// envelope carries no revision, so the file changes only when the layout
// does.
func (o *Orchestrator) saveLayout(entry *documentEntry) error {
	layout, err := model.LayoutOf(entry.document)
	if err != nil {
		return fmt.Errorf("save %s: %w", entry.info.Name, err)
	}
	if layout == nil {
		return nil
	}
	documentType, ok := o.cfg.Registry.Document(entry.info.Format)
	if !ok || documentType.Layout == nil {
		return fmt.Errorf("%w: %s has a layout but no layout format", content.ErrUnknownFormat, entry.info.Format)
	}
	envelope := content.Envelope{
		Format: documentType.Layout.Name, Version: documentType.Layout.Current, ID: entry.info.ID,
		Name: entry.info.Name, Revision: 0, Data: layout,
	}

	return o.project.writeDocument(layoutPath(entry.info.Path), envelope)
}

// mustMessage builds a message from a payload type defined in this program;
// encoding such a payload cannot fail.
func mustMessage(kind protocol.Kind, request uint64, payload any) protocol.Message {
	message, err := protocol.NewMessage(kind, request, payload)
	if err != nil {
		panic(err)
	}

	return message
}
