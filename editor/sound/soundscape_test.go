package sound

import (
	"math"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/protocol"
)

// soundscapeCues adds an ambient cue and a music cue to the test library.
func soundscapeCues() protocol.AudioLibrary {
	library := testCues()
	library.Cues = append(library.Cues,
		protocol.AudioCue{ID: "bed/river", Name: "River", Category: protocol.CategoryAmbient, Gain: 1,
			Recordings: []protocol.AudioRecording{ready("river", 6)}},
		protocol.AudioCue{ID: "music/theme", Name: "Theme", Category: protocol.CategoryMusic, Gain: 0.5,
			Recordings: []protocol.AudioRecording{ready("theme", 6)}})

	return library
}

// fetchAll answers every recording asked for with its whole file.
func fetchAll(t *testing.T, asks *audioAsks) {
	t.Helper()
	for len(asks.recordings) > 0 {
		ask := asks.nextRecording(t)
		ask.done(protocol.Recording{ID: ask.recording, Offset: 0, Version: "v1", Size: 6, Data: []byte("RIFF00")}, nil)
	}
}

func layerGain(t *testing.T, player *Player, slot string) float64 {
	t.Helper()
	snapshot := player.layers[slot].runtime.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("one voice plays %s: %+v", slot, snapshot)
	}

	return snapshot[0].TargetGain
}

func near(got, want float64) bool { return math.Abs(got-want) < 1e-9 }

// TestSoundscapeLayersPlayTogether plays a bed and an SFX that repeats,
// changes a layer's gain while it plays, keeps them through an audition,
// and stops one.
func TestSoundscapeLayersPlayTogether(t *testing.T) {
	t.Parallel()
	player, asks, told, _ := newTestPlayer(t)
	clock := time.Unix(0, 0)
	player.now = func() time.Time { return clock }
	player.PlayLayer(Layer{Slot: "ambient-1", Cue: "bed/river", Gain: 1, Loop: true, Again: false, Rest: 0})
	player.PlayLayer(Layer{Slot: "sfx", Cue: "door/open", Gain: 1, Loop: false, Again: true,
		Rest: 500 * time.Millisecond})
	if !player.LayerPlaying("ambient-1") {
		t.Fatal("a layer waiting for its recording counts as playing")
	}
	asks.libraries[0](soundscapeCues(), nil)
	fetchAll(t, asks)
	if !player.LayerPlaying("ambient-1") || !player.LayerPlaying("sfx") || len(*told) != 0 {
		t.Fatalf("both play: %v", *told)
	}
	voice := player.layers["ambient-1"].voice
	player.SetLayer(Layer{Slot: "ambient-1", Cue: "bed/river", Gain: 0.5, Loop: true, Again: false, Rest: 0})
	if player.layers["ambient-1"].voice != voice || !near(layerGain(t, player, "ambient-1"), 0.5) {
		t.Fatal("a new gain applies without starting again")
	}

	_ = player.layers["sfx"].voice.Stop(0)
	player.Update()
	if !player.LayerPlaying("sfx") {
		t.Fatal("an SFX that plays again counts as playing while it rests")
	}
	clock = clock.Add(600 * time.Millisecond)
	player.Update()
	if !sounding(player.layers["sfx"].voice.Status().State) {
		t.Fatal("it plays again after its rest")
	}

	player.Audition("door_a", nil, 0, false)
	player.Stop()
	if !player.LayerPlaying("ambient-1") {
		t.Fatal("an audition, and stopping it, leave the soundscape")
	}
	player.StopLayer("ambient-1")
	if player.LayerPlaying("ambient-1") || !player.LayerPlaying("sfx") {
		t.Fatal("stopping one layer leaves the other")
	}
}

// TestMonitorLevels plays each category at its level, a recording alone at
// the level of the cues that play it, and with Reference at its calibrated
// level alone.
func TestMonitorLevels(t *testing.T) {
	t.Parallel()
	player, asks, _, _ := newTestPlayer(t)
	player.SetMonitor(Monitor{Music: 0.5, SFX: 0.25, Ambient: 0.75, Reference: false})
	player.Prepare([]string{"door/open"})
	asks.libraries[0](soundscapeCues(), nil)
	fetchAll(t, asks)
	player.Play("door/open")
	snapshot := player.current.runtime.Snapshot()
	if len(snapshot) != 1 || !near(snapshot[0].RequestGain, 0.25) {
		t.Fatalf("SFX at its level: %+v", snapshot)
	}
	if !near(player.monitorGain(recordingCue("river")), 0.75) || !near(player.monitorGain("music/theme"), 0.5) {
		t.Fatal("a recording alone plays at its cues' category level")
	}
	player.PlayLayer(Layer{Slot: "music", Cue: "music/theme", Gain: 0.8, Loop: false, Again: false, Rest: 0})
	fetchAll(t, asks)
	if !near(layerGain(t, player, "music"), 0.5*0.8*0.5) {
		t.Fatalf("cue gain × layer gain × level: %v", layerGain(t, player, "music"))
	}
	player.SetMonitor(Monitor{Music: 0.5, SFX: 0.25, Ambient: 0.75, Reference: true})
	if !near(layerGain(t, player, "music"), 0.8) || !player.stale(player.current) {
		t.Fatalf("Reference drops the cue's gain and the level: %v", layerGain(t, player, "music"))
	}
}
