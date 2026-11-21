package docsource_test

import (
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/views/docsource"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/gameload"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	"github.com/bradhannah/berad-engine/engine/zone"
)

var (
	townID  = frametest.ID(1)
	clerkID = frametest.ID(2)
	goneID  = frametest.ID(3)
)

// project is a window that holds a town and the project index, which also
// lists a clerk the window has not been sent.
func held() *frametest.Docs {
	docs := frametest.NewDocs()
	docs.Put(townID, "Town", zoneplugin.NewDocument())
	docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: townID, Name: "Town", Format: zone.FormatName}, {ID: clerkID, Name: "Ada Clerk", Format: npc.FormatName},
	}))

	return docs
}

// arriving is a window's documents where one arrives the moment after it
// is first looked for, as documents do while a window works.
type arriving struct {
	*frametest.Docs

	late    ids.ULID
	looked  bool
	arrives func()
}

func (a *arriving) View(documentID ids.ULID, visit func(client.Document)) bool {
	shown := a.Docs.View(documentID, visit)
	if documentID == a.late && !a.looked {
		a.looked = true
		a.arrives()
	}

	return shown
}

// TestSourceSaysWhatTheWindowHolds shows a document the window holds, waits
// for one the index lists that has not been sent, and has none the index
// does not list; it names and lists by the index.
func TestSourceSaysWhatTheWindowHolds(t *testing.T) {
	t.Parallel()
	source := docsource.New(held())
	for documentID, want := range map[ids.ULID]gameload.State{
		townID: gameload.Held, clerkID: gameload.Waiting, goneID: gameload.Missing,
	} {
		if got := source.View(documentID, func(gameload.Document) {}); got != want {
			t.Fatalf("%s: got %v, want %v", documentID, got, want)
		}
	}
	source.View(townID, func(document gameload.Document) {
		if _, isZone := document.Value.(*zone.Zone); !isZone || document.Name != "Town" {
			t.Fatalf("a zone, by its name in the index: %+v", document)
		}
	})
	if source.Name(clerkID) != "Ada Clerk" || source.Name(goneID) != "" || len(source.List(npc.FormatName)) != 1 {
		t.Fatal("names and lists come from the index")
	}
}

// TestAskingSubscribesAndSeesChanges subscribes to each document asked for
// that the project holds, and stamps what it saw: the stamp differs once a
// document asked for arrives or changes, and not before.
func TestAskingSubscribesAndSeesChanges(t *testing.T) {
	t.Parallel()
	docs := held()
	var subscribed []ids.ULID
	asking := docsource.New(docs).Asking(func(documentID ids.ULID) { subscribed = append(subscribed, documentID) })
	for _, documentID := range []ids.ULID{townID, clerkID, goneID} {
		asking.View(documentID, func(gameload.Document) {})
	}
	read, saw := asking.Seen()
	if len(subscribed) != 2 || len(read) != 2 {
		t.Fatalf("the town and the clerk are subscribed to and noted, not what the project lacks: %v %v", subscribed, read)
	}
	source := docsource.New(docs)
	if source.Stamp(read) != saw {
		t.Fatal("nothing has changed yet")
	}
	docs.Put(clerkID, "Ada Clerk", npcplugin.NewDocument())
	arrived := source.Stamp(read)
	if arrived == saw {
		t.Fatal("the clerk's arrival is seen")
	}
	docs.Touch(townID)
	if source.Stamp(read) == arrived {
		t.Fatal("an edit to the town is seen")
	}
}

// TestAskingSeesWhatArrivesWhileItGathers notes a document as it was when
// first asked for: one that arrives the moment after, while the gathering
// is still under way, is seen as changed and not missed.
func TestAskingSeesWhatArrivesWhileItGathers(t *testing.T) {
	t.Parallel()
	docs := held()
	late := &arriving{Docs: docs, late: clerkID, looked: false, arrives: func() {
		docs.Put(clerkID, "Ada Clerk", npcplugin.NewDocument())
	}}
	asking := docsource.New(late).Asking(func(ids.ULID) {})
	asking.View(clerkID, func(gameload.Document) {})
	asking.View(townID, func(gameload.Document) {})
	read, saw := asking.Seen()
	if docsource.New(late).Stamp(read) == saw {
		t.Fatal("the clerk arrived after the gathering looked for her, so the gathering must be done again")
	}
}
