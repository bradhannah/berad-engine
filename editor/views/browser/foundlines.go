package browser

import (
	"slices"
	"strings"

	"github.com/bradhannah/berad-engine/editor/client"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/workflowview"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/show"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// A search also lists the lines typed into workflows that have its words,
// each row's ID foundLine followed by "<workflow>:<node>".
const (
	foundLine = "found-line:"
	// foundLines is the Matches group of lines found, and linesLabel its
	// heading.
	foundLines = "found-lines"
	linesLabel = "Lines"
	// valueMark stands for a value's place in a line shown whole.
	valueMark = "‹value›"
)

// line is a node's line found by a search: its workflow and node, the
// workflow's name, the node's kind, and its text, markup's tags left out,
// with each value's place marked.
type line struct {
	workflow, node ids.ULID
	name, kind     string
	text           string
}

// lineThings are the lines typed into every workflow's Says, Show messages
// and Logs whose own words have the search's, a row each; none while only
// tags are searched.
func (s *State) lineThings(docs frame.Docs, match matcher) []thing {
	if !match.active() || match.tags {
		return nil
	}
	var found []thing
	for _, each := range s.findLines(docs, match.text) {
		row := ui.NewRow(foundLine+each.workflow.String()+":"+each.node.String(), each.name+" · "+each.kind)
		row.Icon, row.Tree, row.Detail = ui.IconWorkflow, true, excerpt(each.text, match.text)
		found = append(found, thing{row: row, matches: true, extra: true, values: nil, cells: [columnCount]string{}})
	}

	return found
}

// findLines lists the lines whose typed words have a query, in the
// project's order of workflows.
func (s *State) findLines(docs frame.Docs, query string) []line {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}
	var found []line
	for _, entry := range s.entries(docs, flow.FormatName) {
		docs.View(entry.ID, func(document client.Document) {
			workflowDoc, ok := document.Model.(*workflowplugin.Document)
			if !ok {
				return
			}
			for _, node := range workflowDoc.Workflow.Nodes() {
				typed, shown, has := lineOf(node)
				if has && strings.Contains(strings.ToLower(typed), query) {
					kind, _ := flow.KindOf(node.Kind)
					found = append(found, line{
						workflow: entry.ID, node: node.ID, name: entry.Name, kind: kind.Label, text: shown,
					})
				}
			}
		})
	}

	return found
}

// lineOf is a node's line: its typed text, markup's tags left out, which a
// search matches; the line as shown, with each value's place marked; and
// whether the node has a line at all.
func lineOf(node flow.Node) (string, string, bool) {
	kind, known := flow.KindOf(node.Kind)
	if !known || !slices.ContainsFunc(kind.Params, func(param flow.Param) bool {
		return param.Type == flow.TypeMessage
	}) {
		return "", "", false
	}
	segments, err := node.Message()
	if err != nil {
		return "", "", false
	}
	var typed, shown []string
	for _, segment := range segments {
		if segment.Ref != nil {
			shown = append(shown, valueMark)

			continue
		}
		typed = append(typed, segment.Text)
		shown = append(shown, segment.Text)
	}

	return show.Plain(strings.Join(typed, " ")), show.Plain(strings.Join(shown, "")), true
}

// selectedLine is the line found that is selected, as the search still
// finds it.
func (s *State) selectedLine(docs frame.Docs) (line, bool) {
	rest, isLine := strings.CutPrefix(s.list.Selected, foundLine)
	if !isLine {
		return line{workflow: ids.ULID{}, node: ids.ULID{}, name: "", kind: "", text: ""}, false
	}
	for _, each := range s.findLines(docs, newMatcher(s.search.Field.Value()).text) {
		if each.workflow.String()+":"+each.node.String() == rest {
			return each, true
		}
	}

	return line{workflow: ids.ULID{}, node: ids.ULID{}, name: "", kind: "", text: ""}, false
}

// openLine opens a found line's workflow at its node, and reports whether
// the selected row was one.
func (s *State) openLine(ctx *frame.Context) bool {
	rest, isLine := strings.CutPrefix(s.list.Selected, foundLine)
	if !isLine {
		return false
	}
	workflowText, nodeText, _ := strings.Cut(rest, ":")
	workflowID, err := ids.Parse(workflowText)
	if err == nil {
		ctx.Requests.OpenItem(workflowview.ViewName, workflowID, workflowview.NodeItem+nodeText)
	}

	return true
}
