package orchestrator

import (
	"encoding/json"
	"time"

	projectindex "github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// stamp records that a document's content changed now. The project index
// windows see says so once a day for each document, which is as fine as the
// Project window tells changes apart, so editing does not send the index
// along with every keystroke. Saving keeps the time exactly.
func (o *Orchestrator) stamp(entry *documentEntry) {
	now := o.cfg.Clock()
	entry.info.Changed = now
	index, built := o.liveIndex()
	if !built {
		return
	}
	listed, found := index.Entry(entry.info.ID)
	if !found || sameDay(listed.Changed, now) {
		return
	}
	args, err := json.Marshal(projectindex.TouchArgs{ID: entry.info.ID, Changed: now})
	if err != nil {
		return
	}
	if _, err := (projectindex.Touch{}).Apply(index, args); err != nil {
		o.cfg.Logger.Error("could not say when a document changed", "document", entry.info.ID, "error", err)

		return
	}
	held := o.docs[projectindex.IndexID()]
	held.info.Revision++
	o.broadcast(projectindex.IndexID(), protocol.Event{
		Document: projectindex.IndexID(), Revision: held.info.Revision, Edit: projectindex.TouchEdit, Args: args,
		Origin: ids.ULID{}, Request: 0,
	})
}

// sameDay reports whether two times fall on the same day, each where it was
// read from the clock.
func sameDay(first, second time.Time) bool {
	return first.Year() == second.Year() && first.YearDay() == second.YearDay()
}
