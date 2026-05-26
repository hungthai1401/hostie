package yaml

import (
	"reflect"
	"strings"
	"testing"

	"github.com/hungthai1401/hostie/src/internal/domain"
)

func sampleHostsFile() *domain.HostsFile {
	return &domain.HostsFile{
		Version: 1,
		Groups: []domain.Group{
			{
				Name:        "dev",
				Description: "Local development",
				Entries: []domain.Entry{
					{
						ID:       "01HV0000000000000000000001",
						IP:       "127.0.0.1",
						Hostname: "app.local",
						Aliases:  []string{"api.local"},
						Enabled:  true,
						Comment:  "primary",
					},
				},
				Groups: []domain.Group{},
			},
			{
				Name:    "ipv6",
				Entries: []domain.Entry{
				{
					ID:       "01HV0000000000000000000002",
					IP:       "::1",
					Hostname: "ipv6.local",
					Aliases:  []string{},
					Enabled:  false,
				},
				},
				Groups: []domain.Group{},
			},
		},
	}
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	in := sampleHostsFile()
	data, err := Marshal(in)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	out, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal returned error: %v\nbytes:\n%s", err, data)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\nwant: %#v\ngot:  %#v", in, out)
	}
}

func TestMarshalIndentTwoSpaces(t *testing.T) {
	in := sampleHostsFile()
	data, err := Marshal(in)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	lines := strings.Split(string(data), "\n")
	found := false
	for _, line := range lines {
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected at least one line indented with exactly two spaces; got:\n%s", data)
	}
}

func TestMarshalNilHostsFile(t *testing.T) {
	if _, err := Marshal(nil); err == nil {
		t.Fatal("expected error from Marshal(nil), got nil")
	}
}

func TestUnmarshalSchemaErrors(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantSubstr string
	}{
		{
			name:       "missing_version",
			input:      "groups: []\n",
			wantSubstr: "version",
		},
		{
			name:       "bad_version",
			input:      "version: 2\ngroups: []\n",
			wantSubstr: "version",
		},
		{
			name:       "zero_version",
			input:      "version: 0\ngroups: []\n",
			wantSubstr: "version",
		},
		{
			name: "empty_ip",
			input: "version: 1\n" +
				"groups:\n" +
				"  - name: g1\n" +
				"    entries:\n" +
				"      - id: a\n" +
				"        ip: \"\"\n" +
				"        hostname: app.local\n" +
				"        enabled: true\n",
			wantSubstr: "empty ip",
		},
		{
			name: "empty_hostname",
			input: "version: 1\n" +
				"groups:\n" +
				"  - name: g1\n" +
				"    entries:\n" +
				"      - id: a\n" +
				"        ip: 127.0.0.1\n" +
				"        hostname: \"\"\n" +
				"        enabled: true\n",
			wantSubstr: "empty hostname",
		},
		{
			name: "bad_hostname",
			input: "version: 1\n" +
				"groups:\n" +
				"  - name: g1\n" +
				"    entries:\n" +
				"      - id: a\n" +
				"        ip: 127.0.0.1\n" +
				"        hostname: INVALID..NAME\n" +
				"        enabled: true\n",
			wantSubstr: "hostname",
		},
		{
			name:       "non_list_groups",
			input:      "version: 1\ngroups: not-a-list\n",
			wantSubstr: "groups",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := Unmarshal([]byte(tc.input))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantSubstr)
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.wantSubstr)) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}

func TestUnmarshalAcceptsEmptyGroups(t *testing.T) {
	hf, err := Unmarshal([]byte("version: 1\ngroups: []\n"))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if hf.Version != 1 {
		t.Fatalf("version: want 1, got %d", hf.Version)
	}
	if hf.Groups == nil || len(hf.Groups) != 0 {
		t.Fatalf("groups: want empty slice, got %#v", hf.Groups)
	}
}
