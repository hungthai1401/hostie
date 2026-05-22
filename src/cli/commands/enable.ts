/**
 * Enable command implementation
 * 
 * Enables a host entry by hostname in ~/.hosts
 */

import { readHostsFile, writeHostsFile } from "../../core/file-io";
import type { HostsFile, Group } from "../../domain/types";
import { ExitCode } from "../exit-codes";

/**
 * Execute the enable command
 * 
 * @param hostname - Hostname to enable
 * @returns Exit code (0 = success, 1 = not found, 2 = I/O error)
 */
export async function enableCommand(hostname: string): Promise<number> {
  try {
    // Read ~/.hosts
    const hostsFile = await readHostsFile("~/.hosts");

    // Find and enable the entry
    let found = false;
    const updatedHostsFile = enableEntryByHostname(hostsFile, hostname, (wasFound) => {
      found = wasFound;
    });

    // If not found, return exit code 1
    if (!found) {
      console.error(`Error: Hostname '${hostname}' not found`);
      return ExitCode.VALIDATION;
    }

    // Write back to ~/.hosts
    await writeHostsFile("~/.hosts", updatedHostsFile);

    console.log(`✓ Enabled '${hostname}'`);
    return ExitCode.SUCCESS;

  } catch (err: any) {
    // Handle I/O errors
    console.error(`Error: ${err.message}`);
    return ExitCode.IO_ERROR;
  }
}

/**
 * Recursively enable an entry by hostname in a HostsFile
 * 
 * @param hostsFile - The hosts file structure
 * @param hostname - Hostname to enable
 * @param onFound - Callback to signal if entry was found
 * @returns Updated HostsFile with entry enabled
 */
function enableEntryByHostname(
  hostsFile: HostsFile,
  hostname: string,
  onFound: (found: boolean) => void
): HostsFile {
  let found = false;

  const updatedGroups = hostsFile.groups.map(group => 
    enableInGroup(group, hostname, (wasFound) => {
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
 * Recursively enable an entry in a group and its nested groups
 * 
 * @param group - The group to process
 * @param hostname - Hostname to enable
 * @param onFound - Callback to signal if entry was found
 * @returns Updated Group with entry enabled
 */
function enableInGroup(
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
        enabled: true,
      };
    }
    return entry;
  });

  // Recursively process nested groups
  const updatedNestedGroups = group.groups.map(nestedGroup =>
    enableInGroup(nestedGroup, hostname, onFound)
  );

  return {
    ...group,
    entries: updatedEntries,
    groups: updatedNestedGroups,
  };
}
