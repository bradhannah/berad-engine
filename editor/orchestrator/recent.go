package orchestrator

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// Message directions recorded in the wire tap and failure reports.
const (
	directionIn  = "in"
	directionOut = "out"
)

// Recorded is one message as seen by the orchestrator.
type Recorded struct {
	At        time.Time        `json:"at"`
	Window    ids.ULID         `json:"window"`
	Direction string           `json:"direction"`
	Message   protocol.Message `json:"message"`
}

// ring keeps the most recent messages for one window.
type ring struct {
	items []Recorded
	next  int
	full  bool
}

func newRing(size int) *ring {
	return &ring{items: make([]Recorded, size), next: 0, full: false}
}

func (r *ring) add(item Recorded) {
	r.items[r.next] = item
	r.next = (r.next + 1) % len(r.items)
	if r.next == 0 {
		r.full = true
	}
}

// list returns the recorded messages, oldest first.
func (r *ring) list() []Recorded {
	if !r.full {
		return append([]Recorded(nil), r.items[:r.next]...)
	}

	return append(append([]Recorded(nil), r.items[r.next:]...), r.items[:r.next]...)
}

func writeTap(tap io.Writer, item Recorded) error {
	line, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("encode tap line: %w", err)
	}
	if _, err := tap.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("write tap: %w", err)
	}

	return nil
}
