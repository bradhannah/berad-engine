package workflowview

import (
	"strconv"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/places"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// The rows a Go to's or a Call input's list of places offers first while
// the workflow runs in any zone: a new place input its callers give it, or
// another place value it has.
const (
	extraNewInput = "new-place-input"
	extraAnother  = "another-value"
)

// placeExtras are the rows a place control offers before the places while
// the workflow runs in any zone.
func placeExtras() []places.Extra {
	return []places.Extra{
		{ID: extraNewInput, Label: "New place input…", Detail: "its callers give it"},
		{ID: extraAnother, Label: "Another value…", Detail: "a place value it has"},
	}
}

// declarePlaceInput declares a new place input, named so no local clashes,
// then hands then its ID and path, in the same gesture.
func (s *State) declarePlaceInput(ctx *frame.Context, then func(ids.ULID, string, string)) {
	taken := map[string]bool{}
	for _, value := range s.workflow.Locals().Values() {
		taken[value.Name] = true
	}
	name := "place"
	for number := 2; taken[name]; number++ {
		name = "place_" + strconv.Itoa(number)
	}
	made := worldstate.New(stateplugin.TempID(1), name, worldstate.ScopeLocal, worldstate.TypePlace)
	made.Role = worldstate.RoleInput
	gesture := s.gesture()
	s.command(ctx, workflowplugin.ChangeLocalsEdit, gesture, localChange(&made, made, ids.ULID{}),
		func(result protocol.Result) { then(result.Created, localPath(made), gesture) })
}

// localRef refers to one of the workflow's own local values.
func localRef(local ids.ULID) flow.ValueRef {
	return flow.ValueRef{Scope: worldstate.ScopeLocal, Value: local, Owner: ids.ULID{}, Found: ids.ULID{}}
}

// goToExtras gives a Go to's place control its extra rows: a new place
// input, which the Go to then goes to, or another place value, which the
// Go to is then set to go to.
func (s *State) goToExtras(target placeTarget, nodeID ids.ULID) placeTarget {
	target.extras = placeExtras()
	target.chose = func(ctx *frame.Context, extra string) {
		now, ok := s.currentNode(ctx.Docs, nodeID)
		if !ok {
			return
		}
		if extra == extraAnother {
			s.saveParam(ctx, now, flow.ParamGoes, flow.GoesToValue)
			ctx.Frame.Status("The Go to goes to a place value now. Choose it under Place value.")

			return
		}
		s.declarePlaceInput(ctx, func(created ids.ULID, path, gesture string) {
			current, ok := s.currentNode(ctx.Docs, nodeID)
			if !ok {
				return
			}
			changed, err := current.WithParam(flow.ParamGoes, flow.GoesToValue)
			if err == nil {
				changed, err = changed.WithParam(flow.ParamPlace, localRef(created))
			}
			if err != nil {
				ctx.Frame.Status(err.Error())

				return
			}
			s.command(ctx, workflowplugin.ReplaceEdit, gesture, workflowplugin.ReplaceArgs{Node: changed},
				func(protocol.Result) {
					ctx.Frame.Status("The Go to goes to " + path + ", a new input its callers give. Undo takes both back.")
				})
		})
	}

	return target
}

// inputExtras gives a Call input's place control its extra rows: a new
// place input of this workflow, which the input is then given, or another
// place value, chosen with the value picker.
func (s *State) inputExtras(target placeTarget, nodeID ids.ULID, input worldstate.Declaration) placeTarget {
	target.extras = placeExtras()
	target.chose = func(ctx *frame.Context, extra string) {
		now, ok := s.currentNode(ctx.Docs, nodeID)
		if !ok {
			return
		}
		if extra == extraAnother {
			s.openCallPicker(now, pickInput, input)

			return
		}
		s.declarePlaceInput(ctx, func(created ids.ULID, path, gesture string) {
			current, ok := s.currentNode(ctx.Docs, nodeID)
			if !ok {
				return
			}
			s.joining = gesture
			s.giveInput(ctx, current, input.ID, flow.RefOperand(localRef(created)))
			s.joining = ""
			ctx.Frame.Status("Its " + input.Name + " is " + path + ", a new input this workflow's callers give.")
		})
	}

	return target
}
