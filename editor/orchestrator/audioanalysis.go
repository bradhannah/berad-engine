package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/bradhannah/berad-sdk/modules/berad-audio/authoring"
	"github.com/bradhannah/berad-sdk/modules/berad-audio/backend/ebitengine"

	"github.com/bradhannah/berad-engine/editor/protocol"
)

// Analyses decode recordings for the Audio window's waveform, suggestions and
// checks. They only read, so each runs on its own goroutine, a few at a time,
// and the loop sends the answer. The last recordings decoded are kept, so
// zooming in or moving a region does not decode them again.

// Analysis limits: how many run at once, how many decoded recordings are
// kept, the most peaks one answer carries, and how long one may take.
const (
	analysesAtOnce  = 2
	analysesKept    = 4
	maxColumns      = 4096
	analysisTimeout = 2 * time.Minute
	// Peaks run from silenceDB, sent as 0, to 0 dB, sent as 255.
	silenceDB   = -72.0
	peakLevels  = 255
	amplitudeDB = 20.0
	// soundGapMs is the shortest silence that parts two sounds of one file.
	soundGapMs = 150
	// measuredKept bounds the measurements kept; past it they start afresh.
	measuredKept = 4096
)

// errNoRecordingFile refuses to analyse a recording with no file.
var errNoRecordingFile = errors.New("this recording has no file to analyse")

// analysisDone is an answer for the loop to send: a message, or a reason a
// question was refused.
type analysisDone struct {
	to      *window
	request uint64
	kind    protocol.Kind
	payload any
	reason  string
}

// audioAnalyses runs analyses and keeps the recordings they decoded, and
// how loud each recording measured, by its file, region and policy.
type audioAnalyses struct {
	slots    chan struct{}
	done     chan analysisDone
	mu       sync.Mutex
	kept     []keptRecording
	measured map[string]protocol.RecordingLoudness
}

// keptRecording is a decoded recording, by its file and the file's version.
type keptRecording struct {
	key  string
	clip *authoring.RecordingAudio
}

func newAudioAnalyses() *audioAnalyses {
	return &audioAnalyses{
		slots: make(chan struct{}, analysesAtOnce), done: make(chan analysisDone), mu: sync.Mutex{}, kept: nil,
		measured: map[string]protocol.RecordingLoudness{},
	}
}

// handleAnalysis starts answering a window's analysis or check question,
// which stops when the orchestrator does.
func (o *Orchestrator) handleAnalysis(ctx context.Context, from *window, message protocol.Message) {
	files, problem := o.audio.files()
	if problem != "" {
		o.reply(from, message.Request, problem, 0)

		return
	}
	analyses := o.analyses
	go func() {
		analyses.slots <- struct{}{}
		answer := analyses.answer(ctx, files, message)
		<-analyses.slots
		answer.to, answer.request = from, message.Request
		select {
		case analyses.done <- answer:
		case <-o.stopped:
		}
	}()
}

// analysisFinished sends an answer from the loop.
func (o *Orchestrator) analysisFinished(answer analysisDone) {
	if answer.reason != "" {
		o.reply(answer.to, answer.request, answer.reason, 0)

		return
	}
	o.send(answer.to, mustMessage(answer.kind, answer.request, answer.payload))
}

// answer works out an answer on its goroutine.
func (a *audioAnalyses) answer(ctx context.Context, files audioFiles, message protocol.Message) analysisDone {
	if message.Kind == protocol.KindAudioLoudness {
		payload, err := a.loudness(ctx, files)

		return answered(message.Kind, payload, err)
	}
	var payload any
	var err error
	if message.Kind == protocol.KindAudioCheck {
		payload, err = check(ctx, files)
	} else {
		var asked protocol.AudioAnalysis
		asked, err = protocol.DecodePayload[protocol.AudioAnalysis](message)
		if err == nil {
			payload, err = a.analyse(ctx, files, asked)
		}
	}

	return answered(message.Kind, payload, err)
}

// answered is an answer of a kind for the loop to send, or the reason it
// failed.
func answered(kind protocol.Kind, payload any, err error) analysisDone {
	if err != nil {
		return analysisDone{to: nil, request: 0, kind: kind, payload: nil, reason: err.Error()}
	}

	return analysisDone{to: nil, request: 0, kind: kind, payload: payload, reason: ""}
}

