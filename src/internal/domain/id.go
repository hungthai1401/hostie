// Package domain contains core domain primitives for hostie.
//
// This file ports the v1 TypeScript ULID generator (src/domain/id.ts)
// to Go using github.com/oklog/ulid/v2. It preserves the v1 monotonic
// guarantee: two IDs generated within the same millisecond sort
// lexicographically by generation order.
package domain

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// entropy is a process-wide monotonic entropy source backed by crypto/rand.
// It MUST be accessed only while holding entropyMu — ulid.MonotonicEntropy
// is not safe for concurrent use.
var (
	entropyMu sync.Mutex
	entropy   = ulid.Monotonic(rand.Reader, 0)
)

// NewID returns a freshly generated 26-character Crockford-base32 ULID.
//
// Properties:
//   - Length is always 26 characters.
//   - IDs are lexicographically sortable by generation time.
//   - Within the same millisecond, IDs are strictly monotonically
//     increasing (the v1 monotonic-factory guarantee).
//   - Entropy is sourced from crypto/rand (never math/rand).
//
// Goroutine safety: NewID is safe to call concurrently from any number
// of goroutines. The underlying *ulid.MonotonicEntropy is serialized by
// an internal sync.Mutex, so callers do not need to provide their own
// synchronization.
func NewID() string {
	entropyMu.Lock()
	defer entropyMu.Unlock()
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}
