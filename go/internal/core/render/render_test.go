package render

import (
	"strings"
	"testing"

	"github.com/hungthai1401/hostie/go/internal/domain"
)

func TestMarkerConstants(t *testing.T) {
	if BeginMarker != "# BEGIN HOSTIE" {
		t.Fatalf("BeginMarker = %q, want %q", BeginMarker, "# BEGIN HOSTIE")
	}
	if EndMarker != "# END HOSTIE" {
		t.Fatalf("EndMarker = %q, want %q", EndMarker, "# END HOSTIE")
	}
}

func TestRenderEntry(t *testing.T) {
	tests := []struct {
		name  string
		entry domain.Entry
		want  string
	}{
		{
			name:  "enabled, no aliases, no comment",
			entry: domain.Entry{IP: "192.168.1.1", Hostname: "example.com", Enabled: true},
			want:  "192.168.1.1 example.com",
		},
		{
			name:  "enabled, single alias, no comment",
			entry: domain.Entry{IP: "192.168.1.1", Hostname: "example.com", Aliases: []string{"ex"}, Enabled: true},
			want:  "192.168.1.1 example.com ex",
		},
		{
			name:  "enabled, multiple aliases, no comment",
			entry: domain.Entry{IP: "192.168.1.1", Hostname: "example.com", Aliases: []string{"ex", "ex.local"}, Enabled: true},
			want:  "192.168.1.1 example.com ex ex.local",
		},
		{
			name:  "enabled, no aliases, with comment",
			entry: domain.Entry{IP: "192.168.1.1", Hostname: "example.com", Enabled: true, Comment: "My server"},
			want:  "192.168.1.1 example.com # My server",
		},
		{
			name:  "enabled, aliases AND comment",
			entry: domain.Entry{IP: "192.168.1.100", Hostname: "devserver.local", Aliases: []string{"devserver"}, Enabled: true, Comment: "Development server"},
			want:  "192.168.1.100 devserver.local devserver # Development server",
		},
		{
			name:  "disabled, no aliases, no comment",
			entry: domain.Entry{IP: "192.168.1.1", Hostname: "example.com", Enabled: false},
			want:  "# 192.168.1.1 example.com",
		},
		{
			name:  "disabled, single alias, no comment",
			entry: domain.Entry{IP: "192.168.1.1", Hostname: "example.com", Aliases: []string{"ex"}, Enabled: false},
			want:  "# 192.168.1.1 example.com ex",
		},
		{
			name:  "disabled, no aliases, with comment",
			entry: domain.Entry{IP: "192.168.1.1", Hostname: "example.com", Enabled: false, Comment: "off"},
			want:  "# 192.168.1.1 example.com # off",
		},
		{
			name:  "disabled, aliases AND comment",
			entry: domain.Entry{IP: "192.168.1.100", Hostname: "devserver.local", Aliases: []string{"devserver"}, Enabled: false, Comment: "Development server"},
			want:  "# 192.168.1.100 devserver.local devserver # Development server",
		},
		{
			name:  "IPv6 short form, no aliases",
			entry: domain.Entry{IP: "::1", Hostname: "localhost", Enabled: true},
			want:  "::1 localhost",
		},
		{
			name:  "IPv6 with aliases and comment",
			entry: domain.Entry{IP: "2001:db8::1", Hostname: "ipv6.example.com", Aliases: []string{"ipv6"}, Enabled: true, Comment: "IPv6 test"},
			want:  "2001:db8::1 ipv6.example.com ipv6 # IPv6 test",
		},
		{
			name:  "long alias list (4+)",
			entry: domain.Entry{IP: "10.0.0.1", Hostname: "test.local", Aliases: []string{"t1", "t2", "t3", "t4"}, Enabled: true, Comment: "Test"},
			want:  "10.0.0.1 test.local t1 t2 t3 t4 # Test",
		},
		{
			name:  "comment with special chars",
			entry: domain.Entry{IP: "192.168.1.1", Hostname: "example.com", Enabled: true, Comment: "Server (production) - do not modify!"},
			want:  "192.168.1.1 example.com # Server (production) - do not modify!",
		},
		{
			name:  "RFC-valid hostname with hyphens and dots",
			entry: domain.Entry{IP: "10.0.0.5", Hostname: "my-host-01.sub.example.co.uk", Aliases: []string{"my-host-01"}, Enabled: true},
			want:  "10.0.0.5 my-host-01.sub.example.co.uk my-host-01",
		},
		{
			name:  "single space between all fields (no double spaces)",
			entry: domain.Entry{IP: "10.0.0.1", Hostname: "test.local", Aliases: []string{"t1", "t2", "t3"}, Enabled: true, Comment: "Test"},
			want:  "10.0.0.1 test.local t1 t2 t3 # Test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderEntry(tt.entry)
			if got != tt.want {
				t.Errorf("RenderEntry() = %q, want %q", got, tt.want)
			}
			if strings.Contains(got, "  ") {
				t.Errorf("RenderEntry() contains double space: %q", got)
			}
		})
	}
}

