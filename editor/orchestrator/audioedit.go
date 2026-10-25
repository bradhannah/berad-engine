package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/protocol"
)

// Audio edits change the project's audio library through berad-audio's
// authoring package (D11). One runs at a time, on a worker goroutine, so
// decoding and copying a large recording never holds up the loop; the loop
// then adopts the result: it replies, records the undo step and tells every
// window the library changed. An audio edit is written to the library's files
// before it is acknowledged, so it needs no journal (D9).
//
// Each job opens the library afresh, so it always starts from the files as
// they are, whoever changed them last. An undo step holds the values its edit
// changed, before and after, and is refused when those values have changed
// since, in this editor or outside it (D3).

// audioPatchEdit names audio actions in undo histories.
const audioPatchEdit = "audio.patch"

// Reasons an audio edit or its undo is refused.
const (
	reasonAudioSaving  = "the last sound change is still being saved; try again in a moment"
	reasonAudioChanged = "this sound was changed outside the editor since, so it cannot be undone"
)

type audioJobKind int

const (
	audioEditJob audioJobKind = iota
	audioUndoJob
	audioRedoJob
)

// audioJob is an audio edit, or the undo or redo of an audio step, waiting
// for the worker.
type audioJob struct {
	from    *window
	request uint64
	kind    audioJobKind
	edit    protocol.AudioEdit
	step    step
}

// audioOutcome is a finished job: the values it changed, what it created and
// whether it failed.
type audioOutcome struct {
	job     audioJob
	change  audioChange
	created string
	err     error
}

// audioEdits is the queue of audio jobs and what windows were last told.
type audioEdits struct {
	queue   []audioJob
	running bool
	// current is the undo group of the running job.
	current string
	done    chan audioOutcome
	// revision counts the changes to the audio library the orchestrator has
	// seen; stamp is the library's files as last told to windows.
	revision uint64
	stamp    string
}

func newAudioEdits() *audioEdits {
	return &audioEdits{queue: nil, running: false, current: "", done: make(chan audioOutcome), revision: 0, stamp: ""}
}

// handleAudioEdit queues a window's audio edit.
func (o *Orchestrator) handleAudioEdit(from *window, message protocol.Message) {
	edit, err := protocol.DecodePayload[protocol.AudioEdit](message)
	if err != nil {
		o.reply(from, message.Request, err.Error(), 0)

		return
	}
	o.queueAudio(audioJob{from: from, request: message.Request, kind: audioEditJob, edit: edit, step: noStep()})
}

func noStep() step { return step{gesture: "", actions: nil} }

func noAudioEdit() protocol.AudioEdit { return protocol.AudioEdit{Edit: "", Gesture: "", Args: nil} }

// isAudioStep reports whether an undo step changed the audio library.
func isAudioStep(candidate step) bool {
	return len(candidate.actions) > 0 && candidate.actions[0].document == model.AudioLibraryID()
}

// audioPending reports whether an audio job of an undo group waits or runs:
// until it finishes, that group's latest step is not recorded yet.
func (o *Orchestrator) audioPending(group string) bool {
	for _, job := range o.edits.queue {
		if job.from.group == group {
			return true
		}
	}

	return o.edits.running && o.edits.current == group
}

func (o *Orchestrator) queueAudio(job audioJob) {
	o.edits.queue = append(o.edits.queue, job)
	o.startAudio()
}

// startAudio starts the next audio job, unless one is running.
func (o *Orchestrator) startAudio() {
	for !o.edits.running && len(o.edits.queue) > 0 {
		job := o.edits.queue[0]
		o.edits.queue = o.edits.queue[1:]
		files, problem := o.audio.files()
		if problem != "" {
			o.refuseAudio(job, problem)

			continue
		}
		o.edits.running, o.edits.current = true, job.from.group
		go func() {
			outcome := runAudioJob(files, job)
			select {
			case o.edits.done <- outcome:
			case <-o.stopped:
			}
		}()
	}
}

// refuseAudio answers a job that could not be carried out, and puts back the
// undo or redo step it took.
func (o *Orchestrator) refuseAudio(job audioJob, reason string) {
	switch job.kind {
	case audioUndoJob:
		o.history.pushUndo(job.from.group, job.step)
	case audioRedoJob:
		o.history.pushRedo(job.from.group, job.step)
	case audioEditJob:
	}
	o.reply(job.from, job.request, reason, 0)
}

