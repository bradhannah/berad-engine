package sound

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	audio "github.com/bradhannah/berad-sdk/modules/berad-audio"

	"github.com/bradhannah/berad-engine/editor/protocol"
)

// audioAsks answers the sound player's questions as the test chooses.
type audioAsks struct {
	libraries  []func(protocol.AudioLibrary, error)
	recordings []recordingAsk
}

type recordingAsk struct {
	recording string
	offset    int64
	done      func(protocol.Recording, error)
}

func (a *audioAsks) AudioLibrary(done func(protocol.AudioLibrary, error)) {
	a.libraries = append(a.libraries, done)
}

func (a *audioAsks) Recording(recording string, offset int64, done func(protocol.Recording, error)) {
	a.recordings = append(a.recordings, recordingAsk{recording: recording, offset: offset, done: done})
}

// nextRecording takes the oldest unanswered recording question.
func (a *audioAsks) nextRecording(t *testing.T) recordingAsk {
	t.Helper()
	if len(a.recordings) == 0 {
		t.Fatal("no recording was asked for")
	}
	ask := a.recordings[0]
	a.recordings = a.recordings[1:]

	return ask
}

// silentDevice plays nothing, so tests make no sound.
type silentDevice struct{ opened int }

func (d *silentDevice) SampleRate() int { return sampleRate }

//nolint:ireturn // The runtime takes any transport.
func (d *silentDevice) NewPlayer(io.ReadSeeker) (audio.Transport, error) {
	return &silentTransport{playing: false, position: 0}, nil
}

type silentTransport struct {
	playing  bool
	position time.Duration
}

func (t *silentTransport) Play() {
	t.playing = true
}
func (t *silentTransport) Pause()               { t.playing = false }
func (t *silentTransport) PauseAndStopReading() { t.playing = false }
func (t *silentTransport) IsPlaying() bool      { return t.playing }
func (t *silentTransport) SetVolume(float64)    {}
func (t *silentTransport) Position() time.Duration {
	return t.position
}

func (t *silentTransport) SetPosition(position time.Duration) error {
	t.position = position

	return nil
}

// silence decodes any bytes as a second of silence.
type silence struct{ *bytes.Reader }

func (s silence) Length() int64 { return s.Size() }

//nolint:ireturn // The runtime takes any PCM stream.
func decodeSilence([]byte, string, int) (audio.PCMStream, error) {
	return silence{Reader: bytes.NewReader(make([]byte, 4*sampleRate))}, nil
}

func newTestPlayer(t *testing.T) (*Player, *audioAsks, *[]string, *silentDevice) {
	t.Helper()
	asks, told, device := &audioAsks{libraries: nil, recordings: nil}, &[]string{}, &silentDevice{opened: 0}
	player := New(asks, func(message string) { *told = append(*told, message) })
	player.decode = decodeSilence
	player.device = func() (audio.Backend, error) {
		device.opened++

		return device, nil
	}
	t.Cleanup(func() { _ = player.Close() })

	return player, asks, told, device
}

func ready(id string, size int64) protocol.AudioRecording {
	return protocol.AudioRecording{
		ID: id, Extension: ".wav", Version: "v1", Size: size, CalibrationDB: 0, StartMs: 0, EndMs: 0,
		Status: protocol.RecordingReady,
	}
}

func testCues() protocol.AudioLibrary {
	return protocol.AudioLibrary{Problem: "", Cues: []protocol.AudioCue{
		{ID: "door/open", Name: "Door opens", Category: protocol.CategorySFX, Gain: 0.8, SearchTerms: nil,
			Recordings: []protocol.AudioRecording{ready("door_a", 6)}},
		{ID: "panel/accept", Name: "Accept", Category: protocol.CategorySFX, Gain: 1, SearchTerms: nil,
			Recordings: []protocol.AudioRecording{{
				ID: "quiet", Extension: "", Version: "", Size: 0, CalibrationDB: 0, StartMs: 0, EndMs: 0,
				Status: protocol.RecordingNoFile,
			}}},
		{ID: "bell/ring", Name: "Bell", Category: protocol.CategorySFX, Gain: 1, SearchTerms: nil,
			Recordings: []protocol.AudioRecording{{
				ID: "bell", Extension: ".wav", Version: "", Size: 0, CalibrationDB: 0, StartMs: 0, EndMs: 0,
				Status: protocol.RecordingMissing,
			}}},
	}}
}

