/**
 * Disable command implementation
 * 
 * Disables a host entry by hostname in ~/.hosts
 */

import { readHostsFile, writeHostsFile } from "../../core/file-io";
import type { HostsFile, Group, Entry } from "../../domain/types";

/**
 * Execute the disable command
 * 
 * @param hostname - Hostname to disable
 * @returns Exit code (0 = success, 1 = not found, 2 = I/O error)
 */
export async function disableCommand(hostname: string): Promise<number> {
  try {
    // Read ~/.hosts
    const hostsFile = await readHostsFile("~/.hosts");

    // Find and disable the entry
    let found = false;
    const updatedHostsFile = disableEntryByHostname(hostsFile, hostname, (wasFound) => {
      found = wasFound;
    });

    // If not found, return exit code 1
    if (!found) {
      console.error(`Error: Hostname '${hostname}' not found`);
      return 1;
    }

    // Write back to ~/.hosts
    await writeHostsFile("~/.hosts", updatedHostsFile);

    console.log(`✓ Disabled '${hostname}'`);
    return 0;

  } catch (err: any) {
    // Handle I/O errors
    console.error(`Error: ${err.message}`);
    return 2;
  }
}

/**
 * Recursively disable an entry by hostname in a HostsFile
 * 
 * @param hostsFile - The hosts file structure
 * @param hostname - Hostname to disable
 * @param onFound - Callback to signal if entry was found
 * @returns Updated HostsFile with entry disabled
 */
function disableEntryByHostname(
  hostsFile: HostsFile,
  hostname: string,
  onFound: (found: boolean) => void
): HostsFile {
  let found = false;

  const updatedGroups = hostsFile.groups.map(group => 
    disableInGroup(group, hostname, (wasFound) => {
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
 * Recursively disable an entry in a group and its nested groups
 * 
 * @param group - The group to process
 * @param hostname - Hostname to disable
 * @param onFound - Callback to signal if entry was found
 * @returns Updated Group with entry disabled
 */
function disableInGroup(
  group: Group,
  hostname: string,
  onFound: (found: boolean) => void
): Group {
  // Update entries in this group
  const updatedEntries = group.entries.map(entry => {
    if (entry.hostname === hostname) {
      onFound(true);
      return {
        ...entry,
        enabled: false,
      };
    }
    return entry;
  });

  // Recursively process nested groups
  const updatedNestedGroups = group.groups.map(nestedGroup =>
    disableInGroup(nestedGroup, hostname, onFound)
  );

  return {
    ...group,
    entries: updatedEntries,
    groups: updatedNestedGroups,
  };
}
