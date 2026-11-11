package window

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// TestAFailedSaveStaysInTheStatusBar shows a failed save's notice for as
// long as nothing replaces it, past the few seconds a status lasts, and ends
// it when a save succeeds, saying so.
func TestAFailedSaveStaysInTheStatusBar(t *testing.T) {
	t.Parallel()
	now := time.Now()
	h := &host{frame: frame.NewFrame(func() time.Time { return now })}
	tell := func(kind, text string) {
		value, err := json.Marshal(protocol.Notice{Kind: kind, Text: text})
		if err != nil {
			t.Fatal(err)
		}
		h.onUpdate(client.Update{Session: &protocol.Session{Topic: protocol.TopicNotice, Value: value}})
	}
	tell(protocol.NoticeSaveFailed, "Changes are not reaching the project's files")
	now = now.Add(time.Hour)
	if message, lasting := h.frame.Message(); !lasting || !strings.Contains(message, "not reaching") {
		t.Fatalf("an hour on, the status bar says %q", message)
	}
	tell(protocol.NoticeSaved, "Saved again")
	if message, lasting := h.frame.Message(); lasting || message != "Saved again" {
		t.Fatalf("once saved, the status bar says %q", message)
	}
	now = now.Add(time.Hour)
	if message, lasting := h.frame.Message(); lasting || strings.Contains(message, "not reaching") {
		t.Fatalf("the notice is over: %q", message)
	}
}

// TestARefusedPasteSaysWhy pastes more than a field holds: the field takes
// none of it, and the status bar says why.
func TestARefusedPasteSaysWhy(t *testing.T) {
	t.Parallel()
	h := &host{frame: frame.NewFrame(time.Now)}
	field := ui.NewTextField("name", textedit.NewField("", false, 3), false, nil)
	field.Field.Focus()
	h.tellRefusal(nil)
	field.Field.Type("abcd")
	h.tellRefusal(field)
	if message, _ := h.frame.Message(); field.Field.Value() != "" || !strings.Contains(message, "holds 3 characters") {
		t.Fatalf("the field holds %q and the status bar says %q", field.Field.Value(), message)
	}
}
