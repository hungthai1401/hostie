package etchosts

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestExtractManagedBlock_Cases is the table-driven parity suite with
// v1's src/core/__tests__/etchosts.test.ts, extended to cover the
// malformed-input cases that v1's *strict* extractor in apply.ts threw
// on (collapsed into one seam here per Phase 2 / S4).
func TestExtractManagedBlock_Cases(t *testing.T) {
	type want struct {
		preamble   string
		managed    string
		suffix     string
		errIs      error  // sentinel match via errors.Is
		errContain string // substring match (case-insensitive)
	}
	cases := []struct {
		name  string
		input string
		want  want
	}{
		{
			name: "clean_input_no_markers",
			input: "127.0.0.1 localhost\n" +
				"::1 localhost\n",
			want: want{
				preamble: "127.0.0.1 localhost\n::1 localhost\n",
			},
		},
		{
			name:  "empty_input_no_markers",
			input: "",
			want:  want{preamble: ""},
		},
		{
			name: "input_with_existing_block",
			input: "127.0.0.1 localhost\n" +
				"# BEGIN HOSTIE\n" +
				"192.168.1.10 dev.local\n" +
				"# END HOSTIE\n" +
				"::1 localhost\n",
			want: want{
				preamble: "127.0.0.1 localhost\n",
				managed:  "# BEGIN HOSTIE\n192.168.1.10 dev.local\n# END HOSTIE\n",
				suffix:   "::1 localhost\n",
			},
		},
		{
			name: "input_with_preamble_and_suffix",
			input: "# System hosts\n" +
				"127.0.0.1 localhost\n" +
				"\n" +
				"# BEGIN HOSTIE\n" +
				"192.168.1.10 dev.local\n" +
				"192.168.1.11 staging.local\n" +
				"# END HOSTIE\n" +
				"\n" +
				"# IPv6\n" +
				"::1 localhost\n",
			want: want{
				preamble: "# System hosts\n127.0.0.1 localhost\n\n",
				managed:  "# BEGIN HOSTIE\n192.168.1.10 dev.local\n192.168.1.11 staging.local\n# END HOSTIE\n",
				suffix:   "\n# IPv6\n::1 localhost\n",
			},
		},
		{
			name: "empty_managed_block",
			input: "127.0.0.1 localhost\n" +
				"# BEGIN HOSTIE\n" +
				"# END HOSTIE\n" +
				"::1 localhost\n",
			want: want{
				preamble: "127.0.0.1 localhost\n",
				managed:  "# BEGIN HOSTIE\n# END HOSTIE\n",
				suffix:   "::1 localhost\n",
			},
		},
		{
			name: "marker_with_trailing_whitespace",
			input: "127.0.0.1 localhost\n" +
				"# BEGIN HOSTIE   \n" +
				"192.168.1.10 dev.local\n" +
				"# END HOSTIE\t\n" +
				"::1 localhost\n",
			want: want{
				preamble: "127.0.0.1 localhost\n",
				managed:  "# BEGIN HOSTIE   \n192.168.1.10 dev.local\n# END HOSTIE\t\n",
				suffix:   "::1 localhost\n",
			},
		},
		{
			name: "marker_with_crlf",
			input: "127.0.0.1 localhost\n" +
				"# BEGIN HOSTIE\r\n" +
				"192.168.1.10 dev.local\n" +
				"# END HOSTIE\r\n" +
				"::1 localhost\n",
			want: want{
				preamble: "127.0.0.1 localhost\n",
				managed:  "# BEGIN HOSTIE\r\n192.168.1.10 dev.local\n# END HOSTIE\r\n",
				suffix:   "::1 localhost\n",
			},
		},
		{
			name: "marker_substring_not_matched",
			input: "127.0.0.1 localhost\n" +
				"# BEGIN HOSTIE-foo\n" +
				"192.168.1.10 dev.local\n" +
				"## END HOSTIE\n" +
				"::1 localhost\n",
			want: want{
				preamble: "127.0.0.1 localhost\n# BEGIN HOSTIE-foo\n192.168.1.10 dev.local\n## END HOSTIE\n::1 localhost\n",
			},
		},
		{
			name: "leading_whitespace_marker_not_matched",
			input: "127.0.0.1 localhost\n" +
				"  # BEGIN HOSTIE\n" +
				"192.168.1.10 dev.local\n" +
				"  # END HOSTIE\n",
			want: want{
				preamble: "127.0.0.1 localhost\n  # BEGIN HOSTIE\n192.168.1.10 dev.local\n  # END HOSTIE\n",
			},
		},
		{
			name: "missing_END_marker",
			input: "127.0.0.1 localhost\n" +
				"# BEGIN HOSTIE\n" +
				"192.168.1.10 dev.local\n",
			want: want{errIs: ErrMissingEndMarker, errContain: "END"},
		},
		{
			name: "missing_BEGIN_marker",
			input: "127.0.0.1 localhost\n" +
				"192.168.1.10 dev.local\n" +
				"# END HOSTIE\n",
			want: want{errIs: ErrMissingBeginMarker, errContain: "BEGIN"},
		},
		{
			name: "duplicate_BEGIN",
			input: "# BEGIN HOSTIE\n" +
				"192.168.1.10 dev.local\n" +
				"# BEGIN HOSTIE\n" +
				"# END HOSTIE\n",
			want: want{errIs: ErrDuplicateBeginMarker, errContain: "duplicate BEGIN"},
		},
		{
			name: "duplicate_END",
			input: "# BEGIN HOSTIE\n" +
				"# END HOSTIE\n" +
				"192.168.1.10 stray.local\n" +
				"# END HOSTIE\n",
			want: want{errIs: ErrDuplicateEndMarker, errContain: "duplicate END"},
		},
		{
			name: "END_before_BEGIN",
			input: "127.0.0.1 localhost\n" +
				"# END HOSTIE\n" +
				"# BEGIN HOSTIE\n",
			want: want{errIs: ErrEndBeforeBegin, errContain: "END before BEGIN"},
		},
		{
			name: "no_trailing_newline_on_end_marker",
			input: "127.0.0.1 localhost\n" +
				"# BEGIN HOSTIE\n" +
				"192.168.1.10 dev.local\n" +
				"# END HOSTIE",
			want: want{
				preamble: "127.0.0.1 localhost\n",
				managed:  "# BEGIN HOSTIE\n192.168.1.10 dev.local\n# END HOSTIE",
				suffix:   "",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			preamble, managed, suffix, err := ExtractManagedBlock([]byte(tc.input))

			if tc.want.errIs != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tc.want.errIs)
				}
				if !errors.Is(err, tc.want.errIs) {
					t.Fatalf("expected errors.Is(%v), got %v", tc.want.errIs, err)
				}
				if tc.want.errContain != "" && !strings.Contains(err.Error(), tc.want.errContain) {
					t.Fatalf("expected error containing %q, got %q", tc.want.errContain, err.Error())
				}
				if preamble != nil || managed != nil || suffix != nil {
					t.Fatalf("expected nil slices on error, got preamble=%q managed=%q suffix=%q", preamble, managed, suffix)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := string(preamble); got != tc.want.preamble {
				t.Errorf("preamble mismatch\n got: %q\nwant: %q", got, tc.want.preamble)
			}
			if got := string(managed); got != tc.want.managed {
				t.Errorf("managed mismatch\n got: %q\nwant: %q", got, tc.want.managed)
			}
			if got := string(suffix); got != tc.want.suffix {
				t.Errorf("suffix mismatch\n got: %q\nwant: %q", got, tc.want.suffix)
			}
		})
	}
}

