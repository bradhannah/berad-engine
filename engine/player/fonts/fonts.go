// Package fonts holds the fonts what a game shows is written in: IBM Plex
// Sans (SIL Open Font License; see OFL.txt), unmodified. The editor draws its
// own text in them too, so it takes them from here.
package fonts

import _ "embed" // The fonts are embedded files.

var (
	//go:embed IBMPlexSans-Regular.ttf
	sansRegular []byte
	//go:embed IBMPlexSans-SemiBold.ttf
	sansSemiBold []byte
	//go:embed IBMPlexSans-Italic.ttf
	sansItalic []byte
	//go:embed IBMPlexSans-SemiBoldItalic.ttf
	sansSemiBoldItalic []byte
)

// SansRegular is IBM Plex Sans Regular, as a TrueType file.
func SansRegular() []byte { return sansRegular }

// SansSemiBold is IBM Plex Sans SemiBold, as a TrueType file.
func SansSemiBold() []byte { return sansSemiBold }

// SansItalic is IBM Plex Sans Italic, as a TrueType file.
func SansItalic() []byte { return sansItalic }

// SansSemiBoldItalic is IBM Plex Sans SemiBold Italic, as a TrueType file.
func SansSemiBoldItalic() []byte { return sansSemiBoldItalic }
