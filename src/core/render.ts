/**
 * Hosts file rendering
 * 
 * Converts Entry objects to /etc/hosts format lines.
 */

import type { Entry } from "../domain/types";

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
  
  return line;
}
