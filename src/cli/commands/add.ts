/**
 * Add command implementation
 * 
 * Adds a new host entry to ~/.hosts
 */

import { readHostsFile, writeHostsFile } from "../../core/file-io";
import { validateIP, validateHostname, validateNoDuplicates } from "../../domain/validators";
import { generateId } from "../../domain/id";
import type { Entry, Group, HostsFile } from "../../domain/types";

export type AddOptions = {
  group?: string;
  disabled?: boolean;
  comment?: string;
  hostsFile?: string;
};

/**
 * Execute the add command
 * 
 * @param ip - IP address (IPv4 or IPv6)
 * @param hostname - Primary hostname
 * @param aliases - Array of alias hostnames
 * @param options - Command options
 * @returns Exit code (0 = success, 1 = validation error, 2 = I/O error)
 */
export async function addCommand(
  ip: string,
  hostname: string,
  aliases: string[],
  options: AddOptions = {}
): Promise<number> {
  try {
    // Validate IP address
    const ipValidation = validateIP(ip);
    if (!ipValidation.valid) {
      console.error(`Error: ${ipValidation.error}`);
      return 1;
    }

    // Validate hostname
    const hostnameValidation = validateHostname(hostname);
    if (!hostnameValidation.valid) {
      console.error(`Error: ${hostnameValidation.error}`);
      return 1;
    }

    // Validate aliases
    for (const alias of aliases) {
      const aliasValidation = validateHostname(alias);
      if (!aliasValidation.valid) {
        console.error(`Error: Invalid alias "${alias}" - ${aliasValidation.error}`);
        return 1;
      }
    }

    // Read existing hosts file
    const hostsFilePath = options.hostsFile || "~/.hosts";
    const hostsFile = await readHostsFile(hostsFilePath);

    // Create new entry
    const newEntry: Entry = {
      id: generateId(),
      ip,
      hostname,
      aliases,
      enabled: !options.disabled,
      comment: options.comment,
    };

    // Check for duplicate hostnames
    const allEntries = collectAllEntries(hostsFile);
    const duplicateCheck = validateNoDuplicates([...allEntries, newEntry]);
    if (!duplicateCheck.valid) {
      console.error(`Error: ${duplicateCheck.error}`);
      return 1;
    }

    // Add entry to the appropriate location
    if (options.group) {
      // Add to specified group (create group path if needed)
      addEntryToGroup(hostsFile, newEntry, options.group);
    } else {
      // Add to root level - we need a synthetic root group or root entries array
      // Based on the design, we'll add a root group if it doesn't exist
      addEntryToRoot(hostsFile, newEntry);
    }

    // Write back to file
    await writeHostsFile(hostsFilePath, hostsFile);

    console.log(`✓ Added ${hostname} (${ip})`);
    return 0;

  } catch (err: any) {
    // Handle I/O errors
    if (err.code === "EACCES" || err.code === "ENOENT" || err.code === "EPERM") {
      console.error(`Error: ${err.message}`);
      return 2;
    }

    // Handle other errors
    console.error(`Error: ${err.message}`);
    return 2;
  }
}

/**
 * Collect all entries from all groups recursively
 */
function collectAllEntries(hostsFile: HostsFile): Entry[] {
  const entries: Entry[] = [];
  
  function collectFromGroup(group: Group) {
    entries.push(...group.entries);
    for (const subgroup of group.groups) {
      collectFromGroup(subgroup);
    }
  }
  
  for (const group of hostsFile.groups) {
    collectFromGroup(group);
  }
  
  return entries;
}

/**
 * Add entry to root level (ungrouped entries)
 */
function addEntryToRoot(hostsFile: HostsFile, entry: Entry) {
  // Look for a root/ungrouped group, or create one
  let rootGroup = hostsFile.groups.find(g => g.name === "");
  
  if (!rootGroup) {
    rootGroup = {
      name: "",
      entries: [],
      groups: [],
    };
    hostsFile.groups.push(rootGroup);
  }
  
  rootGroup.entries.push(entry);
}

/**
 * Add entry to a specific group path, creating groups as needed
 */
function addEntryToGroup(hostsFile: HostsFile, entry: Entry, groupPath: string) {
  const pathSegments = groupPath.split("/").filter(s => s.length > 0);
  
  let currentGroups = hostsFile.groups;
  let currentGroup: Group | undefined;
  
  for (const segment of pathSegments) {
    currentGroup = currentGroups.find(g => g.name === segment);
    
    if (!currentGroup) {
      currentGroup = {
        name: segment,
        entries: [],
        groups: [],
      };
      currentGroups.push(currentGroup);
    }
    
    currentGroups = currentGroup.groups;
  }
  
  // Add entry to the final group
  if (currentGroup) {
    currentGroup.entries.push(entry);
  }
}
