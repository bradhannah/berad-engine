package ui

import (
	"sync"

	etext "github.com/hajimehoshi/ebiten/v2/text/v2"
)

// sizedFaces holds the theme's faces opened at each scale drawn at
// (increment 56).
//
//nolint:gochecknoglobals // A cache shared by every theme's free drawing helpers.
var sizedFaces = struct {
	sync.Mutex

	faces map[sizedFace]*etext.GoTextFace
}{Mutex: sync.Mutex{}, faces: map[sizedFace]*etext.GoTextFace{}}

// sizedFace is a face at a scale.
type sizedFace struct {
	face  *etext.GoTextFace
	scale float64
}

// faceAt returns a face opened at its size times a scale, so its text is
// drawn at the screen's own pixels rather than drawn small and enlarged.
func faceAt(face *etext.GoTextFace, scale float64) *etext.GoTextFace {
	if scale == 1 {
		return face
	}
	sizedFaces.Lock()
	defer sizedFaces.Unlock()
	key := sizedFace{face: face, scale: scale}
	if sized, ok := sizedFaces.faces[key]; ok {
		return sized
	}
	var sized etext.GoTextFace
	sized.Source = face.Source
	sized.Size = face.Size * scale
	sizedFaces.faces[key] = &sized

	return &sized
}
