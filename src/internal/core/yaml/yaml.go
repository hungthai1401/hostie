// Package yaml is the single seam for ~/.hosts YAML serialization.
//
// Critical Pattern: One Renderer, One Parser. The rest of the Go tree MUST
// funnel ALL ~/.hosts I/O through Marshal and Unmarshal in this package.
// Do not call gopkg.in/yaml.v3 directly from any other internal package.
package yaml

import (
	"bytes"
	"fmt"

	"github.com/hungthai1401/hostie/src/internal/domain"
	goyaml "gopkg.in/yaml.v3"
)

// Marshal renders a HostsFile to canonical YAML bytes using yaml.v3 with
// two-space indentation. The returned bytes are exactly what the encoder
// emits — callers must not trim or rewrite the trailing newline.
func Marshal(hf *domain.HostsFile) ([]byte, error) {
	if hf == nil {
		return nil, fmt.Errorf("yaml.Marshal: HostsFile is nil")
	}
	var buf bytes.Buffer
	enc := goyaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(hf); err != nil {
		_ = enc.Close()
		return nil, fmt.Errorf("yaml.Marshal: encode failed: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("yaml.Marshal: close failed: %w", err)
	}
	return buf.Bytes(), nil
}

// Unmarshal parses YAML bytes into a HostsFile and validates the schema.
// Any schema violation (missing/wrong version, non-list groups, empty IP
// or hostname, invalid IP/hostname) is reported as an error.
func Unmarshal(data []byte) (*domain.HostsFile, error) {
	// Decode into a yaml.Node first so we can distinguish "missing version"
	// from "version: 0" and verify that `groups` is a YAML sequence.
	var root goyaml.Node
	if err := goyaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("yaml.Unmarshal: parse failed: %w", err)
	}

	// Locate the document mapping node.
	var doc *goyaml.Node
	switch root.Kind {
	case goyaml.DocumentNode:
		if len(root.Content) == 0 {
			return nil, fmt.Errorf("yaml.Unmarshal: empty document")
		}
		doc = root.Content[0]
	case goyaml.MappingNode:
		doc = &root
	case 0:
		return nil, fmt.Errorf("yaml.Unmarshal: empty document")
	default:
		return nil, fmt.Errorf("yaml.Unmarshal: top-level must be a mapping")
	}
	if doc.Kind != goyaml.MappingNode {
		return nil, fmt.Errorf("yaml.Unmarshal: top-level must be a mapping")
	}

	// Scan top-level keys for explicit shape checks.
	var hasVersion bool
	for i := 0; i+1 < len(doc.Content); i += 2 {
		key := doc.Content[i]
		val := doc.Content[i+1]
		switch key.Value {
		case "version":
			hasVersion = true
		case "groups":
			if val.Kind != goyaml.SequenceNode {
				return nil, fmt.Errorf("yaml.Unmarshal: groups must be a list (got non-list YAML node)")
			}
		}
	}
	if !hasVersion {
		return nil, fmt.Errorf("yaml.Unmarshal: missing required key \"version\" (expected version: 1)")
	}

	// Now decode into the typed struct.
	hf := &domain.HostsFile{}
	if err := doc.Decode(hf); err != nil {
		return nil, fmt.Errorf("yaml.Unmarshal: decode failed: %w", err)
	}
	if hf.Groups == nil {
		hf.Groups = []domain.Group{}
	}
	if err := Validate(hf); err != nil {
		return nil, err
	}
	return hf, nil
}

// Validate enforces the HostsFile schema invariants:
//   - non-nil pointer
//   - version == 1 (0 / missing is rejected with "version" in the message)
//   - every entry has a non-empty, well-formed IP and Hostname
func Validate(hf *domain.HostsFile) error {
	if hf == nil {
		return fmt.Errorf("validate: HostsFile is nil")
	}
	if hf.Version != 1 {
		if hf.Version == 0 {
			return fmt.Errorf("validate: unsupported version 0 (missing or zero); expected version: 1")
		}
		return fmt.Errorf("validate: unsupported version %d; expected version: 1", hf.Version)
	}
	// Groups may be empty but must not be a non-list shape; the Unmarshal path
	// guarantees the shape, and an empty []Group is permitted here.
	for gi, g := range hf.Groups {
		for ei, e := range g.Entries {
			if e.IP == "" {
				return fmt.Errorf("validate: groups[%d].entries[%d]: empty ip", gi, ei)
			}
			if e.Hostname == "" {
				return fmt.Errorf("validate: groups[%d].entries[%d]: empty hostname", gi, ei)
			}
			if err := domain.ValidateIP(e.IP); err != nil {
				return fmt.Errorf("validate: groups[%d].entries[%d]: %w", gi, ei, err)
			}
			if err := domain.ValidateHostname(e.Hostname); err != nil {
				return fmt.Errorf("validate: groups[%d].entries[%d]: %w", gi, ei, err)
			}
			for ai, a := range e.Aliases {
				if err := domain.ValidateHostname(a); err != nil {
					return fmt.Errorf("validate: groups[%d].entries[%d].aliases[%d]: %w", gi, ei, ai, err)
				}
			}
		}
	}
	return nil
}
