/**
 * Group command implementation
 * 
 * Manages groups in ~/.hosts
 */

import { readHostsFile, writeHostsFile } from "../../core/file-io";
import type { Group, HostsFile, Entry } from "../../domain/types";

export type GroupCreateOptions = {
  parent?: string;
  hostsFile?: string;
};

/**
 * Validates a group name (must be kebab-case)
 * 
 * Rules:
 * - Lowercase letters, numbers, and hyphens only
 * - Cannot start or end with a hyphen
 * - Cannot contain consecutive hyphens
 * - No slashes (slashes are path separators, not part of a single group name)
 * - At least 1 character
 * 
 * @param name - The group name to validate
 * @returns Validation result with valid flag and optional error message
 */
function validateGroupName(name: string): { valid: boolean; error?: string } {
  // Check for empty string
  if (!name || name.length === 0) {
    return {
      valid: false,
      error: "Group name cannot be empty",
    };
  }

  // Check for slashes (path separators)
  if (name.includes("/")) {
    return {
      valid: false,
      error: "Group name cannot contain slashes (use --parent for nested groups)",
    };
  }

  // Check for leading or trailing hyphen
  if (name.startsWith("-")) {
    return {
      valid: false,
      error: "Group name cannot start with a hyphen",
    };
  }

  if (name.endsWith("-")) {
    return {
      valid: false,
      error: "Group name cannot end with a hyphen",
    };
  }

  // Check for consecutive hyphens
  if (name.includes("--")) {
    return {
      valid: false,
      error: "Group name cannot contain consecutive hyphens",
    };
  }

  // Check for valid kebab-case characters (lowercase letters, numbers, hyphens)
  if (!/^[a-z0-9-]+$/.test(name)) {
    return {
      valid: false,
      error: "Group name must be kebab-case (lowercase letters, numbers, and hyphens only)",
    };
  }

  return {
    valid: true,
  };
}

/**
 * Find a group by path in the hosts file
 * 
 * @param hostsFile - The hosts file to search
 * @param path - The group path (e.g., "work/prod")
 * @returns The group if found, undefined otherwise
 */
function findGroupByPath(hostsFile: HostsFile, path: string): Group | undefined {
  const pathSegments = path.split("/").filter(s => s.length > 0);
  
  let currentGroups = hostsFile.groups;
  let currentGroup: Group | undefined;
  
  for (const segment of pathSegments) {
    currentGroup = currentGroups.find(g => g.name === segment);
    
    if (!currentGroup) {
      return undefined;
    }
    
    currentGroups = currentGroup.groups;
  }
  
  return currentGroup;
}

/**
 * Check if a group name already exists at a specific level
 * 
 * @param groups - The array of groups to search
 * @param name - The group name to check
 * @returns True if the group exists, false otherwise
 */
function groupExists(groups: Group[], name: string): boolean {
  return groups.some(g => g.name === name);
}

/**
 * Execute the group create command
 * 
 * @param name - Group name (must be kebab-case)
 * @param options - Command options
 * @returns Exit code (0 = success, 1 = validation error, 2 = I/O error)
 */
