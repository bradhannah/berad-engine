package ids_test

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bradhannah/berad-engine/engine/ids"
)

func fixedClock(at time.Time) func() time.Time {
	return func() time.Time { return at }
}

func TestStringRoundTrip(t *testing.T) {
	t.Parallel()
	gen := ids.NewGenerator(time.Now, rand.Reader)
	for range 1000 {
		id, err := gen.New()
		if err != nil {
			t.Fatal(err)
		}
		text := id.String()
		if len(text) != 26 {
			t.Fatalf("length %d", len(text))
		}
		parsed, err := ids.Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		if parsed != id {
			t.Fatalf("round trip %s != %s", parsed, id)
		}
	}
}

func TestKnownEncoding(t *testing.T) {
	t.Parallel()
	var max ids.ULID
	for index := range max {
		max[index] = 0xff
	}
	if got := max.String(); got != "7ZZZZZZZZZZZZZZZZZZZZZZZZZ" {
		t.Fatalf("max = %s", got)
	}
	if got := (ids.ULID{}).String(); got != "00000000000000000000000000" {
		t.Fatalf("zero = %s", got)
	}
}

func TestTimeIsEncoded(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 22, 12, 0, 0, 123*int(time.Millisecond), time.UTC)
	gen := ids.NewGenerator(fixedClock(at), rand.Reader)
	id, err := gen.New()
	if err != nil {
		t.Fatal(err)
	}
	if !id.Time().Equal(at) {
		t.Fatalf("time %v != %v", id.Time(), at)
	}
}

func TestMonotonicWithinMillisecond(t *testing.T) {
	t.Parallel()
	gen := ids.NewGenerator(fixedClock(time.UnixMilli(1_700_000_000_000)), rand.Reader)
	previous, err := gen.New()
	if err != nil {
		t.Fatal(err)
	}
	for range 10_000 {
		next, err := gen.New()
		if err != nil {
			t.Fatal(err)
		}
		if next.Compare(previous) <= 0 {
			t.Fatalf("%s not after %s", next, previous)
		}
		if next.String() <= previous.String() {
			t.Fatalf("text order %s <= %s", next, previous)
		}
		previous = next
	}
}

func TestMonotonicWhenClockGoesBackwards(t *testing.T) {
	t.Parallel()
	times := []time.Time{time.UnixMilli(2_000), time.UnixMilli(1_000)}
	call := 0
	gen := ids.NewGenerator(func() time.Time {
		at := times[call]
		call++
		return at
	}, rand.Reader)
	first, err := gen.New()
	if err != nil {
		t.Fatal(err)
	}
	second, err := gen.New()
	if err != nil {
		t.Fatal(err)
	}
	if second.Compare(first) <= 0 {
		t.Fatalf("%s not after %s", second, first)
	}
}

func TestExhaustion(t *testing.T) {
	t.Parallel()
	ones := bytes.NewReader(bytes.Repeat([]byte{0xff}, 10))
	gen := ids.NewGenerator(fixedClock(time.UnixMilli(5)), ones)
	if _, err := gen.New(); err != nil {
		t.Fatal(err)
	}
	if _, err := gen.New(); !errors.Is(err, ids.ErrExhausted) {
		t.Fatalf("err = %v", err)
	}
}

func TestParseRejects(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"", "short", "8ZZZZZZZZZZZZZZZZZZZZZZZZZ", "0000000000000000000000000U", "0000000000000000000000000!"} {
		if _, err := ids.Parse(text); !errors.Is(err, ids.ErrInvalid) {
			t.Fatalf("%q: err = %v", text, err)
		}
	}
}

func TestParseAcceptsLowercase(t *testing.T) {
	t.Parallel()
	id := ids.MustParse("01J9XK4M7QF2ZB8T5N3R6W1VHC")
	lower, err := ids.Parse("01j9xk4m7qf2zb8t5n3r6w1vhc")
	if err != nil {
		t.Fatal(err)
	}
	if lower != id {
		t.Fatal("lowercase differs")
	}
}

func TestJSON(t *testing.T) {
	t.Parallel()
	id := ids.MustParse("01J9XK4M7QF2ZB8T5N3R6W1VHC")
	data, err := json.Marshal(map[string]ids.ULID{"id": id})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"id":"01J9XK4M7QF2ZB8T5N3R6W1VHC"}` {
		t.Fatalf("json = %s", data)
	}
	var decoded map[string]ids.ULID
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["id"] != id {
		t.Fatal("json round trip")
	}
}

func TestShort(t *testing.T) {
	t.Parallel()
	id := ids.MustParse("01J9XK4M7QF2ZB8T5N3R6W1VHC")
	if got := id.Short(6); got != "R6W1VHC"[1:] {
		t.Fatalf("short = %s", got)
	}
}
