package audioview_test

import (
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
)

// classifiedLibrary is the test library with profiles, families, a preset
// for new ambient cues, and three ambient cues, the river heard from 8
// tiles away.
func classifiedLibrary() protocol.AudioLibrary {
	library := testLibrary()
	river := &library.Cues[0]
	river.Ambient = &protocol.AmbientSettings{RadiusTiles: 8, FadeInMs: 800, FadeOutMs: 1200}
	river.Profile, river.Recordings[0].Family = "bed", "water"
	library.Cues = append(library.Cues,
		cue("bed/wind", "Wind", protocol.CategoryAmbient, recording("wind", true)),
		cue("bed/fire", "Fire", protocol.CategoryAmbient, recording("fire", true)))
	profile := func(name, family string) protocol.AudioProfile {
		return protocol.AudioProfile{Name: name, Family: family, Phase: "loop", Outcome: "neutral", MixRole: "bed",
			Emitters: []string{name}, Contexts: []string{"outdoors"}, SearchTerms: nil}
	}
	library.Profiles = []protocol.AudioProfile{profile("bed", "water"), profile("door", "wood")}
	library.Families = []string{"water", "wood"}
	library.AmbientPreset = &protocol.AmbientPreset{CuePrefix: "bed/", Profile: "bed", Family: "water",
		HowToHear: "Stand near water.", Gain: 0.6}

	return library
}

// TestAmbientDistance changes how far away the river is heard, refuses a
// fade too long, and keeps the other settings as saved.
func TestAmbientDistance(t *testing.T) {
	t.Parallel()
	f := newFixtureWith(t, classifiedLibrary())
	f.click("cue:bed/river")
	f.typeInto("radius", "12.5")
	f.keys("Enter")
	var change protocol.AmbientChange
	if err := f.edit(protocol.AudioSetAmbient).Decode(&change); err != nil || change.Cue != "bed/river" ||
		change.Ambient != (protocol.AmbientSettings{RadiusTiles: 12.5, FadeInMs: 800, FadeOutMs: 1200}) {
		t.Fatalf("the distance, with the fades as saved: %+v %v", change, err)
	}
	f.requests.AudioEdits = nil
	f.typeInto("fade-out", "61 s")
	f.keys("Enter")
	if len(f.requests.AudioOf(protocol.AudioSetAmbient)) != 0 {
		t.Fatal("a fade over a minute is refused in the window")
	}
	f.click("ui:add-recording")
	if f.ctx.Files.(*frametest.Files).Asked != 0 {
		t.Fatal("an ambient cue heard from a distance takes no more recordings")
	}
}

// TestAmbientCueWithoutDistance gives an ambient cue the distance a new one
// starts with, and offers none to one that plays recordings in turn.
func TestAmbientCueWithoutDistance(t *testing.T) {
	t.Parallel()
	library := testLibrary()
	library.Cues[1].Category = protocol.CategoryAmbient
	f := newFixtureWith(t, library)
	f.click("cue:bed/river")
	if f.has("ui:radius") {
		t.Fatal("no fields until it has a distance")
	}
	f.click("ui:set-distance")
	var change protocol.AmbientChange
	if err := f.edit(protocol.AudioSetAmbient).Decode(&change); err != nil ||
		change.Ambient != (protocol.AmbientSettings{RadiusTiles: 8, FadeInMs: 1000, FadeOutMs: 1000}) {
		t.Fatalf("the distance a new ambient cue has: %+v %v", change, err)
	}
	f.click("cue:sfx/door")
	if f.has("ui:radius") || f.has("ui:set-distance") {
		t.Fatal("a cue playing recordings in turn has no distance")
	}
}

