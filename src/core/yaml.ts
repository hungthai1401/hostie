/**
 * YAML serialization and deserialization for HostsFile
 */

import YAML from "yaml";
import type { HostsFile, Group, Entry } from "../domain/types";

/**
 * Deserialize YAML string to HostsFile
 * 
 * @param yaml - YAML string representation of a HostsFile
 * @returns Parsed and validated HostsFile object
 * @throws Error if YAML is invalid, version is missing/unsupported, or schema is invalid
 */
export function deserializeHostsFile(yaml: string): HostsFile {
  let parsed: unknown;

  // Parse YAML
  try {
    parsed = YAML.parse(yaml);
  } catch (error) {
    throw new Error(
      `Invalid YAML: ${error instanceof Error ? error.message : String(error)}`
    );
  }

  // Validate parsed result is an object
  if (!parsed || typeof parsed !== "object") {
    throw new Error("Invalid YAML: expected an object at root");
  }

  const data = parsed as Record<string, unknown>;

  // Validate version field exists
  if (!("version" in data)) {
    throw new Error("Invalid schema: missing required 'version' field");
  }

  // Validate version is supported
  if (data.version !== 1) {
    throw new Error(
      `Unsupported schema version: ${data.version} (only version 1 is supported)`
    );
  }

  // Validate groups field exists and is an array
  if (!("groups" in data)) {
    throw new Error("Invalid schema: missing required 'groups' field");
  }

  if (!Array.isArray(data.groups)) {
    throw new Error("Invalid schema: 'groups' must be an array");
  }

  // Validate and transform groups
  const groups = data.groups.map((group, index) => 
    validateGroup(group, `groups[${index}]`)
  );

  return {
    version: 1,
    groups,
  };
}

/**
 * Validate and transform a group object
 */
function validateGroup(data: unknown, path: string): Group {
  if (!data || typeof data !== "object") {
    throw new Error(`Invalid schema at ${path}: expected an object`);
  }

  const group = data as Record<string, unknown>;

  // Validate name
  if (!("name" in group) || typeof group.name !== "string") {
    throw new Error(`Invalid schema at ${path}: missing or invalid 'name' field`);
  }

  // Validate entries
  if (!("entries" in group) || !Array.isArray(group.entries)) {
    throw new Error(`Invalid schema at ${path}: missing or invalid 'entries' field`);
  }

  // Validate groups (nested)
  if (!("groups" in group) || !Array.isArray(group.groups)) {
    throw new Error(`Invalid schema at ${path}: missing or invalid 'groups' field`);
  }

  const entries = group.entries.map((entry, index) =>
    validateEntry(entry, `${path}.entries[${index}]`)
  );

  const nestedGroups = group.groups.map((nestedGroup, index) =>
    validateGroup(nestedGroup, `${path}.groups[${index}]`)
  );

  return {
    name: group.name,
    entries,
    groups: nestedGroups,
  };
}

/**
 * Validate and transform an entry object
 */
function validateEntry(data: unknown, path: string): Entry {
  if (!data || typeof data !== "object") {
    throw new Error(`Invalid schema at ${path}: expected an object`);
  }

  const entry = data as Record<string, unknown>;

  // Validate required fields
  const requiredFields = ["id", "ip", "hostname", "aliases", "enabled"];
  for (const field of requiredFields) {
    if (!(field in entry)) {
      throw new Error(`Invalid schema at ${path}: missing required '${field}' field`);
    }
  }

  // Validate field types
  if (typeof entry.id !== "string") {
    throw new Error(`Invalid schema at ${path}: 'id' must be a string`);
  }

  if (typeof entry.ip !== "string") {
    throw new Error(`Invalid schema at ${path}: 'ip' must be a string`);
  }

  if (typeof entry.hostname !== "string") {
    throw new Error(`Invalid schema at ${path}: 'hostname' must be a string`);
  }

  if (!Array.isArray(entry.aliases)) {
    throw new Error(`Invalid schema at ${path}: 'aliases' must be an array`);
  }

  if (!entry.aliases.every((alias) => typeof alias === "string")) {
    throw new Error(`Invalid schema at ${path}: all 'aliases' must be strings`);
  }

  if (typeof entry.enabled !== "boolean") {
    throw new Error(`Invalid schema at ${path}: 'enabled' must be a boolean`);
  }

  // Validate optional comment field
  if ("comment" in entry && typeof entry.comment !== "string") {
    throw new Error(`Invalid schema at ${path}: 'comment' must be a string`);
  }

  return {
    id: entry.id,
    ip: entry.ip,
    hostname: entry.hostname,
    aliases: entry.aliases as string[],
    enabled: entry.enabled,
    ...(entry.comment ? { comment: entry.comment as string } : {}),
  };
}
