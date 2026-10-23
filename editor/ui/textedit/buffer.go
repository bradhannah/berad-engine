// Package textedit is the editor's text editing core, kept apart from
// Ebitengine and from drawing so it can be tested without a screen: a buffer
// with a caret, a selection and its own undo, and a field that keeps a draft
// until the author saves it.
//
// Offsets are UTF-8 byte offsets, always on grapheme cluster boundaries, so an
// accented letter or an emoji is never split.
package textedit

import (
	"strings"
	"unicode"

	"github.com/rivo/uniseg"
)

// maxHistory bounds the undo steps a buffer keeps.
const maxHistory = 200

type snapshot struct {
	text   string
	caret  int
	anchor int
}

type editKind int

const (
	editOther editKind = iota
	editTyping
	editDeleting
)

// Buffer is editable text with a caret and an optional selection between the
// caret and an anchor.
type Buffer struct {
	text    string
	caret   int
	anchor  int
	past    []snapshot
	future  []snapshot
	last    editKind
	lastEnd int
}

// NewBuffer returns a buffer holding text with the caret at the end.
func NewBuffer(text string) *Buffer {
	return &Buffer{
		text: text, caret: len(text), anchor: len(text),
		past: nil, future: nil, last: editOther, lastEnd: -1,
	}
}

// Text returns the buffer's text.
func (b *Buffer) Text() string { return b.text }

// Caret returns the caret offset.
func (b *Buffer) Caret() int { return b.caret }

// Selection returns the selected range, start before end; empty when nothing
// is selected.
func (b *Buffer) Selection() (int, int) {
	return min(b.caret, b.anchor), max(b.caret, b.anchor)
}

// HasSelection reports whether any text is selected.
func (b *Buffer) HasSelection() bool { return b.caret != b.anchor }

// SelectedText returns the selected text.
func (b *Buffer) SelectedText() string {
	start, end := b.Selection()

	return b.text[start:end]
}

// SetCaret moves the caret to the nearest grapheme boundary at or before an
// offset; extend keeps the anchor, selecting the text between.
func (b *Buffer) SetCaret(offset int, extend bool) {
	b.caret = b.snap(offset)
	if !extend {
		b.anchor = b.caret
	}
	b.last = editOther
}

// Select selects a range, with the caret at its end.
func (b *Buffer) Select(start, end int) {
	b.anchor = b.snap(start)
	b.caret = b.snap(end)
	b.last = editOther
}

// SelectAll selects the whole text.
func (b *Buffer) SelectAll() { b.Select(0, len(b.text)) }

// Insert replaces the selection with text, leaving the caret after it.
// Consecutive typing is undone as one step.
func (b *Buffer) Insert(value string) {
	if value == "" && !b.HasSelection() {
		return
	}
	start, end := b.Selection()
	kind := editTyping
	if strings.ContainsAny(value, " \n") || b.HasSelection() || len(value) > 1 && uniseg.GraphemeClusterCount(value) > 1 {
		kind = editOther
	}
	b.record(kind, start)
	b.text = b.text[:start] + value + b.text[end:]
	b.caret = start + len(value)
	b.anchor = b.caret
	b.lastEnd = b.caret
}

// ReplaceAround replaces before bytes ahead of the selection and after bytes
// following it with replacement, leaving the caret caretInReplacement bytes
// into it. Input methods use this to replace text they inserted, such as the
// plain letter that pressing and holding a key turns into an accented one.
func (b *Buffer) ReplaceAround(before, after int, replacement string, caretInReplacement int) {
	start, end := b.Selection()
	from := max(start-before, 0)
	to := min(end+after, len(b.text))
	b.record(editOther, from)
	b.text = b.text[:from] + replacement + b.text[to:]
	b.caret = b.snap(from + min(max(caretInReplacement, 0), len(replacement)))
	b.anchor = b.caret
}

// DeleteBackward deletes the selection, or the grapheme (or word) before the
// caret.
func (b *Buffer) DeleteBackward(word bool) {
	if !b.HasSelection() {
		target := b.previous(b.caret)
		if word {
			target = b.wordStart(b.caret)
		}
		b.anchor = target
	}
	b.deleteSelection()
}

// DeleteToLineStart deletes from the start of the caret's line to the caret.
func (b *Buffer) DeleteToLineStart(lineStart int) {
	if !b.HasSelection() {
		b.anchor = b.snap(lineStart)
	}
	b.deleteSelection()
}

// DeleteForward deletes the selection, or the grapheme (or word) after the
// caret.
func (b *Buffer) DeleteForward(word bool) {
	if !b.HasSelection() {
		target := b.next(b.caret)
		if word {
			target = b.wordEnd(b.caret)
		}
		b.anchor = target
	}
	b.deleteSelection()
}

func (b *Buffer) deleteSelection() {
	start, end := b.Selection()
	if start == end {
		return
	}
	b.record(editDeleting, end)
	b.text = b.text[:start] + b.text[end:]
	b.caret, b.anchor, b.lastEnd = start, start, start
}