func TestRenderManagedBlock(t *testing.T) {
	enabled := func(ip, host string, aliases ...string) domain.Entry {
		return domain.Entry{IP: ip, Hostname: host, Aliases: aliases, Enabled: true}
	}
	disabled := func(ip, host string) domain.Entry {
		return domain.Entry{IP: ip, Hostname: host, Enabled: false}
	}

	tests := []struct {
		name string
		hf   *domain.HostsFile
		want string
	}{
		{
			name: "empty HostsFile renders BEGIN+END only with no padding",
			hf:   &domain.HostsFile{Version: 1},
			want: "# BEGIN HOSTIE\n# END HOSTIE\n",
		},
		{
			name: "nil HostsFile renders BEGIN+END only",
			hf:   nil,
			want: "# BEGIN HOSTIE\n# END HOSTIE\n",
		},
		{
			name: "one group, one enabled entry",
			hf: &domain.HostsFile{
				Version: 1,
				Groups: []domain.Group{
					{Name: "dev", Entries: []domain.Entry{enabled("192.168.1.1", "dev.local")}},
				},
			},
			want: "# BEGIN HOSTIE\n# group: dev\n192.168.1.1 dev.local\n# END HOSTIE\n",
		},
		{
			name: "multiple groups",
			hf: &domain.HostsFile{
				Version: 1,
				Groups: []domain.Group{
					{Name: "g1", Entries: []domain.Entry{enabled("192.168.1.1", "one.com")}},
					{Name: "g2", Entries: []domain.Entry{enabled("192.168.1.2", "two.com")}},
				},
			},
			want: "# BEGIN HOSTIE\n# group: g1\n192.168.1.1 one.com\n# group: g2\n192.168.1.2 two.com\n# END HOSTIE\n",
		},
		{
			name: "mix of enabled and disabled in one group",
			hf: &domain.HostsFile{
				Version: 1,
				Groups: []domain.Group{
					{Name: "mix", Entries: []domain.Entry{
						enabled("192.168.1.1", "enabled.com"),
						disabled("192.168.1.2", "disabled.com"),
					}},
				},
			},
			want: "# BEGIN HOSTIE\n# group: mix\n192.168.1.1 enabled.com\n# 192.168.1.2 disabled.com\n# END HOSTIE\n",
		},
		{
			name: "group with description present in struct does not break rendering",
			hf: &domain.HostsFile{
				Version: 1,
				Groups: []domain.Group{
					{Name: "prod", Description: "Production servers", Entries: []domain.Entry{enabled("10.0.0.1", "prod.local")}},
				},
			},
			want: "# BEGIN HOSTIE\n# group: prod\n10.0.0.1 prod.local\n# END HOSTIE\n",
		},
		{
			name: "empty group contributes no lines (no header, no blank)",
			hf: &domain.HostsFile{
				Version: 1,
				Groups: []domain.Group{
					{Name: "empty", Entries: nil},
					{Name: "dev", Entries: []domain.Entry{enabled("192.168.1.1", "dev.local")}},
				},
			},
			want: "# BEGIN HOSTIE\n# group: dev\n192.168.1.1 dev.local\n# END HOSTIE\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderManagedBlock(tt.hf)
			if got != tt.want {
				t.Errorf("RenderManagedBlock() mismatch:\n got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestNoBlankLinePaddingAroundMarkers(t *testing.T) {
	hf := &domain.HostsFile{
		Version: 1,
		Groups: []domain.Group{
			{Name: "g", Entries: []domain.Entry{{IP: "1.1.1.1", Hostname: "h", Enabled: true}}},
		},
	}
	out := RenderManagedBlock(hf)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")

	// Find BEGIN
	var beginIdx, endIdx int = -1, -1
	for i, ln := range lines {
		if ln == BeginMarker {
			beginIdx = i
		}
		if ln == EndMarker {
			endIdx = i
		}
	}
	if beginIdx < 0 || endIdx < 0 {
		t.Fatalf("markers not found in output:\n%s", out)
	}

	// No blank line immediately before BEGIN (only valid if BEGIN is first line).
	if beginIdx != 0 {
		t.Fatalf("BEGIN marker must be first line, got index %d", beginIdx)
	}

	// No blank line immediately after BEGIN.
	if beginIdx+1 < len(lines) && lines[beginIdx+1] == "" {
		t.Errorf("blank line immediately after BEGIN marker:\n%s", out)
	}

	// No blank line immediately before END.
	if endIdx-1 >= 0 && lines[endIdx-1] == "" {
		t.Errorf("blank line immediately before END marker:\n%s", out)
	}

	// END must be the final non-empty line (we trimmed the single trailing \n).
	if endIdx != len(lines)-1 {
		t.Errorf("END marker must be last line; got index %d of %d", endIdx, len(lines)-1)
	}

	// Sole trailing newline after END.
	if !strings.HasSuffix(out, EndMarker+"\n") {
		t.Errorf("output must end with EndMarker + single \\n; got %q", out[len(out)-20:])
	}
	if strings.HasSuffix(out, "\n\n") {
		t.Errorf("output must not end with double newline (no padding below END)")
	}
}

func TestRenderManagedBlockIdempotent(t *testing.T) {
	hf := &domain.HostsFile{
		Version: 1,
		Groups: []domain.Group{
			{Name: "g1", Entries: []domain.Entry{
				{IP: "10.0.0.1", Hostname: "a.local", Aliases: []string{"a"}, Enabled: true, Comment: "x"},
				{IP: "10.0.0.2", Hostname: "b.local", Enabled: false},
			}},
			{Name: "g2", Entries: []domain.Entry{
				{IP: "::1", Hostname: "localhost", Enabled: true},
			}},
		},
	}
	a := RenderManagedBlock(hf)
	b := RenderManagedBlock(hf)
	if a != b {
		t.Fatalf("RenderManagedBlock not idempotent:\na=%q\nb=%q", a, b)
	}
}

func TestRenderHostsFileMatchesRenderManagedBlock(t *testing.T) {
	hf := domain.HostsFile{
		Version: 1,
		Groups: []domain.Group{
			{Name: "g", Entries: []domain.Entry{{IP: "1.2.3.4", Hostname: "h.local", Enabled: true}}},
		},
	}
	if got, want := RenderHostsFile(hf), RenderManagedBlock(&hf); got != want {
		t.Errorf("RenderHostsFile() = %q, want %q (must equal RenderManagedBlock)", got, want)
	}
}
