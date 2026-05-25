package domain

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestTypesRoundTrip marshals a populated HostsFile through yaml.v3 and
// unmarshals it back, asserting DeepEqual.
func TestTypesRoundTrip(t *testing.T) {
	original := HostsFile{
		Version: 1,
		Groups: []Group{
			{
				Name:        "work",
				Description: "work hosts",
				Entries: []Entry{
					{
						ID:       "01KSEFY41HP1CAV4VHYEHW5Z85",
						IP:       "10.0.0.1",
						Hostname: "mover.local",
						Aliases:  []string{"mv", "mover"},
						Enabled:  true,
						Comment:  "primary mover",
					},
				},
			},
			{
				Name: "staging",
				Entries: []Entry{
				{
					ID:       "01KSEFY41NVS9JANTX8D555C2M",
					IP:       "10.0.0.2",
					Hostname: "lonely.local",
					Aliases:  []string{},
					Enabled:  false,
				},
				},
			},
		},
	}

	data, err := yaml.Marshal(&original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var round HostsFile
	if err := yaml.Unmarshal(data, &round); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(original, round) {
		t.Fatalf("round-trip mismatch:\noriginal=%#v\nround=%#v", original, round)
	}
}

// TestTypesOmitemptyDropsZeroValues asserts that yaml struct tags with
// omitempty actually drop zero values from the rendered YAML.
func TestTypesOmitemptyDropsZeroValues(t *testing.T) {
	hf := HostsFile{
		Version: 1,
		Groups: []Group{
			{
				Name:    "bare",
				Entries: []Entry{{ID: "x", IP: "1.1.1.1", Hostname: "h", Enabled: true}},
			},
		},
	}

	data, err := yaml.Marshal(&hf)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := string(data)

	for _, banned := range []string{"description:", "comment:"} {
		if strings.Contains(out, banned) {
			t.Errorf("expected %q to be omitted from YAML, got:\n%s", banned, out)
		}
	}

	for _, required := range []string{"version:", "groups:", "name:", "entries:", "id:", "ip:", "hostname:", "enabled:"} {
		if !strings.Contains(out, required) {
			t.Errorf("expected %q to be present in YAML, got:\n%s", required, out)
		}
	}
}

// TestTypesYAMLTagsPresent verifies via reflection that every exported field
// carries a yaml struct tag (no accidental omissions).
func TestTypesYAMLTagsPresent(t *testing.T) {
	for _, target := range []any{Entry{}, Group{}, HostsFile{}} {
		typ := reflect.TypeOf(target)
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if tag := f.Tag.Get("yaml"); tag == "" {
				t.Errorf("%s.%s missing yaml tag", typ.Name(), f.Name)
			}
		}
	}
}
