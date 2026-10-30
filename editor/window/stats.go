package window

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// statsEnv names a file the editor window appends a line of frame
// statistics to as it closes, for measuring (increment 10): how long its ticks
// and draws took, its frame rates, and its views' command-to-event times.
const statsEnv = "BERAD_FRAME_STATS"

// statsFile is the statistics file's permissions.
const statsFile = 0o600

// The shares of frames the median, p95 and largest figures cover.
const (
	median       = 0.5
	percentile95 = 0.95
	largest      = 1
)

// frameStats collects a window's frame times while it runs.
type frameStats struct {
	path      string
	view      string
	ticks     []time.Duration
	draws     []time.Duration
	latencies []time.Duration
	tps, fps  float64
}

// newFrameStats returns statistics for a window when they are asked for.
func newFrameStats(view string) *frameStats {
	path := os.Getenv(statsEnv)
	if path == "" {
		return nil
	}

	return &frameStats{path: path, view: view, ticks: nil, draws: nil, latencies: nil, tps: 0, fps: 0}
}

// timedGame times a game's ticks and draws.
type timedGame struct {
	game  ebiten.Game
	stats *frameStats
}

// timed wraps a game to time it when statistics are asked for.
//
//nolint:ireturn // It returns the game as it was when no statistics are asked for.
func timed(game ebiten.Game, stats *frameStats) ebiten.Game {
	if stats == nil {
		return game
	}

	return &timedGame{game: game, stats: stats}
}

// Update times one tick.
func (g *timedGame) Update() error {
	start := time.Now()
	err := g.game.Update()
	g.stats.ticks = append(g.stats.ticks, time.Since(start))
	g.stats.tps, g.stats.fps = ebiten.ActualTPS(), ebiten.ActualFPS()

	return err //nolint:wrapcheck // The game's own errors, ebiten.Termination among them, pass through.
}

// Draw times one draw.
func (g *timedGame) Draw(screen *ebiten.Image) {
	start := time.Now()
	g.game.Draw(screen)
	g.stats.draws = append(g.stats.draws, time.Since(start))
}

// Layout passes the window's size on.
func (g *timedGame) Layout(outsideWidth, outsideHeight int) (int, int) {
	return g.game.Layout(outsideWidth, outsideHeight)
}

// addLatencies keeps a view's command-to-event times.
func (f *frameStats) addLatencies(latencies []time.Duration) {
	if f != nil {
		f.latencies = append(f.latencies, latencies...)
	}
}

// statsLine is one window's statistics, in milliseconds.
type statsLine struct {
	View          string  `json:"view"`
	Ticks         int     `json:"ticks"`
	TPS           float64 `json:"tps"`
	FPS           float64 `json:"fps"`
	TickMedian    float64 `json:"tickMedianMs"`
	TickP95       float64 `json:"tickP95Ms"`
	TickMax       float64 `json:"tickMaxMs"`
	DrawMedian    float64 `json:"drawMedianMs"`
	DrawP95       float64 `json:"drawP95Ms"`
	DrawMax       float64 `json:"drawMaxMs"`
	Commands      int     `json:"commands"`
	LatencyMedian float64 `json:"latencyMedianMs"`
	LatencyMax    float64 `json:"latencyMaxMs"`
}

// write appends the window's statistics to the statistics file.
func (f *frameStats) write() error {
	if f == nil {
		return nil
	}
	line := statsLine{
		View: f.view, Ticks: len(f.ticks), TPS: f.tps, FPS: f.fps,
		TickMedian: quantile(f.ticks, median), TickP95: quantile(f.ticks, percentile95), TickMax: quantile(f.ticks, largest),
		DrawMedian: quantile(f.draws, median), DrawP95: quantile(f.draws, percentile95), DrawMax: quantile(f.draws, largest),
		Commands: len(f.latencies), LatencyMedian: quantile(f.latencies, median), LatencyMax: quantile(f.latencies, largest),
	}
	encoded, err := json.Marshal(line)
	if err != nil {
		return fmt.Errorf("encode frame statistics: %w", err)
	}
	file, err := os.OpenFile(f.path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, statsFile)
	if err != nil {
		return fmt.Errorf("open frame statistics: %w", err)
	}
	_, writeErr := file.Write(append(encoded, '\n'))
	if err := file.Close(); writeErr == nil && err != nil {
		writeErr = err
	}
	if writeErr != nil {
		return fmt.Errorf("write frame statistics: %w", writeErr)
	}

	return nil
}

// quantile returns a share of the way through durations, in milliseconds.
func quantile(durations []time.Duration, share float64) float64 {
	if len(durations) == 0 {
		return 0
	}
	sorted := slices.Clone(durations)
	slices.Sort(sorted)
	index := min(int(share*float64(len(sorted))), len(sorted)-1)

	return float64(sorted[index].Microseconds()) / float64(time.Millisecond/time.Microsecond)
}
