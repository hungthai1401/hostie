package domain

import (
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

// Crockford base32 alphabet used by ULID.
const crockfordAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func TestID_Shape(t *testing.T) {
	// Assert NewID has the signature func() string.
	fn := reflect.ValueOf(NewID)
	ft := fn.Type()
	if ft.Kind() != reflect.Func {
		t.Fatalf("NewID is not a function, got %s", ft.Kind())
	}
	if ft.NumIn() != 0 {
		t.Fatalf("NewID should take 0 args, got %d", ft.NumIn())
	}
	if ft.NumOut() != 1 || ft.Out(0).Kind() != reflect.String {
		t.Fatalf("NewID should return a single string, got %v", ft)
	}
}

func TestID_LengthAndAlphabet(t *testing.T) {
	for i := 0; i < 100; i++ {
		id := NewID()
		if len(id) != 26 {
			t.Fatalf("ULID length = %d, want 26 (id=%q)", len(id), id)
		}
		for _, r := range id {
			if !strings.ContainsRune(crockfordAlphabet, r) {
				t.Fatalf("ULID %q contains non-Crockford char %q", id, r)
			}
		}
	}
}

func TestID_Monotonic(t *testing.T) {
	const n = 1000
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		ids[i] = NewID()
	}
	// Strictly non-decreasing (monotonic factory guarantee: strictly increasing
	// within the same millisecond, equal timestamps cannot occur because the
	// entropy is bumped; across milliseconds the timestamp prefix grows).
	if !sort.SliceIsSorted(ids, func(i, j int) bool { return ids[i] < ids[j] }) {
		for i := 1; i < n; i++ {
			if ids[i] < ids[i-1] {
				t.Fatalf("ULID monotonicity broken at index %d: %q < %q", i, ids[i], ids[i-1])
			}
		}
	}
	// Also verify uniqueness across the batch.
	seen := make(map[string]struct{}, n)
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate ULID in sequential batch: %q", id)
		}
		seen[id] = struct{}{}
	}
}

func TestID_ConcurrentUnique(t *testing.T) {
	const goroutines = 50
	const perGoroutine = 100

	var wg sync.WaitGroup
	var ids sync.Map
	var dupCount int64
	var dupMu sync.Mutex

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				id := NewID()
				if _, loaded := ids.LoadOrStore(id, struct{}{}); loaded {
					dupMu.Lock()
					dupCount++
					dupMu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	if dupCount != 0 {
		t.Fatalf("expected zero duplicates across %d goroutines × %d calls, got %d duplicates",
			goroutines, perGoroutine, dupCount)
	}

	// Count entries in sync.Map to confirm total.
	var total int
	ids.Range(func(_, _ any) bool {
		total++
		return true
	})
	want := goroutines * perGoroutine
	if total != want {
		t.Fatalf("unique ID count = %d, want %d", total, want)
	}
}
