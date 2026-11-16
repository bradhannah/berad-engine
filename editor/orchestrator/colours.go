package orchestrator

import (
	"encoding/json"
	"errors"
	"io/fs"

	"github.com/bradhannah/berad-engine/editor/protocol"
)

// The colour this person last painted with in each sprite (increment 60) is
// kept beside the window's arrangement, in the project's own state folder,
// never in the project itself.
const (
	coloursPath    = ".berad/colours.json"
	coloursVersion = 1
)

// coloursFile is the colours as saved.
type coloursFile struct {
	Version int                   `json:"version"`
	Colours protocol.PixelColours `json:"colours"`
}

// loadColours reads the colours kept and shares them with every window that
// joins. A file that cannot be read is set aside with a warning, as it is
// only this person's convenience.
func (o *Orchestrator) loadColours() {
	colours := protocol.PixelColours{Sprites: map[string]json.RawMessage{}}
	data, err := o.project.root.ReadFile(coloursPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		o.cfg.Logger.Warn("colours unreadable", "error", err)
	default:
		var saved coloursFile
		if err := json.Unmarshal(data, &saved); err != nil || saved.Version > coloursVersion {
			o.cfg.Logger.Warn("colours set aside", "version", saved.Version, "error", err)

			break
		}
		if saved.Colours.Sprites != nil {
			colours = saved.Colours
		}
	}
	o.shareHome(protocol.TopicPixelColours, colours)
}

// keepColours saves the colours as the Pixel editor publishes them.
func (o *Orchestrator) keepColours(value json.RawMessage) {
	var colours protocol.PixelColours
	if err := json.Unmarshal(value, &colours); err != nil {
		o.cfg.Logger.Warn("colours not kept", "error", err)

		return
	}
	data, err := json.MarshalIndent(coloursFile{Version: coloursVersion, Colours: colours}, "", "  ")
	if err == nil {
		err = o.project.writeAtomic(coloursPath, data)
	}
	if err != nil {
		o.cfg.Logger.Warn("colours not saved", "error", err)
	}
}
