package workflowview

import (
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/places"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// runners is who runs the workflow, as last asked: the NPCs whose behaviour
// it is and those whose behaviour calls it, each with how, and what was
// asked from.
type runners struct {
	uses   []protocol.Use
	key    string
	asking bool
	wait   int
	// known is true once the orchestrator has answered.
	known bool
}

// runner is an NPC or a zone that runs the workflow, and the zone it lives
// in, a zone's being itself, zero for none or not loaded yet; home is true
// for the NPCs whose behaviour it is and the zones whose loop it is.
type runner struct {
	npc  ids.ULID
	name string
	zone ids.ULID
	home bool
}

// askRunners asks which NPCs run the workflow when it or the project has
// changed, and now and then besides, as nothing tells it when an NPC's
// behaviour or another workflow's Calls change; it keeps those NPCs
// loaded, to know where they live.
func (s *State) askRunners(ctx *frame.Context) {
	key := strconv.FormatUint(s.laidOut, 10)
	ctx.Docs.View(project.IndexID(), func(document client.Document) {
		key += ":" + strconv.FormatUint(document.Revision, 10)
	})
	s.runs.wait--
	if s.runs.asking || (key == s.runs.key && s.runs.wait > 0) {
		return
	}
	s.runs.asking, s.runs.key, s.runs.wait = true, key, callersEvery
	ctx.Requests.Runners(s.document, func(uses []protocol.Use, err error) {
		s.runs.asking = false
		if err == nil {
			s.runs.uses, s.runs.known = uses, true
		}
	})
	for _, use := range s.runs.uses {
		s.subscribe(use.Document, ctx)
	}
}

// runnersOf lists the NPCs that run the workflow, with where each lives;
// one whose zone was deleted lives nowhere.
func (s *State) runnersOf(docs frame.Docs) []runner {
	found := make([]runner, 0, len(s.runs.uses))
	index := s.names(docs)
	for _, use := range s.runs.uses {
		each := runner{
			npc: use.Document, name: use.Name, zone: ids.ULID{},
			home: use.Label == protocol.RunsAsBehaviour || use.Label == protocol.RunsAsLoop,
		}
		if use.Label == protocol.RunsAsLoop || use.Format == zone.FormatName {
			each.zone = use.Document
		}
		docs.View(use.Document, func(document client.Document) {
			if npcDoc, ok := document.Model.(*npcplugin.Document); ok {
				each.zone = npcDoc.NPC.Zone
			}
		})
		if _, exists := index.Entry(each.zone); !exists {
			each.zone = ids.ULID{}
		}
		found = append(found, each)
	}

	return found
}

// home heads the list of places by where the workflow runs: its zone's
// places first and the rest grey, flagged when chosen; or, when it runs in
// any zone, the extras a control offers first, then every place, each
// tying the workflow to its zone.
func (s *State) home(docs frame.Docs, extras []places.Extra) places.Home {
	name := s.name(docs)
	if zoneID := s.runsIn(docs); !zoneID.IsZero() {
		return places.Home{
			Zones: []ids.ULID{zoneID}, Title: name + " runs in " + s.runsInName(docs),
			Away: "Other zones: flagged when chosen here", Grey: true, Extras: nil,
		}
	}

	return places.Home{
		Zones: nil, Title: name + " runs in any zone", Away: "Or a fixed place: " + name + " then runs in its zone",
		Grey: false, Extras: extras,
	}
}

// andList joins names as "Clerk", "Clerk and Customer" or "Clerk, Customer
// and Fisher".
func andList(names []string) string {
	if len(names) < 2 { //nolint:mnd // One name needs no joining.
		return strings.Join(names, "")
	}

	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}
