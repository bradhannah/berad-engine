package orchestrator_test

import (
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/protocol"
)

type clientUpdate = client.Update

const frame = time.Second / 60

// TestStrictModeLatency measures D1/D12: the time from a pane sending a paint
// command to that pane receiving the orchestrator's event, over a pane's real
// connection and including the journal write. It simulates continuous
// painting.
//
// The budget is one frame edit-to-screen; the round trip itself should be well
// under a millisecond. Run with -v to see the numbers.
func TestStrictModeLatency(t *testing.T) {
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	view := h.dial(t, "")
	count := 2000
	if testing.Short() {
		count = 200
	}

	latencies := make([]time.Duration, 0, count)
	for index := range count {
		args := paintArgs(t, tilefloor.Cell{Col: index % 16, Row: (index / 16) % 16, Layer: tilefloor.LayerGround, Tile: "t" + strconv.Itoa(index%3)})
		request, err := view.Send(protocol.Command{Document: h.floor, Edit: tilefloor.PaintEdit, Gesture: "stroke", Args: args})
		if err != nil {
			t.Fatal(err)
		}
		latency := awaitOwnEvent(t, view.Updates(), request)
		latencies = append(latencies, latency)
	}
	slices.Sort(latencies)
	p50 := latencies[len(latencies)/2]
	p95 := latencies[len(latencies)*95/100]
	p99 := latencies[len(latencies)*99/100]
	worst := latencies[len(latencies)-1]
	t.Logf("command-to-event over %d paints: median %v, p95 %v, p99 %v, worst %v (frame budget %v)",
		count, p50, p95, p99, worst, frame)
	if path := os.Getenv("BERAD_LATENCY_REPORT"); path != "" {
		report := "paints=" + strconv.Itoa(count) + " median=" + p50.String() + " p95=" + p95.String() +
			" p99=" + p99.String() + " worst=" + worst.String() + "\n"
		_ = os.WriteFile(path, []byte(report), 0o600)
	}
	if p50 > time.Millisecond {
		t.Errorf("median %v exceeds 1ms", p50)
	}
	if p99 > frame {
		t.Errorf("p99 %v exceeds one frame", p99)
	}
}

func awaitOwnEvent(t *testing.T, updates <-chan clientUpdate, request uint64) time.Duration {
	t.Helper()
	deadline := time.After(5 * time.Second)
	var latency time.Duration
	for {
		select {
		case update := <-updates:
			if update.Event != nil && update.Event.Request == request {
				latency = update.Latency
			}
			if update.Result != nil && update.Request == request {
				if !update.Result.OK {
					t.Fatalf("paint refused: %s", update.Result.Reason)
				}
				return latency
			}
		case <-deadline:
			t.Fatal("no event for paint")
		}
	}
}

func BenchmarkPaintRoundTrip(b *testing.B) {
	dir := b.TempDir()
	createProject(b, dir)
	h := start(b, dir, nil)
	view := h.dial(b, "")
	args := paintArgs(b, tilefloor.Cell{Col: 1, Row: 1, Layer: tilefloor.LayerGround, Tile: "wall"})
	command := protocol.Command{Document: h.floor, Edit: tilefloor.PaintEdit, Gesture: "", Args: args}
	b.ResetTimer()
	for b.Loop() {
		if _, err := view.Command(b.Context(), command); err != nil {
			b.Fatal(err)
		}
	}
}