// openForReading opens the library to read from, as it is on disk, measuring
// to its reference loudness.
func openForReading(files audioFiles) (*authoring.LibraryEditor, error) {
	options := evenedOptions(nil, func() float64 { return authoring.DefaultCalibrationOptions().TargetDB })
	if data, err := os.ReadFile(files.descriptor); err == nil {
		descriptor, err := authoring.ParseProjectDescriptor(data)
		if err != nil {
			return nil, fmt.Errorf("read the audio project: %w", err)
		}
		reference := referenceOf(descriptor)
		options = evenedOptions(&descriptor, func() float64 { return reference })
	}
	library, err := authoring.OpenLibraryProject(files.library, options)
	if err != nil {
		return nil, fmt.Errorf("open the audio library: %w", err)
	}

	return library, nil
}

// analyse answers a waveform question.
func (a *audioAnalyses) analyse(
	ctx context.Context, files audioFiles, asked protocol.AudioAnalysis,
) (protocol.AudioAnalysis, error) {
	library, err := openForReading(files)
	if err != nil {
		return asked, err
	}
	path, err := library.RecordingPath(asked.Recording)
	if err != nil {
		return asked, errNoRecordingFile
	}
	clip, version, err := a.decoded(ctx, path)
	if err != nil {
		asked.Problem = "The file could not be decoded: " + authoringError(err).Error()

		return asked, nil
	}
	asked.Version, asked.DurationMs = version, clip.DurationMs()
	asked.Peaks = peaks(clip, asked.From, asked.To, asked.Columns)
	if suggested, ok := clip.SuggestedWindow(); ok {
		asked.Suggested = &protocol.AudioWindow{StartMs: suggested.StartMs, EndMs: suggested.EndMs}
	}
	for _, sound := range clip.SuggestedWindows(soundGapMs) {
		asked.Sounds = append(asked.Sounds, protocol.AudioWindow{StartMs: sound.StartMs, EndMs: sound.EndMs})
	}
	options, err := library.CalibrationOptionsFor(asked.Recording)
	if err != nil {
		return asked, authoringError(err)
	}
	region := library.Recording(asked.Recording).PlaybackWindow
	if asked.Region != nil {
		region = playbackWindow(asked.Region)
	}
	if suggestion, err := clip.SuggestCalibration(region, options); err == nil {
		asked.Calibration = &protocol.AudioCalibration{
			Method: suggestion.Method, MeasuredDB: suggestion.MeasuredDB, TargetDB: suggestion.TargetDB,
			PeakDB: suggestion.PeakDB, GainDB: suggestion.GainDB, Limited: suggestion.Limited,
		}
	}

	return asked, nil
}

// decoded returns a recording decoded, from those kept when its file has not
// changed, and the file's version.
func (a *audioAnalyses) decoded(ctx context.Context, path string) (*authoring.RecordingAudio, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", fmt.Errorf("find the recording: %w", err)
	}
	version := strconv.FormatInt(info.Size(), 36) + "-" + strconv.FormatInt(info.ModTime().UnixNano(), 36)
	key := path + "|" + version
	a.mu.Lock()
	for _, kept := range a.kept {
		if kept.key == key {
			a.mu.Unlock()

			return kept.clip, version, nil
		}
	}
	a.mu.Unlock()
	limited, cancel := context.WithTimeout(ctx, analysisTimeout)
	defer cancel()
	clip, err := authoring.LoadRecordingContext(limited, path, ebitengine.Decode)
	if err != nil {
		return nil, "", fmt.Errorf("decode the recording: %w", err)
	}
	a.mu.Lock()
	a.kept = append(a.kept, keptRecording{key: key, clip: clip})
	if len(a.kept) > analysesKept {
		a.kept = a.kept[1:]
	}
	a.mu.Unlock()

	return clip, version, nil
}

// peaks measures the loudest moment of each column between from and to, all
// of the recording when to is not after from.
func peaks(clip *authoring.RecordingAudio, from, until int64, columns int) []byte {
	if until <= from {
		from, until = 0, clip.DurationMs()
	}
	from, until = max(from, 0), min(until, clip.DurationMs())
	columns = min(max(columns, 1), maxColumns)
	levels := make([]byte, columns)
	span := max(until-from, 1)
	for column := range columns {
		start := from + span*int64(column)/int64(columns)
		end := from + span*int64(column+1)/int64(columns)
		levels[column] = peakLevel(clip.PeakBetween(start, max(end-1, start)))
	}

	return levels
}

