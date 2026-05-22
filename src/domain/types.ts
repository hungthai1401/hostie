/**
 * Core data model for Hostie
 * 
 * Represents the structure of host entries, groups, and the overall hosts file.
 */

/**
 * Entry: represents a single host entry
 */
export type Entry = {
  /** ULID; stable across edits, used for TUI selection & references */
  id: string;
  /** IPv4 or IPv6; validated on save */
  ip: string;
  /** Primary hostname; RFC 952/1123 compliant */
  hostname: string;
  /** Additional hostnames on the same line (default: []) */
  aliases: string[];
  /** Whether this entry is active (default: true; false → emitted as a `#`-commented line) */
  enabled: boolean;
  /** Optional trailing `# comment` in rendered output */
  comment?: string;
};

/**
 * Group: hierarchical organization
 */
export type Group = {
  /** Path-segment; e.g., "prod" inside "work/prod" (kebab-case, no slashes) */
  name: string;
  /** Entries directly in this group */
  entries: Entry[];
  /** Nested subgroups (recursive) */
  groups: Group[];
};

/**
 * HostsFile: root structure
 */
export type HostsFile = {
  /** Schema version for forward-compat */
  version: 1;
  /** Top-level groups (a synthetic "ungrouped" root holds loose entries if any) */
  groups: Group[];
};
