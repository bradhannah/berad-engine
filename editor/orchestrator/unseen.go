package orchestrator

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Nothing used goes unseen (increment 58): a part and what holds it, such
// as a floor and its zone, are deleted only together; the windows that hold
// unsaved changes to what is deleted can be named before it goes; and a tab
// closed because its document went says why.

// ErrOutOfStep reports a part deleted while what holds it still lists it,
// or what holds parts deleted while it still lists them.
var ErrOutOfStep = errors.New("a part and what holds it go together")

// refuseOutOfStep refuses deleting a document a container still lists, or
// a container that still lists parts: each part is taken out and deleted
// first, in the same step, as the Project window does for a zone.
func (o *Orchestrator) refuseOutOfStep(documentID ids.ULID, entry *documentEntry) error {
	if containerID := o.containerOf(documentID); !containerID.IsZero() {
		holder := o.docs[containerID].info.Name

		return fmt.Errorf("%w: %s is part of %s; take it out of %s first",
			ErrOutOfStep, entry.info.Name, holder, holder)
	}
	container, ok := entry.document.(model.Container)
	if !ok {
		return nil
	}
	var held []string
	for _, part := range container.Parts() {
		if listed, exists := o.docs[part]; exists {
			held = append(held, listed.info.Name)
		}
	}
	if len(held) == 0 {
		return nil
	}

	return fmt.Errorf("%w: %s still holds %s; take each out and delete it first",
		ErrOutOfStep, entry.info.Name, namesSaid(held))
}

// namesSaid is "Ground floor", "Ground floor and Cellar", or "Ground floor,
// Cellar and Upstairs".
func namesSaid(names []string) string {
	last := len(names) - 1
	if last <= 0 {
		return strings.Join(names, "")
	}

	return strings.Join(names[:last], ", ") + " and " + names[last]
}

// openUnsaved lists the windows other than the one asking that show a
// document and hold unsaved changes to it.
func (o *Orchestrator) openUnsaved(asking *window, documentID ids.ULID) []protocol.Use {
	uses := []protocol.Use{}
	for _, open := range o.windows {
		if open.id == asking.id || open.hello.Target != documentID || open.unsaved.Count == 0 {
			continue
		}
		name := open.unsaved.Name
		if name == "" {
			name = o.docs[documentID].info.Name
		}
		uses = append(uses, protocol.Use{
			Document: documentID, Name: name, Format: open.hello.View,
			Label: changesSaid(open.unsaved.Count), Item: "",
		})
	}

	return uses
}

// tellClosed leaves a notice for a tab closed because its document was
// deleted.
func (o *Orchestrator) tellClosed(closed *window, name string) {
	o.addNotice(protocol.Notice{
		Kind: protocol.NoticeTabClosed, Window: closed.id, Report: "", Key: "closed:" + closed.id.String(),
		Text: "Closed the tab of “" + name + "”: it was deleted. Undo in the tab that deleted it brings it back" +
			unsavedSaid(closed.unsaved.Count) + ".",
		Actions: []protocol.NoticeAction{dismissButton()}, More: 0,
	})
}

// unsavedSaid says what of a closed tab's unsaved changes went with it.
func unsavedSaid(count int) string {
	switch count {
	case 0:
		return ""
	case 1:
		return ", without the 1 change that waited for Save"
	default:
		return ", without the " + strconv.Itoa(count) + " changes that waited for Save"
	}
}
