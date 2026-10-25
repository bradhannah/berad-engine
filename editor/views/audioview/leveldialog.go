package audioview

import (
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// What a level dialog sets: a recording's level by hand, or the loudness its
// measurement aims at.
type levelKind int

const (
	levelByHand levelKind = iota
	levelTarget
)

// The levels berad-audio accepts, in dB.
const (
	minGainDB        = -48.0
	maxGainDB        = 12.0
	minTargetDB      = -60.0
	maxTargetDB      = -6.0
	levelDialogLines = 3
)

// levelDialog asks for a recording's level or target, in dB; a target can
// also go back to its family's.
func (s *State) levelDialog(kind levelKind, recording string, value float64) *fieldDialog {
	low, high := minGainDB, maxGainDB
	title, explained := "Level of “"+recording+"”", "Set by hand, in dB, it stays when the region or file changes."
	lines := make([]string, 0, levelDialogLines)
	if kind == levelTarget {
		low, high = minTargetDB, maxTargetDB
		title, explained = "Target loudness of “"+recording+"”", "How loud measuring makes it, in dB."
	}
	lines = append(lines, explained, "From "+decibels(low)+" to "+decibels(high)+".")
	if kind == levelTarget {
		lines = append(lines, "Family's uses the target of its acoustic family.")
	}
	submit := func(ctx *frame.Context, typed string) string {
		text := strings.ReplaceAll(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(typed), "dB")), "−", "-")
		level, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil || level < low || level > high {
			return "Type a number of dB from " + decibels(low) + " to " + decibels(high) + "."
		}
		change := protocol.CalibrationChange{Recording: recording, GainDB: &level, TargetDB: nil, DefaultTarget: false}
		if kind == levelTarget {
			change.GainDB, change.TargetDB = nil, &level
		}
		s.edit(ctx, protocol.AudioSetCalibration, change, nil)

		return ""
	}
	dialog := newFieldDialog(s.theme, "level", "set-level", title, lines, strconv.FormatFloat(value, 'f', 1, 64),
		submit)
	if kind == levelTarget {
		dialog.withOther("family-level", "Family's", func(ctx *frame.Context) {
			s.edit(ctx, protocol.AudioSetCalibration, protocol.CalibrationChange{
				Recording: recording, GainDB: nil, TargetDB: nil, DefaultTarget: true,
			}, nil)
		})
	}

	return dialog
}
