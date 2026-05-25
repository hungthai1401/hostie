// Package domain defines the core data model for Hostie.
//
// These types mirror v1 src/domain/types.ts and use yaml.v3 struct tags that
// match the existing ~/.hosts YAML schema. This file is the single source of
// truth for YAML field names — downstream packages (core/yaml, core/render,
// core/fileio) inherit these tags via yaml.v3 reflection.
package domain

// Entry represents a single host entry (one line in /etc/hosts).
type Entry struct {
	// ID is a ULID, stable across edits, used for TUI selection and references.
	ID string `yaml:"id"`
	// IP is an IPv4 or IPv6 address; validated on save.
	IP string `yaml:"ip"`
	// Hostname is the primary hostname; RFC 952/1123 compliant.
	Hostname string `yaml:"hostname"`
	// Aliases are additional hostnames on the same line.
	Aliases []string `yaml:"aliases"`
	// Enabled controls whether this entry is active. When false the rendered
	// line is `#`-commented out.
	Enabled bool `yaml:"enabled"`
	// Comment is an optional trailing `# comment` in rendered output.
	Comment string `yaml:"comment,omitempty"`
}

// Group is a hierarchical organization unit for entries.
type Group struct {
	// Name is the path segment (kebab-case, no slashes).
	Name string `yaml:"name"`
	// Description is an optional human-readable summary.
	Description string `yaml:"description,omitempty"`
	// Entries are the host entries directly in this group.
	Entries []Entry `yaml:"entries"`
}

// HostsFile is the root structure persisted to ~/.hosts.
type HostsFile struct {
	// Version is the schema version for forward-compat (literal: 1).
	Version int `yaml:"version"`
	// Groups are the top-level groups.
	Groups []Group `yaml:"groups"`
}