// MoveLeft moves the caret one grapheme or word left. Without extend, a
// selection collapses to its start instead.
func (b *Buffer) MoveLeft(extend, word bool) {
	start, _ := b.Selection()
	if b.HasSelection() && !extend {
		b.SetCaret(start, false)

		return
	}
	target := b.previous(b.caret)
	if word {
		target = b.wordStart(b.caret)
	}
	b.SetCaret(target, extend)
}

// MoveRight moves the caret one grapheme or word right. Without extend, a
// selection collapses to its end instead.
func (b *Buffer) MoveRight(extend, word bool) {
	_, end := b.Selection()
	if b.HasSelection() && !extend {
		b.SetCaret(end, false)

		return
	}
	target := b.next(b.caret)
	if word {
		target = b.wordEnd(b.caret)
	}
	b.SetCaret(target, extend)
}

// SelectWord selects the word around an offset.
func (b *Buffer) SelectWord(offset int) {
	offset = b.snap(offset)
	start, end := offset, offset
	for start > 0 {
		previous := b.previous(start)
		if !isWordText(b.text[previous:start]) {
			break
		}
		start = previous
	}
	for end < len(b.text) {
		next := b.next(end)
		if !isWordText(b.text[end:next]) {
			break
		}
		end = next
	}
	b.Select(start, end)
}

// Undo restores the text before the latest step.
func (b *Buffer) Undo() bool {
	if len(b.past) == 0 {
		return false
	}
	b.future = append(b.future, b.current())
	b.restore(b.past[len(b.past)-1])
	b.past = b.past[:len(b.past)-1]

	return true
}

// Redo re-applies the latest undone step.
func (b *Buffer) Redo() bool {
	if len(b.future) == 0 {
		return false
	}
	b.past = append(b.past, b.current())
	b.restore(b.future[len(b.future)-1])
	b.future = b.future[:len(b.future)-1]

	return true
}

// CanUndo reports whether the buffer has its own steps to undo.
func (b *Buffer) CanUndo() bool { return len(b.past) > 0 }

// LineStart returns the start of the hard line holding an offset.
func (b *Buffer) LineStart(offset int) int {
	return strings.LastIndexByte(b.text[:offset], '\n') + 1
}

// LineEnd returns the end of the hard line holding an offset, before its
// line break.
func (b *Buffer) LineEnd(offset int) int {
	if index := strings.IndexByte(b.text[offset:], '\n'); index >= 0 {
		return offset + index
	}

	return len(b.text)
}

// record saves the state before a change; typing or deleting at the place the
// previous change of the same kind ended joins that step.
func (b *Buffer) record(kind editKind, at int) {
	b.future = nil
	if kind != editOther && kind == b.last && at == b.lastEnd && len(b.past) > 0 {
		return
	}
	b.past = append(b.past, b.current())
	if len(b.past) > maxHistory {
		b.past = b.past[len(b.past)-maxHistory:]
	}
	b.last = kind
}

func (b *Buffer) current() snapshot {
	return snapshot{text: b.text, caret: b.caret, anchor: b.anchor}
}

func (b *Buffer) restore(state snapshot) {
	b.text, b.caret, b.anchor = state.text, state.caret, state.anchor
	b.last = editOther
}

// snap clamps an offset into the text and moves it back to a grapheme
// boundary.
func (b *Buffer) snap(offset int) int {
	offset = min(max(offset, 0), len(b.text))
	position := 0
	state := -1
	rest := b.text
	for rest != "" {
		cluster, remaining, _, newState := uniseg.FirstGraphemeClusterInString(rest, state)
		if position+len(cluster) > offset {
			return position
		}
		position += len(cluster)
		rest, state = remaining, newState
	}

	return position
}

// next returns the boundary after the grapheme at an offset.
func (b *Buffer) next(offset int) int {
	if offset >= len(b.text) {
		return len(b.text)
	}
	graphemes := uniseg.NewGraphemes(b.text[offset:])
	graphemes.Next()
	_, end := graphemes.Positions()

	return offset + end
}

// previous returns the boundary before the grapheme ending at an offset.
func (b *Buffer) previous(offset int) int {
	position := 0
	last := 0
	state := -1
	rest := b.text
	for rest != "" && position < offset {
		cluster, remaining, _, newState := uniseg.FirstGraphemeClusterInString(rest, state)
		last = position
		position += len(cluster)
		rest, state = remaining, newState
	}

	return last
}

// wordStart skips spaces and punctuation, then a word, leftwards.
func (b *Buffer) wordStart(offset int) int {
	position := offset
	for position > 0 && !isWordText(b.text[b.previous(position):position]) {
		position = b.previous(position)
	}
	for position > 0 && isWordText(b.text[b.previous(position):position]) {
		position = b.previous(position)
	}

	return position
}

// wordEnd skips spaces and punctuation, then a word, rightwards.
func (b *Buffer) wordEnd(offset int) int {
	position := offset
	for position < len(b.text) && !isWordText(b.text[position:b.next(position)]) {
		position = b.next(position)
	}
	for position < len(b.text) && isWordText(b.text[position:b.next(position)]) {
		position = b.next(position)
	}

	return position
}

func isWordText(cluster string) bool {
	for _, char := range cluster {
		if unicode.IsLetter(char) || unicode.IsDigit(char) || char == '_' || char == '\'' {
			return true
		}
	}

	return false
}
