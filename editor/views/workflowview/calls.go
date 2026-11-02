package workflowview

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"slices"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/ui"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// A Call workflow's outputs are the ways out its module has now, with the
// ways its lines are drawn from that the module no longer has, struck
// through, and done. The window shows a Call so, whatever ways out it last
// saved; any change the window makes to the Call saves them.

// module is a workflow a Call names, as loaded: its name and a copy.
type module struct {
	name     string
	workflow *flow.Workflow
	revision uint64
}

// callees lists the workflows a workflow's Calls name, each once.
func callees(workflow *flow.Workflow) []ids.ULID { return workflow.Callees() }

// loadModules reads the workflows this one's Calls name, one at a time,
// and returns a key that changes when any of them does.
func (s *State) loadModules(docs frame.Docs, called []ids.ULID) string {
	s.modules = map[ids.ULID]module{}
	parts := make([]string, 0, len(called))
	for _, target := range called {
		made := module{name: s.workflowName(docs, target), workflow: nil, revision: 0}
		docs.View(target, func(document client.Document) {
			if workflowDoc, ok := document.Model.(*workflowplugin.Document); ok {
				made.workflow, made.revision = workflowDoc.Workflow.Clone(), document.Revision
			}
		})
		revision := "-"
		if made.workflow != nil {
			s.modules[target], revision = made, strconv.FormatUint(made.revision, 10)
		}
		parts = append(parts, target.String()+":"+revision)
	}

	return strings.Join(parts, "|")
}

// workflowName is a workflow's name in the project, "" when it has none.
func (s *State) workflowName(docs frame.Docs, target ids.ULID) string {
	name := ""
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			name = index.Name(target, "")
		}
	})

	return name
}

// showWays gives each Call in the window's copy the ways out its module has
// now, keeping those its lines are drawn from that the module lost, and
// marks those as missing.
func (s *State) showWays() {
	s.missingWays = map[ids.ULID]map[string]bool{}
	for _, node := range s.workflow.Nodes() {
		target, ok := node.Called()
		called, loaded := s.modules[target]
		if node.Kind != flow.KindCall || !ok || !loaded {
			continue
		}
		ways, missing := shownWays(node, called.workflow.WaysOut())
		shown, err := node.WithParam(flow.ParamWays, ways)
		if err != nil {
			continue
		}
		if _, err := s.workflow.Replace(shown); err == nil && len(missing) > 0 {
			s.missingWays[node.ID] = missing
		}
	}
}

// shownWays are a Call's ways out as shown: its module's, then those its
// lines are drawn from that the module no longer has, which are missing.
func shownWays(node flow.Node, live []flow.WayOut) ([]flow.WayOut, map[string]bool) {
	saved, err := node.Ways()
	if err != nil {
		saved = nil
	}
	ways, missing := slices.Clone(live), map[string]bool{}
	for _, way := range saved {
		_, linked := node.Next[way.Port]
		if linked && !slices.ContainsFunc(live, func(kept flow.WayOut) bool { return kept.Port == way.Port }) {
			ways, missing[way.Port] = append(ways, way), true
		}
	}

	return ways, missing
}

// drawCallNode draws a Call: its kind, the workflow it calls and what it
// gives its inputs, over a row for each way out and done.
func (s *State) drawCallNode(area *ebiten.Image, drawn shape, docs frame.Docs) {
	rect, width := s.screenRect(drawn.rect), drawn.rect.Dx()-ui.Half*textPad
	s.theme.DrawLabel(area, "Call workflow", rect.Min.X+textPad, rect.Min.Y+labelDrop, ui.ColorMuted())
	s.theme.DrawStrong(area, s.clip(s.callTitle(drawn.node, docs), width), rect.Min.X+textPad, rect.Min.Y+titleDrop,
		ui.ColorInk())
	s.theme.DrawSmall(area, s.clip(s.callInputs(drawn.node, docs), width), rect.Min.X+textPad,
		rect.Min.Y+titleDrop+noteLine+ui.Half, ui.ColorMuted())
	s.drawOutputRows(area, drawn)
}

