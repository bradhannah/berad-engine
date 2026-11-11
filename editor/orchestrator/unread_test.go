package orchestrator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/protocol"
)

// told reports whether a window that has joined was sent a notice: what a
// window is told on joining reaches it before its first answer.
func told(c *client.Client) bool {
	for {
		select {
		case update := <-c.Updates():
			if update.Session != nil && update.Session.Topic == protocol.TopicNotice {
				return true
			}
		default:
			return false
		}
	}
}

// A file that cannot be read as a document is left out of the project, and
// every window is told which file and why: here one with a merge's conflict
// markers left in it, and one that is JSON but no document. A project whose
// files all read says nothing.
func TestAFileThatCannotBeReadIsNamed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	quiet := start(t, dir, nil)
	if told(quiet.dial(t, "")) {
		t.Fatal("a project whose files all read has nothing to tell")
	}
	if err := quiet.stop(); err != nil {
		t.Fatal(err)
	}

	conflicted := "<<<<<<< HEAD\n{\"format\": \"note\"}\n=======\n{\"format\": \"note\"}\n>>>>>>> theirs\n"
	if err := os.MkdirAll(filepath.Join(dir, "notes"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes", "plan--01j00000.json"), []byte(conflicted), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes", "list.json"), []byte(`["not", "a", "document"]`), 0o600); err != nil {
		t.Fatal(err)
	}
	h := start(t, dir, nil)
	notice := nextNotice(t, h.dial(t, ""))
	if notice.Kind != protocol.NoticeFilesLeftOut {
		t.Fatalf("the notice is about files left out: %+v", notice)
	}
	for _, want := range []string{
		filepath.Join("notes", "plan--01j00000.json"), "invalid character '<'", filepath.Join("notes", "list.json"),
	} {
		if !strings.Contains(notice.Text, want) {
			t.Fatalf("the notice names each file and why, here %q: %s", want, notice.Text)
		}
	}
	if err := h.stop(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "notes", "plan--01j00000.json")); err != nil {
		t.Fatalf("the file is left where it was, to be fixed: %v", err)
	}
}
