/**
 * Hosts file rendering
 * 
 * Converts Entry objects to /etc/hosts format lines.
 */

import type { Entry, Group, HostsFile } from "../domain/types";

/**
 * Render an Entry to /etc/hosts format.
 * 
 * Format: '<ip> <hostname> <alias1> <alias2> ...'
 * Optional comment at end if entry.comment exists: '# <comment>'
 * 
 * @param entry - The entry to render
 * @returns A single line in /etc/hosts format
 * 
 * @example
 * renderEntry({
 *   id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
 *   ip: "192.168.1.100",
 *   hostname: "devserver.local",
 *   aliases: ["devserver"],
 *   enabled: true,
 *   comment: "Development server"
 * })
 * // Returns: "192.168.1.100 devserver.local devserver # Development server"
 */
export function renderEntry(entry: Entry): string {
  const parts: string[] = [entry.ip, entry.hostname];

  // Add aliases if present
  if (entry.aliases.length > 0) {
    parts.push(...entry.aliases);
  }

  // Join with single space
  let line = parts.join(" ");

  // Add comment if present
  if (entry.comment) {
    line += ` # ${entry.comment}`;
  }

  // Disabled entries are emitted as `#`-commented lines so they remain
  // visible (and easily re-enabled by toggling the comment) in /etc/hosts.
  // See design.md:108 + rendered example :140-147. (hosts-cli-379.71)
  if (entry.enabled === false) {
    line = `# ${line}`;
  }

  return line;
}

/**
 * Wrap content in managed block markers.
 * 
 * Adds BEGIN HOSTIE and END HOSTIE markers with blank lines for readability.
 * 
 * @param content - The content to wrap (can be empty or multi-line)
 * @returns Content wrapped with markers
 * 
 * @example
 * wrapManagedBlock("192.168.1.1 example.com")
 * // Returns:
 * // # BEGIN HOSTIE
 * //
 * // 192.168.1.1 example.com
 * //
 * // # END HOSTIE
 */
export function wrapManagedBlock(content: string): string {
  return `# BEGIN HOSTIE\n\n${content}\n\n# END HOSTIE`;
}

/**
 * Recursively collect all enabled entries from a group and its subgroups.
 * 
 * @param group - The group to traverse
 * @returns Array of enabled entries (flattened)
 */
function collectAllEntries(group: Group): Entry[] {
  const entries: Entry[] = [];

  // hosts-cli-379.71: include disabled entries. renderEntry now emits them
  // as `#`-commented lines per design.md:108.
  for (const entry of group.entries) {
    entries.push(entry);
  }

  // Recursively collect from subgroups
  for (const subgroup of group.groups) {
    entries.push(...collectAllEntries(subgroup));
  }

  return entries;
}

/**
 * Render a complete HostsFile to /etc/hosts format with managed block markers.
 * 
 * Recursively traverses all groups, collects enabled entries, renders each using
 * renderEntry(), and wraps the result in managed block markers.
 * 
 * Groups are organizational only (per D4) - all enabled entries are flattened
 * into a single list regardless of group hierarchy.
 * 
 * @param hostsFile - The HostsFile to render
 * @returns Complete /etc/hosts content with BEGIN/END markers
 * 
 * @example
 * renderHostsFile({
 *   version: 1,
 *   groups: [
 *     {
 *       name: "dev",
 *       entries: [{ id: "...", ip: "192.168.1.1", hostname: "dev.local", aliases: [], enabled: true }],
 *       groups: []
 *     }
 *   ]
 * })
 * // Returns:
 * // # BEGIN HOSTIE
 * //
 * // 192.168.1.1 dev.local
 * //
 * // # END HOSTIE
 */
export function renderHostsFile(hostsFile: HostsFile): string {
  const allEntries: Entry[] = [];
  
  // Collect ALL entries (enabled and disabled) from all groups (flattened).
  // Disabled entries are rendered as `#`-commented lines by renderEntry.
  for (const group of hostsFile.groups) {
    allEntries.push(...collectAllEntries(group));
  }
  
  // Render each entry
  const lines = allEntries.map(entry => renderEntry(entry));
  
  // Join with newlines and wrap in managed block
  const content = lines.join("\n");
  return wrapManagedBlock(content);
}
