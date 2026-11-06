package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"slices"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Home's memory is this person's alone: the documents opened lately and what
// they keep in the Project window. It is kept in the project's own state
// folder, which a .gitignore of its own keeps out of version control.
const (
	homePath      = ".berad/home.json"
	ignorePath    = ".berad/.gitignore"
	ignoreAll     = "*\n"
	homeVersion   = 1
	homeOpenedMax = 50
)

// homeFile is Home's memory as saved.
type homeFile struct {
	Version int                 `json:"version"`
	Opened  []protocol.HomeOpen `json:"opened"`
	Kept    protocol.HomeKept   `json:"kept"`
}

func newHomeFile() homeFile {
	return homeFile{
		Version: homeVersion, Opened: []protocol.HomeOpen{},
		Kept: protocol.HomeKept{Pins: []ids.ULID{}, Folders: map[string]protocol.FolderChoice{}},
	}
}

// ignoreState keeps the state folder out of version control, unless the
// project already says otherwise in that folder's own .gitignore.
func (p *project) ignoreState() error {
	if _, err := p.root.Stat(ignorePath); err == nil || !errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	return p.writeAtomic(ignorePath, []byte(ignoreAll))
}

// loadHome reads Home's memory and shares it with every window that joins.
// A missing file is an empty memory; one that cannot be read, or that a
// newer editor wrote, is set aside with a warning, as it is only this
// person's convenience.
func (o *Orchestrator) loadHome() {
	o.home = newHomeFile()
	data, err := o.project.root.ReadFile(homePath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		o.cfg.Logger.Warn("home memory unreadable", "error", err)
	default:
		var saved homeFile
		if err := json.Unmarshal(data, &saved); err != nil || saved.Version > homeVersion {
			o.cfg.Logger.Warn("home memory set aside", "version", saved.Version, "error", err)

			break
		}
		o.home = tidyHome(saved)
	}
	o.shareHome(protocol.TopicHomeOpened, protocol.HomeOpened{Opened: o.home.Opened})
	o.shareHome(protocol.TopicHomeKept, o.home.Kept)
}

// tidyHome fills what an older or hand-edited file left out.
func tidyHome(saved homeFile) homeFile {
	home := newHomeFile()
	if saved.Opened != nil {
		home.Opened = saved.Opened
	}
	if saved.Kept.Pins != nil {
		home.Kept.Pins = saved.Kept.Pins
	}
	if saved.Kept.Folders != nil {
		home.Kept.Folders = saved.Kept.Folders
	}

	return home
}

// noteOpened puts a document at the top of what was opened lately, saves
// Home's memory and tells every window.
func (o *Orchestrator) noteOpened(document ids.ULID) {
	if document.IsZero() {
		return
	}
	opened := slices.DeleteFunc(slices.Clone(o.home.Opened), func(open protocol.HomeOpen) bool {
		return open.Document == document
	})
	opened = append([]protocol.HomeOpen{{Document: document, At: o.cfg.Clock().UTC()}}, opened...)
	o.home.Opened = opened[:min(len(opened), homeOpenedMax)]
	o.saveHome()
	o.shareHome(protocol.TopicHomeOpened, protocol.HomeOpened{Opened: o.home.Opened})
}

// keepHome saves what the Project window keeps, as it publishes it.
func (o *Orchestrator) keepHome(value json.RawMessage) error {
	var kept protocol.HomeKept
	if err := json.Unmarshal(value, &kept); err != nil {
		return fmt.Errorf("home: %w", err)
	}
	o.home = tidyHome(homeFile{Version: homeVersion, Opened: o.home.Opened, Kept: kept})
	o.saveHome()

	return nil
}

func (o *Orchestrator) saveHome() {
	data, err := json.MarshalIndent(o.home, "", "  ")
	if err == nil {
		err = o.project.writeAtomic(homePath, data)
	}
	if err != nil {
		o.cfg.Logger.Warn("home memory not saved", "error", err)
	}
}

// shareHome sets a Home topic for windows that join and sends it to every
// window already joined.
func (o *Orchestrator) shareHome(topic string, value any) {
	encoded, err := json.Marshal(value)
	if err != nil {
		o.cfg.Logger.Error("encode home", "topic", topic, "error", err)

		return
	}
	o.session[topic] = encoded
	for _, joined := range o.windows {
		o.send(joined, mustMessage(protocol.KindSession, 0, protocol.Session{Topic: topic, Value: encoded}))
	}
}
