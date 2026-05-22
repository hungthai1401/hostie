/**
 * List command implementation
 * 
 * Lists all host entries from ~/.hosts
 */

import { readHostsFile } from "../../core/file-io";
import type { Entry, Group, HostsFile } from "../../domain/types";

export type ListOptions = {
  json?: boolean;
  hostsFile?: string;
};

type EntryWithGroup = Entry & {
  group: string;
};

/**
 * Execute the list command
 * 
 * @param options - Command options
 * @returns Exit code (0 = success, 2 = I/O error)
 */
export async function listCommand(options: ListOptions = {}): Promise<number> {
  try {
    // Read ~/.hosts
    const hostsFilePath = options.hostsFile || "~/.hosts";
    const hostsFile = await readHostsFile(hostsFilePath);

    // Collect all entries with their group paths
    const entriesWithGroups = collectEntriesWithGroups(hostsFile);

    // Output based on format
    if (options.json) {
      // JSON format
      console.log(JSON.stringify(entriesWithGroups, null, 2));
    } else {
      // Human-readable table format
      printTable(entriesWithGroups);
    }

    return 0;

  } catch (err: any) {
    // Handle I/O errors
    console.error(`Error: ${err.message}`);
    return 2;
  }
}

/**
 * Collect all entries from all groups with their group paths
 */
function collectEntriesWithGroups(hostsFile: HostsFile): EntryWithGroup[] {
  const entries: EntryWithGroup[] = [];
  
  function collectFromGroup(group: Group, parentPath: string) {
    // Build current group path
    const currentPath = parentPath 
      ? (group.name ? `${parentPath}/${group.name}` : parentPath)
      : group.name;
    
    // Add entries from this group
    for (const entry of group.entries) {
      entries.push({
        ...entry,
        group: currentPath,
      });
    }
    
    // Recursively collect from nested groups
    for (const subgroup of group.groups) {
      collectFromGroup(subgroup, currentPath);
    }
  }
  
  for (const group of hostsFile.groups) {
    collectFromGroup(group, "");
  }
  
  return entries;
}

/**
 * Print entries in human-readable table format
 */
function printTable(entries: EntryWithGroup[]) {
  if (entries.length === 0) {
    console.log("No entries found.");
    return;
  }

  // Print header
  console.log("Status  IP              Hostname                 Aliases          Group");
  console.log("------  --------------  -----------------------  ---------------  -----");

  // Print each entry
  for (const entry of entries) {
    const status = entry.enabled ? "✓" : "✗";
    const ip = entry.ip.padEnd(14);
    const hostname = entry.hostname.padEnd(23);
    const aliases = entry.aliases.join(", ").padEnd(15);
    const group = entry.group || "(ungrouped)";
    
    console.log(`${status}       ${ip}  ${hostname}  ${aliases}  ${group}`);
  }
}
