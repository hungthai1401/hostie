/**
 * Remove command implementation
 * 
 * Removes a host entry by hostname from ~/.hosts
 */

import { readHostsFile, writeHostsFile } from "../../core/file-io";
import type { HostsFile, Group } from "../../domain/types";
import { ExitCode } from "../exit-codes";

/**
 * Execute the rm command
 * 
 * @param hostname - Hostname to remove
 * @returns Exit code (0 = success, 1 = not found, 2 = I/O error)
 */
export async function rmCommand(hostname: string): Promise<number> {
  try {
    // Read ~/.hosts
    const hostsFile = await readHostsFile("~/.hosts");

    // Find and remove the entry
    let found = false;
    const updatedHostsFile = removeEntryByHostname(hostsFile, hostname, (wasFound) => {
      found = wasFound;
    });

    // If not found, return exit code 1
    if (!found) {
      console.error(`Error: Hostname '${hostname}' not found`);
      return ExitCode.VALIDATION;
    }

    // Write back to ~/.hosts
    await writeHostsFile("~/.hosts", updatedHostsFile);

    console.log(`✓ Removed '${hostname}'`);
    return ExitCode.SUCCESS;

  } catch (err: any) {
    // Handle I/O errors
    console.error(`Error: ${err.message}`);
    return ExitCode.IO_ERROR;
  }
}

/**
 * Recursively remove an entry by hostname from a HostsFile
 * 
 * @param hostsFile - The hosts file structure
 * @param hostname - Hostname to remove
 * @param onFound - Callback to signal if entry was found
 * @returns Updated HostsFile with entry removed
 */
function removeEntryByHostname(
  hostsFile: HostsFile,
  hostname: string,
  onFound: (found: boolean) => void
): HostsFile {
  let found = false;

  const updatedGroups = hostsFile.groups.map(group => 
    removeFromGroup(group, hostname, (wasFound) => {
      if (wasFound) found = true;
    })
  );

  onFound(found);

  return {
    ...hostsFile,
    groups: updatedGroups,
  };
}

/**
 * Recursively remove an entry from a group and its nested groups
 * 
 * @param group - The group to process
 * @param hostname - Hostname to remove
 * @param onFound - Callback to signal if entry was found
 * @returns Updated Group with entry removed
 */
function removeFromGroup(
  group: Group,
  hostname: string,
  onFound: (found: boolean) => void
): Group {
  // Filter entries in this group
  const filteredEntries = group.entries.filter(entry => {
    if (entry.hostname === hostname) {
      onFound(true);
      return false;
    }
    return true;
  });

  // Recursively process nested groups
  const updatedNestedGroups = group.groups.map(nestedGroup =>
    removeFromGroup(nestedGroup, hostname, onFound)
  );

  return {
    ...group,
    entries: filteredEntries,
    groups: updatedNestedGroups,
  };
}
