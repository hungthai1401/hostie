/**
 * Apply mechanism for hostie
 * 
 * Implements idempotent application of ~/.hosts to /etc/hosts
 * Only writes if content has changed to avoid unnecessary I/O
 */

import { readFileSync, writeFileSync } from "fs";
import { mkdtempSync, renameSync } from "fs";
import { tmpdir } from "os";
import { join } from "path";

// Type definitions matching design.md
// These will be moved to domain/types.ts when that bead is implemented
type Entry = {
  id: string;
  ip: string;
  hostname: string;
  aliases: string[];
  enabled: boolean;
  comment?: string;
};

type Group = {
  name: string;
  entries: Entry[];
  groups: Group[];
};

export type HostsFile = {
  version: 1;
  groups: Group[];
};

export type ApplyResult = {
  changed: boolean;
  message: string;
};

const BEGIN_MARKER = "# BEGIN HOSTIE";
const END_MARKER = "# END HOSTIE";
const ETC_HOSTS_PATH = "/etc/hosts";

/**
 * Re-execute the current process with sudo
 * 
 * This function is called when EACCES is detected during /etc/hosts write.
 * It re-execs the entire process with sudo, preserving all arguments and stdio.
 * 
 * @throws Never returns - calls process.exit() with sudo's exit code
 */
export async function reexecWithSudo(): Promise<never> {
  // Check if already running as root
  if (process.getuid && process.getuid() === 0) {
    // Already root, cannot escalate further
    throw new Error("Cannot write /etc/hosts even as root");
  }
  
  // Re-exec with sudo, passing through all original arguments
  const result = Bun.spawn(['sudo', Bun.argv[0], ...Bun.argv.slice(1)], {
    stdio: ['inherit', 'inherit', 'inherit']
  });
  
  await result.exited;
  process.exit(result.exitCode ?? 1);
}

/**
 * Render a single entry to hosts file format
 */
function renderEntry(entry: Entry): string {
  if (!entry.enabled) {
    const line = `${entry.ip} ${entry.hostname}${entry.aliases.length > 0 ? " " + entry.aliases.join(" ") : ""}`;
    return `# ${line}${entry.comment ? " # " + entry.comment : ""}`;
  }
  
  const line = `${entry.ip} ${entry.hostname}${entry.aliases.length > 0 ? " " + entry.aliases.join(" ") : ""}`;
  return entry.comment ? `${line} # ${entry.comment}` : line;
}

/**
 * Render a group and its entries recursively
 */
function renderGroup(group: Group, path: string = ""): string[] {
  const lines: string[] = [];
  const groupPath = path ? `${path}/${group.name}` : group.name;
  
  if (group.entries.length > 0) {
    lines.push(`# group: ${groupPath}`);
    for (const entry of group.entries) {
      if (entry.enabled) {
        lines.push(renderEntry(entry));
      }
    }
  }
  
  // Recursively render subgroups
  for (const subgroup of group.groups) {
    const subgroupLines = renderGroup(subgroup, groupPath);
    if (subgroupLines.length > 0) {
      lines.push(...subgroupLines);
    }
  }
  
  return lines;
}

/**
 * Render the managed block content from a HostsFile
 * This is the content that goes between BEGIN/END markers
 */
function renderManagedBlock(hostsFile: HostsFile): string {
  const lines: string[] = [];
  
  for (const group of hostsFile.groups) {
    const groupLines = renderGroup(group);
    if (groupLines.length > 0) {
      lines.push(...groupLines);
    }
  }
  
  return lines.join("\n");
}

/**
 * Extract the managed block from /etc/hosts content
 */
function extractManagedBlock(content: string): {
  before: string;
  block: string | null;
  after: string;
  hasBlock: boolean;
} {
  const lines = content.split("\n");
  let beginIdx = -1;
  let endIdx = -1;

  // Find first BEGIN marker
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].trim() === BEGIN_MARKER) {
      beginIdx = i;
      break;
    }
  }

  // Find first END marker after BEGIN
  if (beginIdx !== -1) {
    for (let i = beginIdx + 1; i < lines.length; i++) {
      if (lines[i].trim() === END_MARKER) {
        endIdx = i;
        break;
      }
    }
  }

  // No block found
  if (beginIdx === -1 || endIdx === -1) {
    return {
      before: content,
      block: null,
      after: "",
      hasBlock: false,
    };
  }

  // Extract parts
  const before = lines.slice(0, beginIdx).join("\n");
  const block = lines.slice(beginIdx + 1, endIdx).join("\n");
  const after = lines.slice(endIdx + 1).join("\n");

  return {
    before: before ? before + "\n" : "",
    block,
    after: after ? "\n" + after : "",
    hasBlock: true,
  };
}

/**
 * Build the new /etc/hosts content with the managed block
 */
function buildNewContent(currentContent: string, newBlock: string): string {
  const extracted = extractManagedBlock(currentContent);

  if (!extracted.hasBlock) {
    // First apply: append block
    const trimmed = currentContent.trimEnd();
    return `${trimmed}\n\n${BEGIN_MARKER}\n${newBlock}\n${END_MARKER}\n`;
  }

  // Replace existing block
  return `${extracted.before}${BEGIN_MARKER}\n${newBlock}\n${END_MARKER}${extracted.after}`;
}

/**
 * Apply a HostsFile to /etc/hosts with idempotency check
 * 
 * Only writes if the content has changed to avoid unnecessary I/O
 * and preserve file modification times when nothing changes.
 * 
 * @param hostsFile - The HostsFile to apply
 * @returns ApplyResult with changed flag and status message
 */
export async function applyHostsFile(hostsFile: HostsFile): Promise<ApplyResult> {
  try {
    // Read current /etc/hosts
    const currentContent = readFileSync(ETC_HOSTS_PATH, "utf-8");
    
    // Render new managed block from hostsFile
    const newBlock = renderManagedBlock(hostsFile);
    
    // Build what the new content would be
    const newContent = buildNewContent(currentContent, newBlock);
    
    // Compare: only write if different
    if (newContent === currentContent) {
      return {
        changed: false,
        message: "/etc/hosts is already up to date (no changes needed)",
      };
    }
    
    // Content differs - write atomically
    try {
      // Atomic write: write to temp file, then rename
      const tempDir = mkdtempSync(join(tmpdir(), 'hostie-'));
      const tempFile = join(tempDir, 'hosts');
      
      writeFileSync(tempFile, newContent, 'utf-8');
      renameSync(tempFile, ETC_HOSTS_PATH);
      
      return {
        changed: true,
        message: "/etc/hosts updated successfully",
      };
    } catch (writeErr: any) {
      // Handle write-specific errors
      if (writeErr.code === "EACCES") {
        // Permission denied - re-exec with sudo
        await reexecWithSudo();
      }
      throw writeErr;
    }
    
  } catch (err: any) {
    // Handle errors gracefully
    if (err.code === "ENOENT") {
      return {
        changed: false,
        message: `/etc/hosts not found: ${err.message}`,
      };
    }
    
    if (err.code === "EACCES") {
      return {
        changed: false,
        message: `Permission denied reading /etc/hosts (may need sudo): ${err.message}`,
      };
    }
    
    throw err;
  }
}