export async function groupCreateCommand(
  name: string,
  options: GroupCreateOptions = {}
): Promise<number> {
  try {
    // Validate group name
    const nameValidation = validateGroupName(name);
    if (!nameValidation.valid) {
      console.error(`Error: ${nameValidation.error}`);
      return 1;
    }

    // Read existing hosts file
    const hostsFilePath = options.hostsFile || "~/.hosts";
    const hostsFile = await readHostsFile(hostsFilePath);

    // Determine where to create the group
    let targetGroups: Group[];
    let parentPath: string;

    if (options.parent) {
      // Find parent group
      const parentGroup = findGroupByPath(hostsFile, options.parent);
      
      if (!parentGroup) {
        console.error(`Error: Parent group "${options.parent}" does not exist`);
        return 1;
      }
      
      targetGroups = parentGroup.groups;
      parentPath = options.parent;
    } else {
      // Create at root level
      targetGroups = hostsFile.groups;
      parentPath = "";
    }

    // Check for duplicate group name at this level
    if (groupExists(targetGroups, name)) {
      const fullPath = parentPath ? `${parentPath}/${name}` : name;
      console.error(`Error: Group "${fullPath}" already exists`);
      return 1;
    }

    // Create new group
    const newGroup: Group = {
      name,
      entries: [],
      groups: [],
    };

    targetGroups.push(newGroup);

    // Write back to file
    await writeHostsFile(hostsFilePath, hostsFile);

    const fullPath = parentPath ? `${parentPath}/${name}` : name;
    console.log(`✓ Created group "${fullPath}"`);
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

export type GroupAddOptions = {
  hostsFile?: string;
};

/**
 * Execute the group add command (moves an entry to a group)
 * 
 * @param groupName - Target group path (e.g., "work" or "work/prod")
 * @param hostname - Hostname of the entry to move
 * @param options - Command options
 * @returns Exit code (0 = success, 1 = entry/group not found, 2 = I/O error)
 */
export async function groupAddCommand(
  groupName: string,
  hostname: string,
  options: GroupAddOptions = {}
): Promise<number> {
  try {
    // Read existing hosts file
    const hostsFilePath = options.hostsFile || "~/.hosts";
    const hostsFile = await readHostsFile(hostsFilePath);

    // Find the target group
    const targetGroup = findGroupByPath(hostsFile, groupName);
    
    if (!targetGroup) {
      console.error(`Error: Group "${groupName}" does not exist`);
      return 1;
    }

    // Find and extract the entry from its current location
    let foundEntry: Entry | null = null;
    const updatedHostsFile = extractEntryByHostname(hostsFile, hostname, (entry) => {
      foundEntry = entry;
    });

    // If entry not found, return exit code 1
    if (!foundEntry) {
      console.error(`Error: Entry with hostname '${hostname}' not found`);
      return 1;
    }

    // Add the entry to the target group
    const finalHostsFile = addEntryToGroup(updatedHostsFile, groupName, foundEntry);

    // Write back to file
    await writeHostsFile(hostsFilePath, finalHostsFile);

    console.log(`✓ Moved '${hostname}' to group "${groupName}"`);
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
 * Extract an entry by hostname from the hosts file (removes it from its current location)
 * 
 * @param hostsFile - The hosts file structure
 * @param hostname - Hostname to extract
 * @param onFound - Callback to receive the found entry
 * @returns Updated HostsFile with entry removed
 */
function extractEntryByHostname(
  hostsFile: HostsFile,
  hostname: string,
  onFound: (entry: Entry | null) => void
): HostsFile {
  let foundEntry: Entry | null = null;

  const updatedGroups = hostsFile.groups.map(group => 
    extractFromGroup(group, hostname, (entry) => {
      if (entry) foundEntry = entry;
    })
  );

  onFound(foundEntry);

  return {
    ...hostsFile,
    groups: updatedGroups,
  };
}

/**
 * Recursively extract an entry from a group and its nested groups
 * 
 * @param group - The group to process
 * @param hostname - Hostname to extract
 * @param onFound - Callback to receive the found entry
 * @returns Updated Group with entry removed
 */
function extractFromGroup(
  group: Group,
  hostname: string,
  onFound: (entry: Entry | null) => void
): Group {
  // Find and extract the entry from this group
  let extractedEntry: Entry | null = null;
  const filteredEntries = group.entries.filter(entry => {
    if (entry.hostname === hostname) {
      extractedEntry = entry;
      onFound(entry);
      return false;
    }
    return true;
  });

  // Recursively process nested groups
  const updatedNestedGroups = group.groups.map(nestedGroup =>
    extractFromGroup(nestedGroup, hostname, onFound)
  );

  return {
    ...group,
    entries: filteredEntries,
    groups: updatedNestedGroups,
  };
}

/**
 * Add an entry to a specific group by path
 * 
 * @param hostsFile - The hosts file structure
 * @param groupPath - Path to the target group (e.g., "work/prod")
 * @param entry - The entry to add
 * @returns Updated HostsFile with entry added to target group
 */
function addEntryToGroup(
  hostsFile: HostsFile,
  groupPath: string,
  entry: Entry
): HostsFile {
  const pathSegments = groupPath.split("/").filter(s => s.length > 0);
  
  const updatedGroups = hostsFile.groups.map(group => 
    addToGroupRecursive(group, pathSegments, entry, 0)
  );

  return {
    ...hostsFile,
    groups: updatedGroups,
  };
}

/**
 * Recursively add an entry to a group at a specific path depth
 * 
 * @param group - The current group
 * @param pathSegments - Array of path segments
 * @param entry - The entry to add
 * @param depth - Current depth in the path
 * @returns Updated Group
 */
function addToGroupRecursive(
  group: Group,
  pathSegments: string[],
  entry: Entry,
  depth: number
): Group {
  // Check if this is the target group
  if (depth < pathSegments.length && group.name === pathSegments[depth]) {
    // If we're at the last segment, add the entry here
    if (depth === pathSegments.length - 1) {
      return {
        ...group,
        entries: [...group.entries, entry],
      };
    }
    
    // Otherwise, recurse into nested groups
    const updatedNestedGroups = group.groups.map(nestedGroup =>
      addToGroupRecursive(nestedGroup, pathSegments, entry, depth + 1)
    );
    
    return {
      ...group,
      groups: updatedNestedGroups,
    };
  }
  
  // Not the target path, return unchanged
  return group;
}