// TestSoundsFetchThenPlay asks for the library, fetches the recording in
// parts, then plays; a held cue plays again without asking, and Stop stops
// it. The device opens only once something plays.
func TestSoundsFetchThenPlay(t *testing.T) {
	t.Parallel()
	player, asks, told, device := newTestPlayer(t)
	player.Play("door/open")
	if len(asks.libraries) != 1 || !player.Playing("door/open") || device.opened != 0 {
		t.Fatalf("waits for the library: %d asks, opened %d", len(asks.libraries), device.opened)
	}
	asks.libraries[0](testCues(), nil)
	first := asks.nextRecording(t)
	first.done(protocol.Recording{ID: "door_a", Offset: 0, Version: "v1", Size: 6, Data: []byte("RIF")}, nil)
	rest := asks.nextRecording(t)
	if rest.offset != 3 {
		t.Fatalf("asks for the rest from 3, not %d", rest.offset)
	}
	rest.done(protocol.Recording{ID: "door_a", Offset: 3, Version: "v1", Size: 6, Data: []byte("F00")}, nil)
	if device.opened != 1 || !player.Playing("door/open") || len(*told) != 0 {
		t.Fatalf("plays once the recording is held: opened %d, told %v", device.opened, *told)
	}
	player.Stop()
	player.Update()
	if player.Playing("door/open") {
		t.Fatal("Stop stops it")
	}
	player.Play("door/open")
	if len(asks.libraries) != 1 || len(asks.recordings) != 0 || !player.Playing("door/open") {
		t.Fatalf("a held cue plays at once: %d library asks, %d recording asks", len(asks.libraries), len(asks.recordings))
	}
}

// TestSoundsThatCannotPlay say once why a cue cannot play, and play nothing
// for a silent slot without a word.
func TestSoundsThatCannotPlay(t *testing.T) {
	t.Parallel()
	player, asks, told, _ := newTestPlayer(t)
	player.Prepare([]string{"bell/ring", "panel/accept", "no/such"})
	asks.libraries[0](testCues(), nil)
	for round := range 2 {
		for _, cue := range []string{"bell/ring", "panel/accept", "no/such"} {
			player.Play(cue)
			if cue == "no/such" {
				// A cue the listing lacks is looked for in a fresh one first.
				asks.libraries[1+round](testCues(), nil)
			}
			if player.Playing(cue) {
				t.Fatalf("%s plays nothing", cue)
			}
		}
	}
	joined := strings.Join(*told, "|")
	if len(*told) != 2 || !strings.Contains(joined, "“bell” is missing") || !strings.Contains(joined, "not in the audio library") {
		t.Fatalf("each problem is told once: %q", *told)
	}
}

// TestSoundsFindANewCue plays a cue added to the library after the last
// listing, once a fresh listing has it.
func TestSoundsFindANewCue(t *testing.T) {
	t.Parallel()
	player, asks, told, _ := newTestPlayer(t)
	player.Prepare(nil)
	asks.libraries[0](protocol.AudioLibrary{Problem: "", Cues: testCues().Cues[1:]}, nil)
	player.Play("door/open")
	if len(asks.libraries) != 2 || !player.Playing("door/open") {
		t.Fatal("a cue the listing lacks waits for a fresh one")
	}
	asks.libraries[1](testCues(), nil)
	asks.nextRecording(t).done(protocol.Recording{ID: "door_a", Offset: 0, Version: "v1", Size: 6, Data: []byte("RIFF00")}, nil)
	if !player.Playing("door/open") || len(*told) != 0 {
		t.Fatalf("the new cue plays: told %v", *told)
	}
}

// TestSoundsRefetchAChangedFile starts again when the file changes while it
// is fetched in parts.
func TestSoundsRefetchAChangedFile(t *testing.T) {
	t.Parallel()
	player, asks, _, _ := newTestPlayer(t)
	player.Prepare([]string{"door/open"})
	asks.libraries[0](testCues(), nil)
	asks.nextRecording(t).done(protocol.Recording{ID: "door_a", Offset: 0, Version: "v1", Size: 6, Data: []byte("RIF")}, nil)
	asks.nextRecording(t).done(protocol.Recording{ID: "door_a", Offset: 3, Version: "v2", Size: 6, Data: []byte("F00")}, nil)
	if again := asks.nextRecording(t); again.offset != 0 {
		t.Fatalf("a changed file is fetched again from the start, not from %d", again.offset)
	}
}

// TestSoundsPlayOneRecordingAndRelist plays one recording on its own, even
// one no cue plays, and lists the library again once it changes.
func TestSoundsPlayOneRecordingAndRelist(t *testing.T) {
	t.Parallel()
	player, asks, told, _ := newTestPlayer(t)
	player.Changed()
	if len(asks.libraries) != 0 {
		t.Fatal("a player that never listed has nothing to refresh")
	}
	player.PlayRecording("spare")
	library := testCues()
	library.Unused = []protocol.AudioRecording{ready("spare", 6)}
	asks.libraries[0](library, nil)
	asks.nextRecording(t).done(protocol.Recording{ID: "spare", Offset: 0, Version: "v1", Size: 6, Data: []byte("RIFF00")}, nil)
	if !player.PlayingRecording("spare") || player.Playing("door/open") || len(*told) != 0 {
		t.Fatalf("the unused recording plays on its own: told %v", *told)
	}
	player.Changed()
	if len(asks.libraries) != 2 {
		t.Fatal("a change lists the library again")
	}
}
