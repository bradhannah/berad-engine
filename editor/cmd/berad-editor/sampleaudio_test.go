package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bradhannah/berad-sdk/modules/berad-audio/authoring"
	"github.com/bradhannah/berad-sdk/modules/berad-audio/backend/ebitengine"
)

// TestSampleAudioIsALibrary writes a schema-3 library and a version-1
// descriptor that berad-audio reads, with one cue of each category and tones
// that decode, and leaves an audio folder that is already there alone.
func TestSampleAudioIsALibrary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := sampleAudio(dir); err != nil {
		t.Fatal(err)
	}
	descriptorPath := filepath.Join(dir, "audio", "project.json")
	descriptor, err := authoring.ReadProjectDescriptor(descriptorPath)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(descriptor.LibraryPath(descriptorPath))
	if err != nil {
		t.Fatal(err)
	}
	document, err := authoring.ParseDocument(data, nil)
	if err != nil {
		t.Fatal(err)
	}
	for cue, want := range map[string]string{
		"sample/chime": authoring.CategorySFX, "sample/hum": authoring.CategoryAmbient,
		"sample/tune": authoring.CategoryMusic,
	} {
		if got := authoring.CueCategory(descriptor.Targets, document.Routing, cue); got != want {
			t.Fatalf("%s is %s, want %s", cue, got, want)
		}
	}
	for id, recording := range document.Recordings {
		encoded, err := os.ReadFile(filepath.Join(dir, "audio", filepath.FromSlash(*recording.File)))
		if err != nil {
			t.Fatal(err)
		}
		stream, err := ebitengine.Decode(encoded, *recording.File, 48000)
		if err != nil || stream.Length() <= 0 {
			t.Fatalf("%s decodes: %v", id, err)
		}
	}
	if err := os.WriteFile(descriptorPath, []byte("kept"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := sampleAudio(dir); err != nil {
		t.Fatal(err)
	}
	if kept, err := os.ReadFile(descriptorPath); err != nil || string(kept) != "kept" {
		t.Fatal("an audio folder that is already there is left alone")
	}
}
