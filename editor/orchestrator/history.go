package orchestrator

import (
	"encoding/json"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// action is one applied edit inside an undo step.
type action struct {
	document ids.ULID
	edit     string
	// forward re-applies the action; back reverts it.
	forward json.RawMessage
	back    json.RawMessage
	touched []string
	seq     uint64
}

// step is one undo step: a single edit or a whole gesture.
type step struct {
	gesture string
	actions []action
}

type undoList struct {
	undo    []step
	redo    []step
	members int
}

type lastWriter struct {
	seq   uint64
	group string
}

// history keeps one undo list per undo group and remembers who last changed
// each item, so an undo never silently overwrites a later change from another
// window.
type history struct {
	limit   int
	groups  map[string]*undoList
	writers map[string]lastWriter
}

func newHistory(limit int) *history {
	return &history{limit: limit, groups: map[string]*undoList{}, writers: map[string]lastWriter{}}
}

func (h *history) join(group string) {
	list, ok := h.groups[group]
	if !ok {
		list = &undoList{undo: nil, redo: nil, members: 0}
		h.groups[group] = list
	}
	list.members++
}

// leave drops a group's undo list when its last window closes.
func (h *history) leave(group string) {
	list, ok := h.groups[group]
	if !ok {
		return
	}
	list.members--
	if list.members <= 0 {
		delete(h.groups, group)
	}
}

// record adds an action to the group's history, extending the latest step when
// it belongs to the same gesture, and clears the redo list.
func (h *history) record(group, gesture string, applied action) {
	list, ok := h.groups[group]
	if !ok {
		return
	}
	list.redo = nil
	last := len(list.undo) - 1
	if gesture != "" && last >= 0 && list.undo[last].gesture == gesture {
		list.undo[last].actions = append(list.undo[last].actions, applied)

		return
	}
	list.undo = append(list.undo, step{gesture: gesture, actions: []action{applied}})
	if len(list.undo) > h.limit {
		list.undo = list.undo[len(list.undo)-h.limit:]
	}
}

func (h *history) noteWrite(group string, document ids.ULID, touched []string, seq uint64) {
	for _, item := range touched {
		h.writers[itemKey(document, item)] = lastWriter{seq: seq, group: group}
	}
}

// clash reports whether another group changed any item in the step after it.
func (h *history) clash(group string, candidate step) bool {
	for _, applied := range candidate.actions {
		for _, item := range applied.touched {
			writer, ok := h.writers[itemKey(applied.document, item)]
			if ok && writer.group != group && writer.seq > applied.seq {
				return true
			}
		}
	}

	return false
}

func (h *history) popUndo(group string) (step, bool) {
	return pop(h.groups[group], func(list *undoList) *[]step { return &list.undo })
}

func (h *history) popRedo(group string) (step, bool) {
	return pop(h.groups[group], func(list *undoList) *[]step { return &list.redo })
}

func (h *history) pushUndo(group string, item step) {
	if list, ok := h.groups[group]; ok {
		list.undo = append(list.undo, item)
	}
}

func (h *history) pushRedo(group string, item step) {
	if list, ok := h.groups[group]; ok {
		list.redo = append(list.redo, item)
	}
}

func pop(list *undoList, pick func(*undoList) *[]step) (step, bool) {
	if list == nil {
		return step{gesture: "", actions: nil}, false
	}
	stack := pick(list)
	if len(*stack) == 0 {
		return step{gesture: "", actions: nil}, false
	}
	last := (*stack)[len(*stack)-1]
	*stack = (*stack)[:len(*stack)-1]

	return last, true
}

func itemKey(document ids.ULID, item string) string {
	return document.String() + "/" + item
}
