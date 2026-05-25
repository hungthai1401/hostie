// Package render produces the on-disk text shape of Hostie's managed block
// inside /etc/hosts. This is the ONE canonical renderer for the Go tree — see
// "One Renderer, One Parser — Share or Pin" in docs/learnings/critical-patterns.
//
// Field separator: a SINGLE space between ip, hostname, aliases, and the
// optional " # <comment>" suffix. This matches v1 src/core/render.ts
// (renderEntry uses parts.join(" ")). /etc/hosts is whitespace-tolerant, so
// space-vs-tab is observationally irrelevant; we pin to space for byte-exact
// parity with the v1 renderer that has shipped to users.
//
// Managed-block shape: BEGIN marker, then for each group an optional
// "# group: <name>" header followed by its entries (enabled rendered raw,
// disabled rendered with a leading "# "), then END marker. There is NO
// blank-line padding inside the markers. This collapses v1's two-shape
// divergence (src/core/render.ts:wrapManagedBlock added blank lines;
// src/core/apply.ts:renderManagedBlock did not) into one canonical shape.
// The v1→v2 padding difference is documented in
// docs/go-migration/release-notes-2.0.0.md.
package render

import (
	"strings"

	"github.com/hungthai1401/hostie/go/internal/domain"
)

// Marker constants for the Hostie-managed region of /etc/hosts.
//
// These strings are matched literally (after trimming surrounding whitespace)
// by the parser in core/etchosts. They must remain byte-identical to v1's
// BEGIN_MARKER / END_MARKER in src/core/apply.ts and src/core/etchosts.ts.
const (
	BeginMarker = "# BEGIN HOSTIE"
	EndMarker   = "# END HOSTIE"
)

// RenderEntry returns a single /etc/hosts line for the given entry.
//
// Enabled:  "<ip> <hostname>[ <alias>...][ # <comment>]"
// Disabled: "# <ip> <hostname>[ <alias>...][ # <comment>]"
//
// Disabled entries are emitted as "#"-commented lines so they remain visible
// (and easily re-enabled by toggling the leading "# ") in /etc/hosts. This
// matches v1 behavior per design.md:108 (hosts-cli-379.71).
func RenderEntry(e domain.Entry) string {
	parts := make([]string, 0, 2+len(e.Aliases))
	parts = append(parts, e.IP, e.Hostname)
	parts = append(parts, e.Aliases...)

	line := strings.Join(parts, " ")
	if e.Comment != "" {
		line += " # " + e.Comment
	}
	if !e.Enabled {
		line = "# " + line
	}
	return line
}

// RenderManagedBlock returns the full managed-block string, including the
// BEGIN and END marker lines, with a trailing newline after the END marker.
//
// Shape (no blank-line padding above BEGIN or below END):
//
//	# BEGIN HOSTIE
//	# group: <name>            (only if the group has at least one entry)
//	<entry-line>
//	...
//	# END HOSTIE
//
// Each line is '\n'-terminated. Empty groups (zero entries) contribute zero
// lines — no header is emitted for them. Both enabled and disabled entries
// are rendered (disabled are "#"-commented by RenderEntry).
func RenderManagedBlock(hf *domain.HostsFile) string {
	var b strings.Builder
	b.WriteString(BeginMarker)
	b.WriteByte('\n')

	if hf != nil {
		for _, g := range hf.Groups {
			if len(g.Entries) == 0 {
				continue
			}
			b.WriteString("# group: ")
			b.WriteString(g.Name)
			b.WriteByte('\n')
			for _, e := range g.Entries {
				b.WriteString(RenderEntry(e))
				b.WriteByte('\n')
			}
		}
	}

	b.WriteString(EndMarker)
	b.WriteByte('\n')
	return b.String()
}

// RenderHostsFile is a thin convenience wrapper around RenderManagedBlock for
// callers (e.g. the golden harness, apply --dry-run) that hold a HostsFile
// value rather than a pointer. The bytes returned are byte-identical to
// RenderManagedBlock(&hf).
func RenderHostsFile(hf domain.HostsFile) string {
	return RenderManagedBlock(&hf)
}
