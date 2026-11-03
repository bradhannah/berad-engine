package workflow_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// logWith returns a Log node holding a message.
func logWith(t *testing.T, message []workflow.Segment) workflow.Node {
	t.Helper()
	made, err := workflow.NewNode(workflow.KindLog)
	if err != nil {
		t.Fatal(err)
	}
	made.ID = id(2)
	if made, err = made.WithParam(workflow.ParamMessage, message); err != nil {
		t.Fatal(err)
	}

	return made
}

// TestALogsMessage saves a Log's message of text and values and reads it
// back, offers Log in both kinds of workflow, and refuses a message that
// is not text and values or is too long.
func TestALogsMessage(t *testing.T) {
	t.Parallel()
	for _, purpose := range workflow.Purposes() {
		if !slices.ContainsFunc(workflow.KindsFor(purpose), func(kind workflow.Kind) bool {
			return kind.Name == workflow.KindLog
		}) {
			t.Fatalf("%s offers Log", purpose)
		}
	}
	zone := workflow.ValueRef{Scope: worldstate.ScopeNPC, Value: worldstate.ZoneID(), Owner: ids.ULID{}, Found: ids.ULID{}}
	message := []workflow.Segment{{Text: "Chasing in ", Ref: nil}, {Text: "", Ref: &zone}}
	saved, err := workflow.New(id(1))
	if err != nil {
		t.Fatal(err)
	}
	if err := saved.Insert([]workflow.Node{logWith(t, message)}, nil,
		[]workflow.Link{{From: id(1), Output: workflow.OutputNext, To: id(2)}}); err != nil {
		t.Fatal(err)
	}
	data, err := saved.Encode()
	if err != nil {
		t.Fatal(err)
	}
	read, err := workflow.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	logged, _ := read.Node(id(2))
	back, err := logged.Message()
	if err != nil || len(back) != 2 || back[0].Text != "Chasing in " || back[1].Ref == nil || *back[1].Ref != zone {
		t.Fatalf("a message reads back as its text and values: %+v %v", back, err)
	}
	both := workflow.Segment{Text: "and", Ref: &zone}
	many := slices.Repeat([]workflow.Segment{{Text: "", Ref: &zone}}, workflow.MaxSegments+1)
	long := []workflow.Segment{{Text: strings.Repeat("a", workflow.MaxMessageText+1), Ref: nil}}
	for name, wrong := range map[string][]workflow.Segment{
		"text and a value": {both}, "neither": {{Text: "", Ref: nil}}, "too many parts": many, "too long": long,
	} {
		if err := workflow.CheckParams(logWith(t, wrong)); !errors.Is(err, workflow.ErrNode) {
			t.Fatalf("a message part with %s is refused: %v", name, err)
		}
	}
}
