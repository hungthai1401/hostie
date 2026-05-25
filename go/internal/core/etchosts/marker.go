// Package etchosts owns reading, parsing, and atomic writing of /etc/hosts
// for the hostie managed block.
//
// This file is the SINGLE seam for managed-block extraction and replacement.
//
// In v1 (TypeScript) there were two parallel extractors:
//
//   - src/core/etchosts.ts: a *lenient* extractor that silently returned an
//     "un-managed" view on malformed input.
//   - src/core/apply.ts:    a *strict* extractor that threw on malformed
//     input (duplicate markers, only-BEGIN, only-END, wrong order) so the
//     apply path could not silently append a duplicate block onto a
//     half-written file (hosts-cli-379.65).
//
// Carrying both into Go would re-create the lenient/strict drift. Per
// Phase 2 / S4 ("Marker extraction is ONE function") and the critical
// pattern "One Renderer, One Parser — Share or Pri", this file collapses
// both into a single extractor:
//
//   - Missing markers entirely        → not an error; the whole input is
//                                       returned as preamble. The file is
//                                       simply not yet hostie-managed.
//   - Malformed marker layout         → error. All callers (apply included)
//                                       fail fast rather than silently
//                                       splicing onto a corrupt file.
//
// The BeginMarker / EndMarker constants are also the strings that
// core/render (Phase 2 / S3) emits when assembling the managed block, so
// "what we write" and "what we parse" are pinned to the same literals here.
package etchosts

import (
	"bytes"
	"errors"
)

// BeginMarker is the literal line that opens the hostie-managed block in
// /etc/hosts. Must match the strings v1 writes (src/core/etchosts.ts and
// src/core/apply.ts both use "# BEGIN HOSTIE") and the strings the render
// package emits when assembling the managed block.
const (
	BeginMarker = "# BEGIN HOSTIE"
	EndMarker   = "# END HOSTIE"
)

// Sentinel errors returned by ExtractManagedBlock on malformed input. All
// are wrapped as plain errors with descriptive messages — callers that
// need to distinguish cases can use errors.Is.
var (
	ErrMissingEndMarker    = errors.New("hostie markers malformed: missing END")
	ErrMissingBeginMarker  = errors.New("hostie markers malformed: missing BEGIN")
	ErrDuplicateBeginMarker = errors.New("hostie markers malformed: duplicate BEGIN")
	ErrDuplicateEndMarker   = errors.New("hostie markers malformed: duplicate END")
	ErrEndBeforeBegin       = errors.New("hostie markers malformed: END before BEGIN")
)

// ExtractManagedBlock splits /etc/hosts content into three byte slices:
//
//   - preamble: everything before the BEGIN HOSTIE line (when present).
//   - managed:  the BEGIN HOSTIE line through the END HOSTIE line, inclusive
//               of the END line's trailing newline (if any).
//   - suffix:   everything after the END HOSTIE line.
//
// The returned slices alias the input — do not mutate them.
//
// Behavior:
//   - No markers present       → preamble = entire input, managed = nil,
//                                suffix = nil, err = nil. The file is
//                                un-managed and ready to receive a block.
//   - Both markers, well-formed → split as described.
//   - Malformed marker layout   → returns a non-nil error and nil slices.
//     Malformed = duplicate BEGIN, duplicate END, only BEGIN, only END,
//     or END appearing before BEGIN.
//
// Marker matching is full-line and exact, after right-trimming trailing
// whitespace (spaces, tabs, carriage returns). Examples:
//
//	"# BEGIN HOSTIE"       → match
//	"# BEGIN HOSTIE  "     → match (trailing whitespace tolerated)
//	"# BEGIN HOSTIE\r"     → match (CRLF tolerated)
//	"## BEGIN HOSTIE"      → NO match (different prefix)
//	"# BEGIN HOSTIE-foo"   → NO match (substring is not enough)
//	"  # BEGIN HOSTIE"     → NO match (leading whitespace is significant)
func ExtractManagedBlock(etcHosts []byte) (preamble, managed, suffix []byte, err error) {
	beginBytes := []byte(BeginMarker)
	endBytes := []byte(EndMarker)

	var beginStart, endLineEndExclusive int = -1, -1
	var beginCount, endCount int

	i := 0
	for i <= len(etcHosts) {
		// Find end of current line (newline or EOF).
		lineStart := i
		j := i
		for j < len(etcHosts) && etcHosts[j] != '\n' {
			j++
		}
		line := etcHosts[lineStart:j]
		trimmed := rtrimASCIIWhitespace(line)

		// Determine where the next line begins (skip past '\n' if present).
		var nextLineStart int
		if j < len(etcHosts) {
			nextLineStart = j + 1
		} else {
			nextLineStart = j
		}

		if bytes.Equal(trimmed, beginBytes) {
			beginCount++
			if beginCount == 1 {
				beginStart = lineStart
			}
		}
		if bytes.Equal(trimmed, endBytes) {
			endCount++
			if endCount == 1 {
				endLineEndExclusive = nextLineStart
			}
		}

		// Advance. If we were at EOF (j == len), break to avoid infinite loop.
		if j == len(etcHosts) {
			break
		}
		i = nextLineStart
	}

	switch {
	case beginCount == 0 && endCount == 0:
		return etcHosts, nil, nil, nil
	case beginCount > 1:
		return nil, nil, nil, ErrDuplicateBeginMarker
	case endCount > 1:
		return nil, nil, nil, ErrDuplicateEndMarker
	case beginCount == 1 && endCount == 0:
		return nil, nil, nil, ErrMissingEndMarker
	case beginCount == 0 && endCount == 1:
		return nil, nil, nil, ErrMissingBeginMarker
	case endLineEndExclusive <= beginStart:
		// We have one of each but END appears before BEGIN.
		return nil, nil, nil, ErrEndBeforeBegin
	}

	preamble = etcHosts[:beginStart]
	managed = etcHosts[beginStart:endLineEndExclusive]
	suffix = etcHosts[endLineEndExclusive:]
	return preamble, managed, suffix, nil
}

