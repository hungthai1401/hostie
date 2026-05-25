/**
 * /etc/hosts file operations for hostie
 *
 * Handles reading and writing /etc/hosts with proper error handling.
 *
 * `writeEtcHosts` is the canonical atomic-write primitive. It:
 *   - Captures the current /etc/hosts mode + uid + gid so the atomic
 *     rename preserves them (regression guard for hosts-cli-379.64;
 *     /etc/hosts must stay 0644 root:wheel/root:root or DNS breaks).
 *   - Writes to a fresh temp directory (`<tmpdir>/hostie-XXXX/hosts`)
 *     so concurrent invocations cannot stomp each other.
 *   - chmod + chown the temp file BEFORE rename, so the destination
 *     inherits the correct perms atomically.
 *   - Tolerates EPERM on chown (non-root callers) — the rename still
 *     proceeds; perms simply remain whatever fs default gave us.
 */

import {
  readFileSync,
  writeFileSync,
  renameSync,
  unlinkSync,
  statSync,
  chmodSync,
  chownSync,
  mkdtempSync,
} from "fs";
import { tmpdir } from "os";
import { join } from "path";

const ETC_HOSTS_PATH = "/etc/hosts";

/**
 * Read /etc/hosts file content.
 *
 * @returns Full content of /etc/hosts as a string, or empty string if file doesn't exist
 */
export async function readEtcHosts(): Promise<string> {
  try {
    const content = readFileSync(ETC_HOSTS_PATH, "utf-8");
    return content;
  } catch (err: any) {
    // Handle ENOENT (file doesn't exist) gracefully
    if (err.code === "ENOENT") {
      return "";
    }
    // Re-throw other errors (like EACCES)
    throw err;
  }
}

/**
 * Extract managed block from /etc/hosts content.
 *
 * Lenient variant used by callers that want to splice content without
 * raising on malformed input. For the strict, throw-on-malformed
 * variant used by `applyHostsFile`, see `core/apply.ts:extractManagedBlock`.
 *
 * @param content - Full /etc/hosts file content
 * @returns Object with before, managed, and after sections
 */
export function extractManagedBlock(content: string): {
  before: string;
  managed: string;
  after: string;
} {
  const beginMarker = "# BEGIN HOSTIE";
  const endMarker = "# END HOSTIE";

  const beginIndex = content.indexOf(beginMarker);
  const endIndex = content.indexOf(endMarker);

  // If markers not found or malformed (only one marker), return original content
  if (beginIndex === -1 || endIndex === -1 || endIndex <= beginIndex) {
    return {
      before: content,
      managed: "",
      after: "",
    };
  }

  // Extract the three sections
  const before = content.substring(0, beginIndex);
  const managed = content.substring(beginIndex, endIndex + endMarker.length);
  const after = content.substring(endIndex + endMarker.length);

  return {
    before,
    managed,
    after,
  };
}

/**
 * Replace managed block in /etc/hosts content with new content.
 *
 * Splices new block between content before and after the existing managed block.
 * If no existing managed block is found, appends the new block with proper spacing.
 * Preserves blank lines for readability.
 *
 * @param original - Original /etc/hosts file content
 * @param newBlock - New managed block content (including BEGIN/END markers)
 * @returns Updated /etc/hosts content with replaced managed block
 */
export function replaceManagedBlock(original: string, newBlock: string): string {
  const { before, after } = extractManagedBlock(original);

  // If no existing managed block (first-time insertion)
  if (before === original && after === "") {
    // If original is empty, just return the new block
    if (original === "") {
      return newBlock;
    }

    // Append new block with blank line for readability.
    // Ensure there's a newline before adding blank line.
    const needsNewline = !original.endsWith("\n");
    return original + (needsNewline ? "\n" : "") + "\n" + newBlock;
  }

  // Replace existing managed block.
  // Preserve any blank lines that were before/after the old block.
  return before + newBlock + after;
}

/**
 * Write content to /etc/hosts atomically while preserving mode + ownership.
 *
 * Strategy: write to a fresh temp dir, chmod + chown the temp file to match
 * the current /etc/hosts, then rename over the destination. The rename is
 * the atomic step; perms are inherited from the source inode.
 *
 * If /etc/hosts does not exist (ENOENT on stat), falls back to mode 0644
 * with no chown (leaves the file owned by the caller).
 *
 * @param content - Full content to write to /etc/hosts
 * @throws Error if write fails (e.g., EACCES for permission denied on rename)
 */
export async function writeEtcHosts(content: string): Promise<void> {
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

  const tempDir = mkdtempSync(join(tmpdir(), "hostie-"));
  const tempFile = join(tempDir, "hosts");

  try {
    writeFileSync(tempFile, content, "utf-8");

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
  } catch (err: any) {
    // Clean up temp file if it exists (best-effort).
    try {
      unlinkSync(tempFile);
    } catch {
      // Ignore cleanup errors
    }
    // Re-throw the original error
    throw err;
  }
}