// audioFinished adopts a finished job: it records the undo step, replies,
// tells every window and starts the next job.
func (o *Orchestrator) audioFinished(outcome audioOutcome) {
	o.edits.running, o.edits.current = false, ""
	job := outcome.job
	if outcome.err != nil {
		o.refuseAudio(job, outcome.err.Error())
		o.startAudio()

		return
	}
	o.seq++
	touched := outcome.change.touched
	switch job.kind {
	case audioEditJob:
		if len(touched) > 0 {
			o.history.record(job.from.group, job.edit.Gesture, action{
				document: model.AudioLibraryID(), edit: audioPatchEdit, backEdit: "", forward: outcome.change.forward,
				back: outcome.change.back, touched: touched, covers: nil, seq: o.seq,
			})
		}
	case audioUndoJob:
		o.history.pushRedo(job.from.group, restamp(job.step, o.seq))
	case audioRedoJob:
		o.history.pushUndo(job.from.group, restamp(job.step, o.seq))
	}
	o.history.noteWrite(job.from.group, model.AudioLibraryID(), touched, o.seq)
	o.send(job.from, mustMessage(protocol.KindResult, job.request, protocol.Result{
		OK: true, Reason: "", Revision: o.edits.revision + 1, Created: model.AudioLibraryID(), Item: outcome.created,
	}))
	o.audioChanged()
	o.startAudio()
}

// restamp gives a reapplied step the sequence number of its new write, so a
// later change from another window is seen as later (D3).
func restamp(applied step, seq uint64) step {
	actions := slices.Clone(applied.actions)
	for index := range actions {
		actions[index].seq = seq
	}

	return step{gesture: applied.gesture, actions: actions}
}

// audioChanged tells every window that the audio library changed.
func (o *Orchestrator) audioChanged() {
	o.edits.revision++
	o.edits.stamp = o.audio.stamp()
	message := mustMessage(protocol.KindAudioChanged, 0, protocol.AudioChanged{Revision: o.edits.revision})
	for _, connected := range o.windows {
		o.send(connected, message)
	}
}

// watchAudio tells windows when the audio library's files changed outside the
// editor, such as in the terminal audio editor.
func (o *Orchestrator) watchAudio() {
	if o.edits.running {
		return
	}
	if stamp := o.audio.stamp(); stamp != o.edits.stamp {
		o.audioChanged()
	}
}

// waitForAudio lets a running audio job finish writing before shutdown.
func (o *Orchestrator) waitForAudio(deadline time.Time) {
	if !o.edits.running {
		return
	}
	select {
	case outcome := <-o.edits.done:
		o.edits.running = false
		o.reply(outcome.job.from, outcome.job.request, "the editor is closing", 0)
	case <-time.After(time.Until(deadline)):
	}
}

// audioFiles are the absolute paths an audio job reads and writes: the
// audio project descriptor, which may not exist, and the library.
type audioFiles struct {
	descriptor string
	library    string
}

// files says where audio jobs read and write, or why they cannot.
func (a *projectAudio) files() (audioFiles, string) {
	place, problem := a.locate()
	if problem != "" {
		return audioFiles{descriptor: "", library: ""}, problem
	}

	return audioFiles{
		descriptor: filepath.Join(a.dir, filepath.FromSlash(audioDescriptor)),
		library:    filepath.Join(a.dir, filepath.FromSlash(place.library)),
	}, ""
}

// stamp describes the audio files' sizes and times, which change whenever
// the files do.
func (a *projectAudio) stamp() string {
	place, problem := a.locate()
	parts := []string{problem}
	for _, name := range []string{audioDescriptor, place.library} {
		info, err := a.root.Stat(name)
		if name == "" || err != nil {
			parts = append(parts, "-")

			continue
		}
		parts = append(parts, fmt.Sprintf("%d-%d", info.Size(), info.ModTime().UnixNano()))
	}

	return strings.Join(parts, "|")
}

// runAudioJob carries out a job on the worker: it opens the library, applies
// the edit or the undo step, saves, and reports the values that changed.
func runAudioJob(files audioFiles, job audioJob) audioOutcome {
	outcome := audioOutcome{job: job, change: noChange(), created: "", err: nil}
	session, err := openAudioSession(files, job.kind == audioEditJob && createsCue(job.edit.Edit))
	if err != nil {
		outcome.err = err

		return outcome
	}
	before := session.entries()
	switch job.kind {
	case audioEditJob:
		outcome.created, err = session.apply(job.edit)
	case audioUndoJob:
		err = session.replay(job.step, true)
	case audioRedoJob:
		err = session.replay(job.step, false)
	}
	if err == nil {
		err = session.save()
	}
	if err != nil {
		outcome.err = errors.Join(err, session.discard())

		return outcome
	}
	outcome.change = diffEntries(before, session.entries())

	return outcome
}

// createsCue reports whether an edit may start the library: only one that
// adds a cue.
func createsCue(edit string) bool {
	return edit == protocol.AudioCreateCue || edit == protocol.AudioAddSound
}

// encodeValue encodes a schema value for a patch; schema values always encode.
func encodeValue(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}

	return string(data)
}

// removeCreated removes a file this job created when the job fails.
func removeCreated(path string, created bool) error {
	if !created {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove the unused audio library: %w", err)
	}

	return nil
}
