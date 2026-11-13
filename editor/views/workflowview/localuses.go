package workflowview

import (
	"strconv"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// A local value in use is refused, as a world value is (increment 58): the
// nodes that use it are listed, and Select selects the one chosen, so each
// can be changed before the local goes.

// localUseSelect is the button that selects the node chosen.
const localUseSelect = "local-use-select"

// refuseLocalInUse refuses deleting a local value that nodes use, and lists
// them. It reports whether it refused.
func (s *State) refuseLocalInUse(ctx *frame.Context, declared worldstate.Declaration) bool {
	if s.workflow == nil {
		return false
	}
	uses := workflowplugin.LocalUses(s.workflow, declared.ID)
	if len(uses) == 0 {
		return false
	}
	list := ui.NewList("local-uses", "")
	for _, node := range uses {
		kind, _ := flow.KindOf(node.Kind)
		list.Rows = append(list.Rows, ui.NewRow(node.ID.String(), kind.Label))
	}
	list.Selected = list.Rows[0].ID
	path := localPath(declared)
	used := "1 node"
	if len(uses) > 1 {
		used = strconv.Itoa(len(uses)) + " nodes"
	}
	s.dialog = ui.NewDialog(path+" is in use", []string{
		"It is used by " + used + ", and can be deleted once nothing uses it.", "Choose a node to select it.",
	}, ui.DialogButton(localUseSelect, "Select"), ui.DialogButton(ui.DialogCancel, "Close"))
	s.dialog.List = list
	s.dialogThen = func(*frame.Context, string) {
		if chosen, err := ids.Parse(list.Selected); err == nil {
			s.showing = chosen
			s.showNode()
		}
	}
	ctx.Frame.Refuse(path + " is used by " + used + ": it can be deleted once nothing uses it")

	return true
}
