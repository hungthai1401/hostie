/**
 * Apply mechanism for hostie
 * 
 * Implements idempotent application of ~/.hosts to /etc/hosts
 * Only writes if content has changed to avoid unnecessary I/O
 */

import { readFileSync, writeFileSync, statSync, chmodSync, chownSync } from "fs";
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
 * Extract the managed block from /etc/hosts content.
 *
 * Detects malformed marker layouts and throws so callers do NOT silently
 * append a duplicate block on top of a half-written file. Malformed cases:
 *   - BEGIN marker present but no matching END (truncated write)
 *   - END marker present but no preceding BEGIN (manual edit damage)
 *   - Two or more BEGIN markers (duplicated apply or nested block)
 *   - END appears before BEGIN (order corruption)
 *
 * Exported for direct unit testing.
 *
 * @throws Error with a human-actionable message suggesting either manual
 *         repair of /etc/hosts or rerunning the command with --force.
 */
export function extractManagedBlock(content: string): {
  before: string;
  block: string | null;
  after: string;
  hasBlock: boolean;
} {
  const lines = content.split("\n");

  // Collect every marker occurrence so we can diagnose malformed input.
  const beginIndices: number[] = [];
  const endIndices: number[] = [];
  for (let i = 0; i < lines.length; i++) {
    const trimmed = lines[i].trim();
    if (trimmed === BEGIN_MARKER) beginIndices.push(i);
    else if (trimmed === END_MARKER) endIndices.push(i);
  }

  // No markers at all — first apply, treat as no block.
  if (beginIndices.length === 0 && endIndices.length === 0) {
    return { before: content, block: null, after: "", hasBlock: false };
  }

  const repairHint =
    "Repair /etc/hosts manually (remove the stray markers) or rerun with --force to overwrite the managed region.";

  // Multiple BEGINs (includes nested BEGIN/BEGIN/END/END layouts).
  if (beginIndices.length > 1) {
    throw new Error(
      `Malformed /etc/hosts: found multiple "${BEGIN_MARKER}" markers (lines ${beginIndices
        .map((i) => i + 1)
        .join(", ")}). The managed block must appear exactly once. ${repairHint}`,
    );
  }

  // Multiple ENDs.
  if (endIndices.length > 1) {
    throw new Error(
      `Malformed /etc/hosts: found multiple "${END_MARKER}" markers (lines ${endIndices
        .map((i) => i + 1)
        .join(", ")}). The managed block must appear exactly once. ${repairHint}`,
    );
  }

  // BEGIN without matching END.
  if (beginIndices.length === 1 && endIndices.length === 0) {
    throw new Error(
      `Malformed /etc/hosts: unbalanced markers — found "${BEGIN_MARKER}" at line ${
        beginIndices[0] + 1
      } but no matching "${END_MARKER}". The file may have been truncated mid-write. ${repairHint}`,
    );
  }

  // END without matching BEGIN.
  if (endIndices.length === 1 && beginIndices.length === 0) {
    throw new Error(
      `Malformed /etc/hosts: unbalanced markers — found "${END_MARKER}" at line ${
        endIndices[0] + 1
      } but no preceding "${BEGIN_MARKER}". ${repairHint}`,
    );
  }

  // Exactly one of each — verify ordering.
  const beginIdx = beginIndices[0];
  const endIdx = endIndices[0];
  if (endIdx < beginIdx) {
    throw new Error(
      `Malformed /etc/hosts: unbalanced markers — "${END_MARKER}" at line ${
        endIdx + 1
      } appears before "${BEGIN_MARKER}" at line ${beginIdx + 1} (wrong order). ${repairHint}`,
    );
  }

  // Well-formed: extract the surrounding context.
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
      // Capture original ownership/mode so the atomic rename preserves them.
      // /etc/hosts must remain 0644 root:wheel (macOS) / root:root (Linux);
      // dropping these breaks system DNS lookups (see hosts-cli-379.64).
      let originalMode = 0o644;
      let originalUid: number | null = null;
      let originalGid: number | null = null;
      try {
        const stats = statSync(ETC_HOSTS_PATH);
        // Mask to permission bits only (drop file-type bits) so chmodSync
        // receives a clean mode value.
        originalMode = stats.mode & 0o7777;
        originalUid = stats.uid;
        originalGid = stats.gid;
      } catch (statErr: any) {
        // If /etc/hosts doesn't exist, fall back to 0644 with no chown.
        if (statErr.code !== "ENOENT") throw statErr;
      }

      // Atomic write: write to temp file, apply perms+ownership, then rename.
      const tempDir = mkdtempSync(join(tmpdir(), 'hostie-'));
      const tempFile = join(tempDir, 'hosts');

      writeFileSync(tempFile, newContent, 'utf-8');

      // Preserve mode (0644) and uid/gid before the rename so the destination
      // inherits the correct ownership atomically.
      chmodSync(tempFile, originalMode);
      if (originalUid !== null && originalGid !== null) {
        try {
          chownSync(tempFile, originalUid, originalGid);
        } catch (chownErr: any) {
          // chown may fail with EPERM when not running as root. The atomic
          // rename can still proceed; the file simply keeps the caller's uid.
          // We only swallow EPERM — everything else is fatal.
          if (chownErr.code !== "EPERM") throw chownErr;
        }
      }

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
