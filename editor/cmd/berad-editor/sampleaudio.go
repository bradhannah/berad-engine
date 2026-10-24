package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
)

// Generated sample tones: mono 16-bit PCM at a low rate, kept quiet.
const (
	toneRate      = 22050
	toneLevel     = 0.25
	toneFade      = 0.02
	wavHeader     = 44
	riffPrefix    = 8
	wavFormatSize = 16
	wavPCM        = 1
	bitsPerSample = 16
	bytesPerFrame = bitsPerSample / 8
	fullScale     = math.MaxInt16
	decayRate     = 6.0
	// turn is a full turn in radians; voices mixes a hum and its overtone.
	turn   = 2 * math.Pi
	voices = 2
)

// Pitches of the sample's notes, in hertz, and their lengths, in seconds.
const (
	pitchA2 = 110.0
	pitchE3 = 164.81
	pitchC5 = 523.25
	pitchE5 = 659.25
	pitchG5 = 783.99
	pitchC6 = 1046.5
	pitchE6 = 1318.51
	pitchA6 = 1760.0

	shortNote = 0.18
	ringNote  = 0.32
	tuneNote  = 0.4
	lastNote  = 0.6
	humLength = 2.0
)

// note is one tone of a sample: its pitch in hertz and how long it lasts in
// seconds; overtone adds a second pitch above it, for a hum.
type note struct {
	pitch, seconds, overtone float64
	decays                   bool
}

// sampleLibrary is a schema-3 audio library with three cues, one of each
// category: a chime (SFX), a hum (ambient, by its routing) and a tune
// (music, by the descriptor). Its tones are generated, not game content.
const sampleLibrary = `{
  "schemaVersion": 3,
  "recordings": {
    "sample_chime": {"file": "sounds/chime.wav", "provenance": {"originalFilename": "chime.wav",
      "sourceType": "generated", "sourceUrl": "unknown", "sourceCreator": "berad-editor sample",
      "license": "MIT", "importedFrom": "unknown", "processing": []}},
    "sample_hum": {"file": "sounds/hum.wav", "provenance": {"originalFilename": "hum.wav",
      "sourceType": "generated", "sourceUrl": "unknown", "sourceCreator": "berad-editor sample",
      "license": "MIT", "importedFrom": "unknown", "processing": []}},
    "sample_tune": {"file": "sounds/tune.wav", "provenance": {"originalFilename": "tune.wav",
      "sourceType": "generated", "sourceUrl": "unknown", "sourceCreator": "berad-editor sample",
      "license": "MIT", "importedFrom": "unknown", "processing": []}}
  },
  "cues": {
    "sample/chime": {"name": "Chime", "recordingId": "sample_chime", "volumeModifier": 0.8,
      "howToHear": "A Play sound in a workflow."},
    "sample/hum": {"name": "Hum", "recordingId": "sample_hum", "volumeModifier": 0.6,
      "howToHear": "A Play sound in a workflow."},
    "sample/tune": {"name": "Tune", "recordingId": "sample_tune", "volumeModifier": 0.7,
      "howToHear": "A Play sound in a workflow."}
  },
  "taxonomy": {
    "recordingFamilies": {"tone": ["sample_chime", "sample_hum", "sample_tune"]},
    "cueProfiles": {"sample": ["sample/chime", "sample/hum", "sample/tune"]},
    "profiles": {"sample": {"targetAcousticFamily": "tone", "phase": "start", "outcome": "neutral",
      "mixRole": "interface", "emitters": ["sample"], "contexts": ["editor"]}}
  },
  "routing": {"ambient": {"sample/hum": {"radiusTiles": 0, "fadeInMs": 200, "fadeOutMs": 400}}}
}
`

// sampleDescriptor names the library and makes the tune music, as the
// terminal audio editor reads it too.
const sampleDescriptor = `{"schemaVersion": 1, "library": "library.json",
  "targets": {"CueCategories": {"sample/tune": "music"}}}
`

// sampleAudio writes the sample project's audio: the library, its
// descriptor and three generated tones, in berad-audio's own formats. It
// writes nothing when the project already has an audio folder.
func sampleAudio(projectDir string) error {
	dir := filepath.Join(projectDir, "audio")
	if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
		return err //nolint:wrapcheck // Nil when the folder is there; otherwise stat's own error.
	}
	tones := map[string][]note{
		"chime.wav": {
			{pitch: pitchE6, seconds: shortNote, overtone: 0, decays: true},
			{pitch: pitchA6, seconds: ringNote, overtone: 0, decays: true},
		},
		"hum.wav": {{pitch: pitchA2, seconds: humLength, overtone: pitchE3, decays: false}},
		"tune.wav": {
			{pitch: pitchC5, seconds: tuneNote, overtone: 0, decays: true},
			{pitch: pitchE5, seconds: tuneNote, overtone: 0, decays: true},
			{pitch: pitchG5, seconds: tuneNote, overtone: 0, decays: true},
			{pitch: pitchC6, seconds: lastNote, overtone: 0, decays: true},
		},
	}
	if err := os.MkdirAll(filepath.Join(dir, "sounds"), dirPerm); err != nil {
		return fmt.Errorf("sample audio: %w", err)
	}
	files := map[string][]byte{"library.json": []byte(sampleLibrary), "project.json": []byte(sampleDescriptor)}
	for name, notes := range tones {
		files[filepath.Join("sounds", name)] = wav(notes)
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, filePerm); err != nil {
			return fmt.Errorf("sample audio: %w", err)
		}
	}

	return nil
}

// wav encodes notes, one after another, as a mono 16-bit WAV file. Each note
// fades in and out briefly so it does not click.
func wav(notes []note) []byte {
	var samples []int16
	var size uint32
	for _, played := range notes {
		count := int(played.seconds * toneRate)
		for index := range count {
			seconds := float64(index) / toneRate
			level := min(1, seconds/toneFade, (played.seconds-seconds)/toneFade)
			if played.decays {
				level *= math.Exp(-decayRate * seconds)
			}
			value := math.Sin(turn * played.pitch * seconds)
			if played.overtone > 0 {
				value = (value + math.Sin(turn*played.overtone*seconds)) / voices
			}
			samples = append(samples, int16(value*level*toneLevel*fullScale))
			size += bytesPerFrame
		}
	}
	var encoded bytes.Buffer
	encoded.Grow(wavHeader + len(samples)*bytesPerFrame)
	encoded.WriteString("RIFF")
	write(&encoded, uint32(wavHeader-riffPrefix)+size)
	encoded.WriteString("WAVEfmt ")
	write(&encoded, uint32(wavFormatSize))
	write(&encoded, uint16(wavPCM))
	write(&encoded, uint16(1))
	write(&encoded, uint32(toneRate))
	write(&encoded, uint32(toneRate*bytesPerFrame))
	write(&encoded, uint16(bytesPerFrame))
	write(&encoded, uint16(bitsPerSample))
	encoded.WriteString("data")
	write(&encoded, size)
	write(&encoded, samples)

	return encoded.Bytes()
}

// write appends a little-endian value; a bytes.Buffer cannot fail to grow.
func write(encoded *bytes.Buffer, value any) {
	if err := binary.Write(encoded, binary.LittleEndian, value); err != nil {
		panic(err)
	}
}
