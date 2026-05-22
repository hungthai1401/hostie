/**
 * Group command implementation
 * 
 * Manages groups in ~/.hosts
 */

import { readHostsFile, writeHostsFile } from "../../core/file-io";
import type { Group, HostsFile } from "../../domain/types";

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
