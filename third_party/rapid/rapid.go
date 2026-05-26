package rapid

import (
	"math/rand"
	"testing"
	"time"
)

const defaultChecks = 100

// T provides deterministic pseudo-random helpers for lightweight property tests.
type T struct {
	*testing.T
	rnd *rand.Rand
}

// Check runs fn multiple times with fresh deterministic pseudo-random state.
func Check(t *testing.T, fn func(t *T)) {
	t.Helper()

	seed := time.Now().UnixNano()
	for i := 0; i < defaultChecks; i++ {
		t.Run("", func(t *testing.T) {
			fn(&T{
				T:   t,
				rnd: rand.New(rand.NewSource(seed + int64(i))),
			})
		})
	}
}

// IntRange returns a pseudo-random integer in the inclusive range [min, max].
func (t *T) IntRange(min, max int) int {
	t.Helper()
	if max < min {
		t.Fatalf("invalid range: [%d, %d]", min, max)
	}
	if min == max {
		return min
	}
	return min + t.rnd.Intn(max-min+1)
}

// Bool returns a pseudo-random boolean.
func (t *T) Bool() bool {
	t.Helper()
	return t.rnd.Intn(2) == 0
}

// StringN returns a pseudo-random string up to maxLen runes long.
func (t *T) StringN(maxLen int) string {
	t.Helper()
	if maxLen < 0 {
		t.Fatalf("invalid maxLen %d", maxLen)
	}

	alphabet := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 <>/&@._-\n\téöü")
	n := t.IntRange(0, maxLen)
	out := make([]rune, n)
	for i := range out {
		out[i] = alphabet[t.rnd.Intn(len(alphabet))]
	}
	return string(out)
}

// BytesN returns a pseudo-random byte slice up to maxLen bytes long.
func (t *T) BytesN(maxLen int) []byte {
	t.Helper()
	if maxLen < 0 {
		t.Fatalf("invalid maxLen %d", maxLen)
	}

	n := t.IntRange(0, maxLen)
	out := make([]byte, n)
	for i := range out {
		out[i] = byte(t.rnd.Intn(256))
	}
	return out
}

// Sample returns a pseudo-random element from values.
func Sample[V any](rt *T, values ...V) V {
	rt.Helper()
	if len(values) == 0 {
		rt.Fatal("Sample called with no values")
	}
	return values[rt.rnd.Intn(len(values))]
}
