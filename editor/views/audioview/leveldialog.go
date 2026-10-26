package audioview

import (
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// The levels berad-audio accepts, in dB.
const (
	minGainDB = -48.0
	maxGainDB = 12.0
)

// levelDialog asks for a recording's level by hand, in dB, for a recording
// measuring gets wrong; it stays when the region or file changes.
func (s *State) levelDialog(recording string, value float64) *fieldDialog {
	title := "Level of “" + recording + "” by hand"
	lines := []string{
		"In dB. It stays when the region or file changes; Use the reference measures it again.",
		"From " + decibels(minGainDB) + " to " + decibels(maxGainDB) + ".",
	}
	submit := func(ctx *frame.Context, typed string) string {
		text := strings.ReplaceAll(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(typed), "dB")), "−", "-")
		level, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil || level < minGainDB || level > maxGainDB {
			return "Type a number of dB from " + decibels(minGainDB) + " to " + decibels(maxGainDB) + "."
		}
		s.edit(ctx, protocol.AudioSetCalibration, protocol.CalibrationChange{
			Recording: recording, GainDB: &level, TargetDB: nil, DefaultTarget: false,
		}, nil)

		return ""
	}

	return newFieldDialog(s.theme, "level", "set-level", title, lines, strconv.FormatFloat(value, 'f', 1, 64), submit)
}