// ReplaceManagedBlock returns the /etc/hosts content with the managed
// block replaced by newManaged.
//
// If etcHosts already contains a well-formed managed block, that block is
// swapped out and the surrounding preamble and suffix are preserved
// byte-for-byte.
//
// If etcHosts contains no markers at all, newManaged is appended after
// the existing content. Phase 2 decision (matches the render package):
// no blank-line padding is inserted. Exactly one '\n' is guaranteed
// between the preamble's last byte and the BEGIN line, and only when
// needed:
//
//   - preamble is empty                  → result = newManaged
//   - preamble already ends with '\n'    → result = preamble + newManaged
//   - preamble ends with any other byte  → result = preamble + "\n" + newManaged
//
// Caller is responsible for whether newManaged itself ends with '\n';
// this function is byte-faithful and does not normalize newManaged.
//
// If etcHosts contains malformed markers (see ExtractManagedBlock), the
// error from ExtractManagedBlock is propagated and no output is produced.
func ReplaceManagedBlock(etcHosts []byte, newManaged []byte) ([]byte, error) {
	preamble, managed, suffix, err := ExtractManagedBlock(etcHosts)
	if err != nil {
		return nil, err
	}

	if managed == nil {
		// No existing block — append after preamble with at most one '\n'
		// separator. No blank-line padding.
		if len(preamble) == 0 {
			out := make([]byte, len(newManaged))
			copy(out, newManaged)
			return out, nil
		}
		needsSep := preamble[len(preamble)-1] != '\n'
		size := len(preamble) + len(newManaged)
		if needsSep {
			size++
		}
		out := make([]byte, 0, size)
		out = append(out, preamble...)
		if needsSep {
			out = append(out, '\n')
		}
		out = append(out, newManaged...)
		return out, nil
	}

	// Existing block — swap managed, preserve preamble + suffix exactly.
	out := make([]byte, 0, len(preamble)+len(newManaged)+len(suffix))
	out = append(out, preamble...)
	out = append(out, newManaged...)
	out = append(out, suffix...)
	return out, nil
}

// rtrimASCIIWhitespace returns line with trailing ASCII whitespace
// (space, tab, carriage return, vertical tab, form feed) removed. We
// intentionally do NOT strip leading whitespace — a marker line with
// leading indentation is not a hostie marker.
func rtrimASCIIWhitespace(line []byte) []byte {
	n := len(line)
	for n > 0 {
		c := line[n-1]
		if c == ' ' || c == '\t' || c == '\r' || c == '\v' || c == '\f' {
			n--
			continue
		}
		break
	}
	return line[:n]
}
