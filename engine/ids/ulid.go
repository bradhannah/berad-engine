// Package ids provides ULIDs: permanent, sortable 128-bit identifiers.
//
// A ULID holds a 48-bit millisecond timestamp followed by 80 random bits and is
// written as 26 characters of Crockford base32. IDs from one Generator are
// strictly increasing, even within the same millisecond.
package ids

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// ErrInvalid reports text that is not a well-formed ULID.
var ErrInvalid = errors.New("invalid ULID")

// ErrTime reports a timestamp outside the 48-bit millisecond range.
var ErrTime = errors.New("ULID time out of range")

// ErrExhausted reports that a millisecond's monotonic sequence overflowed.
var ErrExhausted = errors.New("ULID sequence exhausted for this millisecond")

const (
	alphabet     = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	encodedLen   = 26
	byteLen      = 16
	halfLen      = byteLen / 2
	timeBytes    = 6
	randomBytes  = byteLen - timeBytes
	bitsPerChar  = 5
	charMask     = 0x1f
	maxFirstChar = '7'
	maxMillis    = 1<<48 - 1
	byteBits     = 8
	carryShift   = 64 - bitsPerChar
)

// ULID is a 128-bit identifier. The zero value is not a valid generated ID.
type ULID [byteLen]byte

// String returns the 26-character Crockford base32 form.
func (u ULID) String() string {
	high, low := u.halves()
	var out [encodedLen]byte
	for index := encodedLen - 1; index >= 0; index-- {
		out[index] = alphabet[low&charMask]
		low = low>>bitsPerChar | high<<carryShift
		high >>= bitsPerChar
	}

	return string(out[:])
}

// Short returns the last chars characters, for readable file names.
func (u ULID) Short(chars int) string {
	text := u.String()
	if chars <= 0 || chars >= len(text) {
		return text
	}

	return text[len(text)-chars:]
}

// IsZero reports whether u is the zero value.
func (u ULID) IsZero() bool {
	return u == ULID{}
}

// Compare orders ULIDs by creation time, then by their random part.
func (u ULID) Compare(other ULID) int {
	return bytes.Compare(u[:], other[:])
}

// Time returns the millisecond timestamp encoded in u.
func (u ULID) Time() time.Time {
	var millis int64
	for index := range timeBytes {
		millis = millis<<byteBits | int64(u[index])
	}

	return time.UnixMilli(millis).UTC()
}

// MarshalText encodes u as its string form.
func (u ULID) MarshalText() ([]byte, error) {
	return []byte(u.String()), nil
}

// UnmarshalText decodes the string form produced by MarshalText.
func (u *ULID) UnmarshalText(text []byte) error {
	parsed, err := Parse(string(text))
	if err != nil {
		return err
	}
	*u = parsed

	return nil
}

// Parse decodes a 26-character ULID. Lowercase letters are accepted.
func Parse(text string) (ULID, error) {
	if len(text) != encodedLen {
		return ULID{}, fmt.Errorf("%w: %q has length %d", ErrInvalid, text, len(text))
	}
	upper := strings.ToUpper(text)
	if upper[0] > maxFirstChar {
		return ULID{}, fmt.Errorf("%w: %q overflows 128 bits", ErrInvalid, text)
	}
	var high, low uint64
	for index := range encodedLen {
		value := strings.IndexByte(alphabet, upper[index])
		if value < 0 {
			return ULID{}, fmt.Errorf("%w: %q has character %q", ErrInvalid, text, upper[index])
		}
		high = high<<bitsPerChar | low>>carryShift
		low = low<<bitsPerChar | uint64(value)
	}

	return fromHalves(high, low), nil
}

// MustParse is Parse for constants in tests and examples; it panics on error.
func MustParse(text string) ULID {
	parsed, err := Parse(text)
	if err != nil {
		panic(err)
	}

	return parsed
}

func (u ULID) halves() (uint64, uint64) {
	var high, low uint64
	for index := range byteLen / 2 {
		high = high<<byteBits | uint64(u[index])
		low = low<<byteBits | uint64(u[index+halfLen])
	}

	return high, low
}

func fromHalves(high, low uint64) ULID {
	var out ULID
	for index := halfLen - 1; index >= 0; index-- {
		out[index] = byte(high)
		out[index+halfLen] = byte(low)
		high >>= byteBits
		low >>= byteBits
	}

	return out
}

// Generator produces strictly increasing ULIDs. It is safe for concurrent use.
type Generator struct {
	mu     sync.Mutex
	now    func() time.Time
	random io.Reader
	millis uint64
	last   [randomBytes]byte
}

// NewGenerator returns a generator using the supplied clock and randomness,
// typically time.Now and crypto/rand.Reader.
func NewGenerator(now func() time.Time, random io.Reader) *Generator {
	return &Generator{
		mu:     sync.Mutex{},
		now:    now,
		random: random,
		millis: 0,
		last:   [randomBytes]byte{},
	}
}

// New returns the next ULID.
func (g *Generator) New() (ULID, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	unixMillis := g.now().UnixMilli()
	if unixMillis < 0 || unixMillis > maxMillis {
		return ULID{}, fmt.Errorf("%w: %d", ErrTime, unixMillis)
	}
	millis := uint64(unixMillis)

	if millis <= g.millis && g.millis != 0 {
		if !increment(&g.last) {
			return ULID{}, ErrExhausted
		}
	} else {
		g.millis = millis
		if _, err := io.ReadFull(g.random, g.last[:]); err != nil {
			return ULID{}, fmt.Errorf("read ULID randomness: %w", err)
		}
	}

	var out ULID
	stamp := g.millis
	for index := timeBytes - 1; index >= 0; index-- {
		out[index] = byte(stamp)
		stamp >>= byteBits
	}
	copy(out[timeBytes:], g.last[:])

	return out, nil
}

func increment(random *[randomBytes]byte) bool {
	for index := randomBytes - 1; index >= 0; index-- {
		random[index]++
		if random[index] != 0 {
			return true
		}
	}

	return false
}
