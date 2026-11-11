package workflowview

import (
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// A workflow other workflows call shows who calls it, and its ways out, in
// a strip along the top of its canvas; while any Call uses it, its inputs'
// and outputs' types are fixed, as a used World state value's is.
const (
	stripHeight = 26
	// callersEvery is how many frames go by before the window asks again
	// who calls it, as nothing tells it when another workflow's Calls
	// change.
	callersEvery = 120
)

// callers is who calls the workflow, as last asked: the workflows, each
// once, and what was asked from.
type callers struct {
	names  []string
	key    string
	asking bool
	wait   int
	known  bool
}

// askCallers asks which workflows call this one when it or the project has
// changed, and now and then besides.
func (s *State) askCallers(ctx *frame.Context) {
	key := strconv.FormatUint(s.laidOut, 10)
	ctx.Docs.View(project.IndexID(), func(document client.Document) {
		key += ":" + strconv.FormatUint(document.Revision, 10)
	})
	s.callers.wait--
	if s.callers.asking || (key == s.callers.key && s.callers.wait > 0) {
		return
	}
	s.callers.asking, s.callers.key, s.callers.wait = true, key, callersEvery
	ctx.Requests.WhereUsed(s.document, workflowplugin.CallItem, func(uses []protocol.Use, err error) {
		s.callers.asking = false
		if err != nil {
			return
		}
		var names []string
		for _, use := range uses {
			if !slices.Contains(names, use.Name) {
				names = append(names, use.Name)
			}
		}
		s.callers.names, s.callers.known = names, true
	})
}

// callersLine says who calls the workflow and its ways out, as "Called by
// 2: Shopkeeper day, Guard rounds · Ways out: Arrived, Closed", or "" for a
// workflow nothing calls with no ways out.
func (s *State) callersLine() string {
	if s.workflow == nil {
		return ""
	}
	var parts []string
	if count := len(s.callers.names); count > 0 {
		parts = append(parts, "Called by "+strconv.Itoa(count)+": "+strings.Join(s.callers.names, ", "))
	}
	var ways []string
	for _, way := range s.workflow.WaysOut() {
		ways = append(ways, way.Label)
	}
	if len(ways) > 0 {
		parts = append(parts, "Ways out: "+strings.Join(ways, ", ")+", done")
	}

	return strings.Join(parts, " · ")
}

// drawStrip draws the strip along the top of the canvas.
func (s *State) drawStrip(screen *ebiten.Image, docs frame.Docs) {
	line := s.stripLine(docs)
	if line == "" {
		return
	}
	canvas := s.Canvas()
	strip := image.Rect(canvas.Min.X, canvas.Min.Y, canvas.Max.X, canvas.Min.Y+stripHeight)
	ui.FillRect(screen, strip, ui.ColorPage())
	ui.FillRect(screen, image.Rect(strip.Min.X, strip.Max.Y-ui.Hairline, strip.Max.X, strip.Max.Y), ui.ColorRule())
	s.theme.DrawText(screen, s.clip(line, strip.Dx()-ui.Half*ui.Inset), strip.Min.X+ui.Inset,
		strip.Min.Y+(stripHeight-ui.TextSize)/ui.Half-1, ui.ColorMuted())
}

// lockLocal fixes the type of an input or output while any Call uses the
// workflow, saying why.
func (s *State) lockLocal(declared worldstate.Declaration) string {
	count := len(s.callers.names)
	if declared.Role == worldstate.RoleLocal || count == 0 {
		return ""
	}
	calling := s.callers.names[0] + " calls this workflow"
	if count > 1 {
		calling = strconv.Itoa(count) + " workflows call this one"
	}

	return "Fixed while " + calling + ". For another type, add a new local."
}
