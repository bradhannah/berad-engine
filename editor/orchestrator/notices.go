package orchestrator

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Lasting notices (increment 57): several can last at once, each under its
// own key, so a crashed tab, a failed save and a file changed on disk no
// longer replace one another. Windows show the newest with how many others
// last, and keep each in History as it arrives. A notice ends when what it
// tells of is over, or when its author answers it.

// addNotice makes a notice last, in place of one with the same key, and
// shows it as the newest.
func (o *Orchestrator) addNotice(notice protocol.Notice) {
	if notice.Key == "" {
		notice.Key = notice.Kind
	}
	o.notices = slices.DeleteFunc(o.notices, func(kept protocol.Notice) bool { return kept.Key == notice.Key })
	o.notices = append(o.notices, notice)
	o.publishNewest()
}

// endNotice ends the notice with a key, and shows the newest still lasting,
// or none.
func (o *Orchestrator) endNotice(key string) {
	before := len(o.notices)
	o.notices = slices.DeleteFunc(o.notices, func(kept protocol.Notice) bool { return kept.Key == key })
	if len(o.notices) != before {
		o.publishNewest()
	}
}

// lasting reports whether the notice with a key lasts.
func (o *Orchestrator) lasting(key string) bool {
	return slices.ContainsFunc(o.notices, func(kept protocol.Notice) bool { return kept.Key == key })
}

// publishNewest shows every window the newest lasting notice, with how many
// others last beside it, or that none lasts.
func (o *Orchestrator) publishNewest() {
	if len(o.notices) == 0 {
		o.publishNotice(noNotice())
		delete(o.session, protocol.TopicNotice)

		return
	}
	newest := o.notices[len(o.notices)-1]
	newest.More = len(o.notices) - 1
	o.publishNotice(newest)
}

// tellWindows shows every window open now a passing notice, such as a save
// that succeeded, without keeping it for windows that open later.
func (o *Orchestrator) tellWindows(notice protocol.Notice) {
	update := protocol.Session{Topic: protocol.TopicNotice, Value: mustJSON(notice)}
	for _, other := range o.windows {
		o.send(other, mustMessage(protocol.KindSession, 0, update))
	}
}

// noticeChoice carries out the button pressed on a notice, or the answer to
// a project that cannot open as it is.
func (o *Orchestrator) noticeChoice(message protocol.Message) {
	choice, err := protocol.DecodePayload[protocol.NoticeChoice](message)
	if err != nil {
		return
	}
	switch {
	case choice.Key == protocol.OpeningKey && choice.Action == protocol.OpenAnyway:
		o.openAnyway()
	case strings.HasPrefix(choice.Key, changedKeyPrefix):
		o.settleChanged(strings.TrimPrefix(choice.Key, changedKeyPrefix), choice.Action)
	case choice.Action == dismissAction:
		o.endNotice(choice.Key)
	}
}

// dismissAction is the button that ends a notice telling of something
// already done, such as fields a save will drop.
const dismissAction = "dismiss"

// dismissButton is a notice's Dismiss.
func dismissButton() protocol.NoticeAction {
	return protocol.NoticeAction{ID: dismissAction, Label: "Dismiss"}
}

// noNotice is the notice of no kind, which ends the one windows show.
func noNotice() protocol.Notice {
	return protocol.Notice{Kind: "", Text: "", Window: ids.ULID{}, Report: "", Key: "", Actions: nil, More: 0}
}

// mustJSON encodes a value of a type defined in this program, which cannot
// fail.
func mustJSON(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}

	return encoded
}