// TestProfileAndFamily chooses a cue's profile and a recording's family
// from their menus, and names a new family.
func TestProfileAndFamily(t *testing.T) {
	t.Parallel()
	f := newFixtureWith(t, classifiedLibrary())
	f.click("cue:bed/river")
	f.click("ui:profile")
	f.click("ui:profile:door")
	var profile protocol.ProfileChange
	if err := f.edit(protocol.AudioSetProfile).Decode(&profile); err != nil ||
		profile != (protocol.ProfileChange{Cue: "bed/river", Profile: "door"}) {
		t.Fatalf("the chosen profile: %+v %v", profile, err)
	}
	f.click("ui:tab-recordings")
	f.click("rec:river")
	f.click("ui:family")
	f.click("ui:family:wood")
	f.click("ui:family")
	f.click("ui:family-new")
	f.typeInto("family-name", "Big Room")
	f.click("ui:set-family")
	f.typeInto("family-name", "stone")
	f.click("ui:set-family")
	families := f.requests.AudioOf(protocol.AudioSetFamily)
	var first, second protocol.FamilyChange
	if len(families) != 2 || families[0].Decode(&first) != nil || families[1].Decode(&second) != nil ||
		first.Family != "wood" || second != (protocol.FamilyChange{Recording: "river", Family: "stone"}) {
		t.Fatalf("a family chosen, then a new one named: %+v %+v", first, second)
	}
}

// TestSoundscapeLayers fills the layers from cues, asks which ambient layer
// a third ambient cue replaces, mutes, solos and removes layers, and sets
// the monitor.
func TestSoundscapeLayers(t *testing.T) {
	t.Parallel()
	f := newFixtureWith(t, classifiedLibrary())
	scape := f.ctx.Soundscape.(*frametest.Soundscape)
	for _, id := range []string{"sfx/door", "bed/river", "bed/wind", "theme/title"} {
		f.click("cue:" + id)
		f.click("ui:add-layer")
	}
	if scape.Layers["sfx"].Cue != "sfx/door" || scape.Layers["sfx"].Rest != 500*time.Millisecond ||
		!scape.Layers["ambient-1"].Loop || scape.Layers["ambient-2"].Cue != "bed/wind" ||
		scape.Layers["music"].Cue != "theme/title" {
		t.Fatalf("each cue in its layer: %+v", scape.Layers)
	}
	f.click("cue:bed/fire")
	f.click("ui:add-layer")
	f.click("ui:replace:ambient-2")
	if scape.Layers["ambient-2"].Cue != "bed/fire" || !f.has("state:layer=ambient-1:bed/river") {
		t.Fatalf("the chosen ambient layer is replaced: %+v", scape.Layers)
	}
	f.click("ui:layer-mute-ambient-1")
	f.click("ui:layer-solo-music")
	f.click("ui:layer-again-music")
	if scape.Layers["ambient-1"].Gain != 0 || scape.Layers["sfx"].Gain != 0 || scape.Layers["music"].Gain != 1 ||
		!scape.Layers["music"].Again {
		t.Fatalf("muted, soloed and looped: %+v", scape.Layers)
	}
	f.click("ui:layer-remove-sfx")
	f.click("ui:monitor-quieter-sfx")
	f.click("ui:reference")
	if _, playing := scape.Layers["sfx"]; playing || scape.Monitor.SFX != 0.6 || scape.Monitor.Music != 0.7 ||
		!scape.Monitor.Reference {
		t.Fatalf("removed, and the monitor set: %+v %+v", scape.Layers, scape.Monitor)
	}
	f.click("ui:stop-all")
	if len(scape.Layers) != 0 || !f.has("state:layer=music:theme/title") {
		t.Fatalf("Stop all stops the soundscape and keeps its layers: %+v", scape.Layers)
	}
}

// TestNewAmbientCueUsesThePreset suggests a new ambient cue's ID from the
// audio project's preset.
func TestNewAmbientCueUsesThePreset(t *testing.T) {
	t.Parallel()
	f := newFixtureWith(t, classifiedLibrary())
	f.click("ui:new-cue")
	f.click("ui:dialog-ambient")
	f.typeInto("cue-name", "Rain")
	f.click("ui:dialog-ok")
	var made protocol.NewCue
	if err := f.edit(protocol.AudioCreateCue).Decode(&made); err != nil || made.ID != "bed/rain" ||
		made.Category != protocol.CategoryAmbient {
		t.Fatalf("an ID from the preset: %+v %v", made, err)
	}
}
