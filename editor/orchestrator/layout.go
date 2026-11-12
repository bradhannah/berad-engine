package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

	"github.com/bradhannah/berad-engine/editor/protocol"
)

// How this person arranges the editor window (increment 53) is kept beside
// Home's memory, in the project's own state folder, which a .gitignore of
// its own keeps out of version control: never in the project itself.
const (
	arrangementPath = ".berad/layout.json"
	layoutVersion   = 1
)

// layoutFile is the window's arrangement as saved.
type layoutFile struct {
	Version int             `json:"version"`
	Layout  protocol.Layout `json:"layout"`
}

// loadLayout reads the window's arrangement and shares it with every window
// that joins. A missing file is no arrangement yet; one that cannot be read
// is set aside with a warning, as it is only this person's convenience.
func (o *Orchestrator) loadLayout() {
	layout := protocol.Layout{Screens: map[string]protocol.Arrangement{}}
	data, err := o.project.root.ReadFile(arrangementPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		o.cfg.Logger.Warn("layout unreadable", "error", err)
	default:
		var saved layoutFile
		if err := json.Unmarshal(data, &saved); err != nil || saved.Version > layoutVersion {
			o.cfg.Logger.Warn("layout set aside", "version", saved.Version, "error", err)

			break
		}
		if saved.Layout.Screens != nil {
			layout = saved.Layout
		}
	}
	o.shareHome(protocol.TopicLayout, layout)
}

// keepLayout saves the window's arrangement as the window publishes it.
func (o *Orchestrator) keepLayout(value json.RawMessage) error {
	var layout protocol.Layout
	if err := json.Unmarshal(value, &layout); err != nil {
		return fmt.Errorf("layout: %w", err)
	}
	if err := o.project.ignoreState(); err != nil {
		o.cfg.Logger.Warn("state folder not ignored", "error", err)
	}
	data, err := json.MarshalIndent(layoutFile{Version: layoutVersion, Layout: layout}, "", "  ")
	if err == nil {
		err = o.project.writeAtomic(arrangementPath, data)
	}
	if err != nil {
		o.cfg.Logger.Warn("layout not saved", "error", err)
	}

	return nil
}