// pressOrOpen opens what a Call calls on a double-click, or begins what a
// press begins.
func (s *State) pressOrOpen(ctx *frame.Context, point image.Point, double bool) {
	if !double || !s.openCalled(ctx, point) {
		s.press(ctx, point)
	}
}

// openCalled opens the workflow a Call under a point calls, in its own
// tab, and reports whether there was one.
func (s *State) openCalled(ctx *frame.Context, point image.Point) bool {
	hit, ok := s.nodeAt(point)
	node, found := s.workflow.Node(hit)
	target, calls := node.Called()
	if !ok || !found || node.Kind != flow.KindCall || !calls {
		return false
	}
	ctx.Requests.Open(ViewName, target, false)

	return true
}

// hasCallState answers "output:<kind>:<n>:<output>=<label>" when the nth
// node of a kind shows an output so named, and "missing:<kind>:<n>:<way>"
// when a Call's way out is struck through, its module having lost it.
func (s *State) hasCallState(state string) (bool, bool) {
	for _, prefix := range []string{"output:", "missing:"} {
		rest, ok := strings.CutPrefix(state, prefix)
		if !ok {
			continue
		}
		where, label, _ := strings.Cut(rest, "=")
		parts := strings.Split(where, ":")
		if len(parts) != portParts {
			return false, true
		}
		drawn, found := s.nthNode(parts[0] + ":" + parts[1])
		if prefix == "missing:" {
			return found && s.missingWays[drawn.node.ID][parts[2]], true
		}
		shown, has := drawn.portOf(parts[2])

		return found && has && shown.output.Label == label, true
	}

	return false, false
}

// moduleOf returns the module a Call names, if it is loaded.
func (s *State) moduleOf(node flow.Node) (module, bool) {
	target, ok := node.Called()
	if !ok {
		return module{name: "", workflow: nil, revision: 0}, false
	}
	called, loaded := s.modules[target]

	return called, loaded
}

// callTitle is what a Call's node says it calls.
func (s *State) callTitle(node flow.Node, docs frame.Docs) string {
	target, ok := node.Called()
	if !ok {
		return "No workflow chosen"
	}
	if name := s.workflowName(docs, target); name != "" {
		return name
	}

	return "A deleted workflow"
}

// callInputs says what a Call gives its module's inputs, as place = Shop
// counter · opens_at = 9; an input it leaves unfilled is not named.
func (s *State) callInputs(node flow.Node, docs frame.Docs) string {
	called, ok := s.moduleOf(node)
	if !ok {
		return ""
	}
	given := node.Inputs()
	var parts []string
	for _, input := range called.workflow.Inputs() {
		if raw, filled := given[input.ID]; filled {
			parts = append(parts, input.Name+" = "+s.givenLabel(docs, input, raw))
		}
	}
	if len(parts) == 0 {
		return "inputs as they start"
	}

	return strings.Join(parts, " · ")
}

// placeLabel names a place by its anchor, or says it is none.
func (s *State) placeLabel(docs frame.Docs, place worldstate.Place) string {
	if place.IsZero() {
		return "no place yet"
	}
	name, _ := s.anchorLabel(docs, floor.AnchorRef{Floor: place.Floor, Anchor: place.Anchor})

	return name
}

// givenLabel shows what a Call gives an input: another value's path, a
// place's name, or the value.
func (s *State) givenLabel(docs frame.Docs, input worldstate.Declaration, raw []byte) string {
	if ref, refers := flow.OperandRef(raw); refers {
		_, path, problem := s.resolve(docs, ref)
		if problem != "" {
			return problem
		}

		return path
	}
	value, err := input.Parse(raw)
	if err != nil {
		return "?"
	}
	switch input.Type {
	case worldstate.TypePlace:
		return s.placeLabel(docs, value.Place)
	case worldstate.TypeText:
		return textLabel(value.Text)
	case worldstate.TypeInteger, worldstate.TypeBool, worldstate.TypeChoice, worldstate.TypeNPC,
		worldstate.TypeGroup, worldstate.TypeTemplate:
	}

	return input.Label(value)
}
