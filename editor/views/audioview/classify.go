package audioview

import (
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// A cue's classification profile and a recording's acoustic family, each
// chosen from a menu of the library's. A new family can be named.

// Menu item IDs: a profile or family by name, and a new family.
const (
	profileItem   = "profile:"
	familyItem    = "family:"
	newFamilyItem = "family-new"
)

// classifier is the inspector's choice of profile or family, and the menu
// that offers them while open.
type classifier struct {
	profile, family *ui.Button
	menu            ui.Menu
}

func newClassifier() *classifier {
	made := &classifier{
		profile: ui.NewButton("profile", "Profile"), family: ui.NewButton("family", "Family"),
		menu: ui.Menu{Items: nil, Width: 0},
	}
	made.profile.Caret, made.family.Caret = true, true

	return made
}

// profileSummary says what a profile describes: the family it is for, its
// phase, outcome and role in the mix.
func profileSummary(profile protocol.AudioProfile) string {
	return strings.Join([]string{profile.Family, profile.Phase, profile.Outcome, profile.MixRole}, " · ")
}

// profileOf finds a profile by name.
func (s *State) profileOf(name string) (protocol.AudioProfile, bool) {
	if s.library != nil {
		for _, profile := range s.library.Profiles {
			if profile.Name == name {
				return profile, true
			}
		}
	}

	return protocol.AudioProfile{
		Name: name, Family: "", Phase: "", Outcome: "", MixRole: "", Emitters: nil, Contexts: nil, SearchTerms: nil,
	}, false
}

// openProfiles offers the library's profiles for a cue.
func (s *State) openProfiles() {
	pick := s.inspector.classes
	pick.menu.Items = nil
	for _, profile := range s.library.Profiles {
		pick.menu.Items = append(pick.menu.Items, ui.MenuItem{
			ID: profileItem + profile.Name, Label: profile.Name + " — " + profileSummary(profile), Icon: ui.IconNone,
			Disabled: false, Danger: false, Rule: false,
		})
	}
	s.openMenu(pick.profile.Rect)
}

// openFamilies offers the library's families for a recording, and a new one.
func (s *State) openFamilies() {
	pick := s.inspector.classes
	pick.menu.Items = nil
	for _, family := range s.library.Families {
		pick.menu.Items = append(pick.menu.Items, ui.MenuItem{
			ID: familyItem + family, Label: family, Icon: ui.IconNone, Disabled: false, Danger: false, Rule: false,
		})
	}
	pick.menu.Items = append(pick.menu.Items, ui.MenuItem{
		ID: newFamilyItem, Label: "New family…", Icon: ui.IconNone, Disabled: false, Danger: false,
		Rule: len(pick.menu.Items) > 0,
	})
	s.openMenu(pick.family.Rect)
}

// openMenu opens the menu below a button, as wide as its widest choice.
func (s *State) openMenu(anchor image.Rectangle) {
	menu := &s.inspector.classes.menu
	window := image.Rectangle{Min: image.Pt(0, 0), Max: s.size}
	menu.Width = min(ui.MenuWidth(menu.Items, s.theme), window.Dx()-ui.Half*ui.Inset)
	menu.Open(image.Rect(window.Max.X-menu.Width-ui.Inset, anchor.Min.Y, window.Max.X-ui.Inset, anchor.Max.Y), window)
}

// updateClassMenu handles the open menu, which takes all input, and sends
// the choice.
func (s *State) updateClassMenu(input *ui.Input, ctx *frame.Context) {
	chosen := s.inspector.classes.menu.Update(input)
	input.ClearAll()
	switch {
	case chosen == newFamilyItem:
		if recording, ok := s.selectedRecording(); ok {
			s.prompt = s.familyDialog(recording.ID)
		}
	case strings.HasPrefix(chosen, profileItem):
		if cue, ok := s.selectedCue(); ok {
			s.edit(ctx, protocol.AudioSetProfile, protocol.ProfileChange{
				Cue: cue.ID, Profile: strings.TrimPrefix(chosen, profileItem),
			}, nil)
		}
	case strings.HasPrefix(chosen, familyItem):
		s.setFamily(ctx, strings.TrimPrefix(chosen, familyItem))
	}
}

// setFamily puts the selected recording in a family.
func (s *State) setFamily(ctx *frame.Context, family string) {
	if recording, ok := s.selectedRecording(); ok {
		s.edit(ctx, protocol.AudioSetFamily, protocol.FamilyChange{Recording: recording.ID, Family: family}, nil)
	}
}

// familyDialog asks for a new family's name.
func (s *State) familyDialog(recording string) *fieldDialog {
	return newFieldDialog(s.theme, "family-name", "set-family", "New family for “"+recording+"”", []string{
		"An acoustic family groups recordings that sound alike,", "and sets the loudness measuring aims at.",
		"Lowercase letters, digits, _ and -.",
	}, "", func(ctx *frame.Context, typed string) string {
		name := strings.TrimSpace(typed)
		if name == "" || strings.Trim(name, "abcdefghijklmnopqrstuvwxyz0123456789_-") != "" {
			return "Use lowercase letters, digits, _ and -."
		}
		s.setFamily(ctx, name)

		return ""
	})
}

// layoutProfile places a cue's profile choice and returns where it ends.
func (s *State) layoutProfile(cue protocol.AudioCue, left, right, top int) int {
	button := s.inspector.classes.profile
	button.Label = orNone(cue.Profile)
	top += ui.LineHeight
	button.Rect = image.Rect(left, top, right, top+ui.ButtonH)
	button.Disabled = s.library == nil || len(s.library.Profiles) == 0

	return button.Rect.Max.Y + ui.LineHeight + ui.Inset
}

// layoutFamily places a recording's family choice and returns where it
// ends.
func (s *State) layoutFamily(recording protocol.AudioRecording, left, right, top int) int {
	button := s.inspector.classes.family
	button.Label = orNone(recording.Family)
	top += ui.LineHeight
	button.Rect = image.Rect(left, top, right, top+ui.ButtonH)

	return button.Rect.Max.Y + ui.LineHeight + ui.Inset
}

// drawProfile draws a cue's profile choice under its label, and what the
// profile describes.
func (s *State) drawProfile(screen *ebiten.Image, cue protocol.AudioCue) {
	button := s.inspector.classes.profile
	s.theme.DrawLabel(screen, "PROFILE", button.Rect.Min.X, button.Rect.Min.Y-ui.LineHeight+ui.Gap, ui.ColorMuted())
	button.Draw(screen, s.theme)
	summary := "What kind of moment it plays for."
	if profile, found := s.profileOf(cue.Profile); found {
		summary = profileSummary(profile)
	}
	s.theme.DrawText(screen, s.theme.FitText(summary, button.Rect.Dx()), button.Rect.Min.X,
		button.Rect.Max.Y+ui.Gap, ui.ColorMuted())
}

// drawFamily draws a recording's family choice under its label.
func (s *State) drawFamily(screen *ebiten.Image) {
	button := s.inspector.classes.family
	s.theme.DrawLabel(screen, "ACOUSTIC FAMILY", button.Rect.Min.X, button.Rect.Min.Y-ui.LineHeight+ui.Gap,
		ui.ColorMuted())
	button.Draw(screen, s.theme)
	s.theme.DrawText(screen, "A measured recording is measured again.", button.Rect.Min.X,
		button.Rect.Max.Y+ui.Gap, ui.ColorMuted())
}