// TestExtractManagedBlock_RoundTrip verifies that Extract → Replace with
// the same managed bytes reproduces the original input byte-for-byte.
func TestExtractManagedBlock_RoundTrip(t *testing.T) {
	original := []byte(
		"# System hosts\n" +
			"127.0.0.1 localhost\n" +
			"\n" +
			"# BEGIN HOSTIE\n" +
			"192.168.1.10 dev.local\n" +
			"# END HOSTIE\n" +
			"\n" +
			"::1 localhost\n",
	)
	_, managed, _, err := ExtractManagedBlock(original)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}
	out, err := ReplaceManagedBlock(original, managed)
	if err != nil {
		t.Fatalf("replace failed: %v", err)
	}
	if !bytes.Equal(out, original) {
		t.Fatalf("round-trip mismatch\n got: %q\nwant: %q", out, original)
	}
}

func TestReplaceManagedBlock(t *testing.T) {
	newBlock := []byte("# BEGIN HOSTIE\n192.168.1.20 new.local\n# END HOSTIE\n")

	cases := []struct {
		name     string
		input    string
		newBlock string
		want     string
		wantErr  error
	}{
		{
			name:     "first_time_append_to_preamble_with_newline",
			input:    "127.0.0.1 localhost\n",
			newBlock: string(newBlock),
			// No blank-line padding: exactly one \n between preamble and BEGIN.
			want: "127.0.0.1 localhost\n" + string(newBlock),
		},
		{
			name:     "first_time_append_to_preamble_without_newline",
			input:    "127.0.0.1 localhost",
			newBlock: string(newBlock),
			want:     "127.0.0.1 localhost\n" + string(newBlock),
		},
		{
			name:     "empty_input_replace",
			input:    "",
			newBlock: string(newBlock),
			want:     string(newBlock),
		},
		{
			name: "replaces_existing_preserving_preamble_and_suffix",
			input: "# System hosts\n" +
				"127.0.0.1 localhost\n" +
				"\n" +
				"# BEGIN HOSTIE\n" +
				"192.168.1.10 old.local\n" +
				"# END HOSTIE\n" +
				"\n" +
				"# IPv6\n" +
				"::1 localhost\n",
			newBlock: string(newBlock),
			want: "# System hosts\n" +
				"127.0.0.1 localhost\n" +
				"\n" +
				"# BEGIN HOSTIE\n" +
				"192.168.1.20 new.local\n" +
				"# END HOSTIE\n" +
				"\n" +
				"# IPv6\n" +
				"::1 localhost\n",
		},
		{
			name:     "malformed_input_propagates_error",
			input:    "# BEGIN HOSTIE\n",
			newBlock: string(newBlock),
			wantErr:  ErrMissingEndMarker,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := ReplaceManagedBlock([]byte(tc.input), []byte(tc.newBlock))
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected errors.Is(%v), got %v", tc.wantErr, err)
				}
				if got != nil {
					t.Fatalf("expected nil output on error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("output mismatch\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

// TestReplaceManagedBlock_PreserveBytes verifies that preamble + suffix
// surrounding an existing block are preserved byte-for-byte (including
// odd whitespace, comments, trailing characters).
func TestReplaceManagedBlock_PreserveBytes(t *testing.T) {
	preamble := "# weird\r\n  indented stuff \t\n127.0.0.1\tlocalhost\n\n\n"
	suffix := "\n\n  # trailing  \n::1 localhost"
	original := preamble +
		"# BEGIN HOSTIE\n" +
		"old\n" +
		"# END HOSTIE\n" +
		suffix
	newBlock := []byte("# BEGIN HOSTIE\nnew line A\nnew line B\n# END HOSTIE\n")
	out, err := ReplaceManagedBlock([]byte(original), newBlock)
	if err != nil {
		t.Fatalf("replace failed: %v", err)
	}
	want := preamble + string(newBlock) + suffix
	if string(out) != want {
		t.Fatalf("byte-for-byte mismatch\n got: %q\nwant: %q", out, want)
	}
}

// TestMarker is a convenience entry point so the bead's verification
// command (`go test -run TestMarker`) catches all marker-related tests.
func TestMarker(t *testing.T) {
	t.Run("extract", TestExtractManagedBlock_Cases)
	t.Run("roundtrip", TestExtractManagedBlock_RoundTrip)
	t.Run("replace", TestReplaceManagedBlock)
	t.Run("preserve_bytes", TestReplaceManagedBlock_PreserveBytes)
}
