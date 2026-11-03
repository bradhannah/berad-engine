package workflow

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// KindLog writes a line of text and values to the Watch window, for its
// author, and carries on at once.
const KindLog = "log"

// ParamMessage is a Log's message, and TypeMessage its type: text and
// values, in order.
const (
	ParamMessage             = "message"
	TypeMessage    ParamType = "message"
	MaxSegments              = 32
	MaxMessageText           = 200
)

// Segment is one part of a Log's message: text, or a value, by reference,
// printed as its text form.
type Segment struct {
	Text string    `json:"text,omitempty"`
	Ref  *ValueRef `json:"ref,omitempty"`
}

// logKind declares Log, in the Annotate palette beside Note.
func logKind(next []Output) Kind {
	return Kind{
		Name: KindLog, Label: "Log", Group: GroupAnnotate,
		Params: []Param{{
			Name: ParamMessage, Label: "Message", Type: TypeMessage, Default: mustJSON([]Segment{}), Choices: nil,
		}},
		Outputs: next,
	}
}

// Message returns a Log's message.
func (n Node) Message() ([]Segment, error) {
	var segments []Segment

	return segments, n.decode(ParamMessage, &segments)
}

// checkMessage reads a message: at most MaxSegments parts, each text or a
// value, its text at most MaxMessageText characters in all.
func checkMessage(value json.RawMessage) error {
	var segments []Segment
	if err := json.Unmarshal(value, &segments); err != nil {
		return err //nolint:wrapcheck // Wrapped by the caller.
	}
	if len(segments) > MaxSegments {
		return fmt.Errorf("%w: a message has at most %d parts", ErrNode, MaxSegments)
	}
	text := 0
	for _, segment := range segments {
		if (segment.Ref == nil) == (segment.Text == "") {
			return fmt.Errorf("%w: each part of a message is text or a value", ErrNode)
		}
		text += utf8.RuneCountInString(segment.Text)
	}
	if text > MaxMessageText {
		return fmt.Errorf("%w: a message's text is at most %d characters", ErrNode, MaxMessageText)
	}

	return nil
}

// checkValues reads a parameter that refers to values: a value, an operand,
// or a Log's message.
func checkValues(paramType ParamType, value json.RawMessage) error {
	if paramType == TypeMessage {
		return checkMessage(value)
	}
	if paramType == TypeOperand {
		return checkOperand(value)
	}

	return checkRef(value)
}
