package window

import (
	"os"

	"github.com/ncruces/zenity"
)

// testSoundEnv names the file a real-window test chooses whenever a window
// asks for a sound, instead of the system's Open dialog.
const testSoundEnv = "BERAD_TEST_SOUND_FILE"

// nativeFiles shows the system's Open dialog, off the window's goroutine so
// the window keeps drawing, and posts the answer back to it.
type nativeFiles struct {
	post func(func())
}

// ChooseSound asks the author for a WAV, OGG or MP3 file.
func (f nativeFiles) ChooseSound(done func(string, bool)) {
	if path := os.Getenv(testSoundEnv); path != "" {
		f.post(func() { done(path, true) })

		return
	}
	go func() {
		path, err := zenity.SelectFile(zenity.Title("Add a sound"), zenity.FileFilter{
			Name: "Sounds", Patterns: []string{"*.wav", "*.ogg", "*.mp3"}, CaseFold: true,
		})
		f.post(func() { done(path, err == nil && path != "") })
	}()
}
