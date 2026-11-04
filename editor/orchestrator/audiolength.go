package orchestrator

import (
	"io"
	"sync"

	"github.com/bradhannah/berad-sdk/modules/berad-audio/authoring"
	"github.com/bradhannah/berad-sdk/modules/berad-audio/backend/ebitengine"

	"github.com/bradhannah/berad-engine/editor/protocol"
)

// Decoded sound is 16-bit stereo: four bytes a frame.
const bytesPerFrame = 4

// recordingLengths keeps each recording file's length, in milliseconds, by
// its path and version, so a file is decoded for it once while it is
// unchanged.
type recordingLengths struct {
	mu    sync.Mutex
	known map[string]int64
}

func newRecordingLengths() *recordingLengths {
	return &recordingLengths{mu: sync.Mutex{}, known: map[string]int64{}}
}

// lengthOf is how long the part of a ready recording that plays lasts, in
// milliseconds: its region, or the whole file, decoded to find out; zero
// when it cannot be decoded.
func (a *projectAudio) lengthOf(file string, sent *protocol.AudioRecording) int64 {
	if sent.EndMs > sent.StartMs {
		return sent.EndMs - sent.StartMs
	}
	key := file + "|" + sent.Version
	a.lengths.mu.Lock()
	known, found := a.lengths.known[key]
	a.lengths.mu.Unlock()
	if found {
		return known
	}
	measured := a.decodeLength(file)
	a.lengths.mu.Lock()
	a.lengths.known[key] = measured
	a.lengths.mu.Unlock()

	return measured
}

// decodeLength decodes a file's header far enough to know its length.
func (a *projectAudio) decodeLength(file string) int64 {
	data, err := a.root.ReadFile(file)
	if err != nil {
		return 0
	}
	stream, err := ebitengine.Decode(data, file, authoring.EditorSampleRate)
	if err != nil {
		return 0
	}
	if closer, ok := stream.(io.Closer); ok {
		defer closer.Close() //nolint:errcheck // Nothing more is read from it.
	}
	const millisecond = 1000

	return stream.Length() * millisecond / (bytesPerFrame * authoring.EditorSampleRate)
}