// peakLevel turns a peak into 0 for silenceDB or quieter, up to 255 for 0 dB.
func peakLevel(peak float64) byte {
	if peak <= 0 {
		return 0
	}
	db := amplitudeDB * math.Log10(peak)
	level := math.Round((db - silenceDB) / -silenceDB * peakLevels)

	return byte(min(max(level, 0), peakLevels))
}

// playbackWindow is a part of a recording as berad-audio writes it.
func playbackWindow(window *protocol.AudioWindow) *authoring.PlaybackWindow {
	if window == nil {
		return nil
	}

	return &authoring.PlaybackWindow{StartMs: window.StartMs, EndMs: window.EndMs}
}

// check decodes every recording with a file and lists those that cannot play.
func check(ctx context.Context, files audioFiles) (protocol.AudioCheck, error) {
	library, err := openForReading(files)
	if err != nil {
		return protocol.AudioCheck{Broken: nil}, err
	}
	limited, cancel := context.WithTimeout(ctx, analysisTimeout)
	defer cancel()
	var broken []protocol.AudioProblem
	for _, id := range library.RecordingIDs() {
		if library.Recording(id).File == nil {
			continue
		}
		if found := library.InspectRecording(limited, id); found.State != authoring.RecordingReady {
			broken = append(broken, protocol.AudioProblem{Recording: id, Reason: found.Reason})
		}
	}

	return protocol.AudioCheck{Broken: broken}, nil
}

// loudness measures every recording with a file: how loud its region is, its
// loudest sample and the target it is evened out to. A measurement is kept
// until the file, region or policy changes, so asking again is quick.
func (a *audioAnalyses) loudness(ctx context.Context, files audioFiles) (protocol.AudioLoudness, error) {
	library, err := openForReading(files)
	if err != nil {
		return protocol.AudioLoudness{Recordings: nil}, err
	}
	limited, cancel := context.WithTimeout(ctx, analysisTimeout)
	defer cancel()
	var measured []protocol.RecordingLoudness
	for _, id := range library.RecordingIDs() {
		recording := library.Recording(id)
		if recording.File == nil {
			continue
		}
		measured = append(measured, a.measure(limited, library, id, recording))
		if err := limited.Err(); err != nil {
			return protocol.AudioLoudness{Recordings: nil}, fmt.Errorf("measure the recordings: %w", err)
		}
	}

	return protocol.AudioLoudness{Recordings: measured}, nil
}

// measure measures one recording, or says why it cannot be.
func (a *audioAnalyses) measure(
	ctx context.Context, library *authoring.LibraryEditor, recordingID string, recording authoring.Recording,
) protocol.RecordingLoudness {
	result := protocol.RecordingLoudness{ID: recordingID, MeasuredDB: 0, PeakDB: 0, TargetDB: 0, Problem: ""}
	path, err := library.RecordingPath(recordingID)
	if err != nil {
		result.Problem = errNoRecordingFile.Error()

		return result
	}
	options, err := library.CalibrationOptionsFor(recordingID)
	if err != nil {
		result.Problem = authoringError(err).Error()

		return result
	}
	info, err := os.Stat(path)
	if err != nil {
		result.Problem = "The file is missing."

		return result
	}
	key := fmt.Sprintf("%s|%d-%d|%v|%s|%v", path, info.Size(), info.ModTime().UnixNano(), recording.PlaybackWindow,
		options.Method, options.TargetDB)
	a.mu.Lock()
	kept, found := a.measured[key]
	a.mu.Unlock()
	if found {
		kept.ID = recordingID

		return kept
	}
	clip, err := authoring.LoadRecordingContext(ctx, path, ebitengine.Decode)
	if err != nil {
		result.Problem = "The file could not be decoded: " + authoringError(err).Error()

		return result
	}
	suggestion, err := clip.SuggestCalibration(recording.PlaybackWindow, options)
	if err != nil {
		result.Problem = "It is silent, so there is nothing to even out."
	} else {
		result.MeasuredDB, result.PeakDB, result.TargetDB = suggestion.MeasuredDB, suggestion.PeakDB, suggestion.TargetDB
	}
	a.mu.Lock()
	if len(a.measured) >= measuredKept {
		clear(a.measured)
	}
	a.measured[key] = result
	a.mu.Unlock()

	return result
}
