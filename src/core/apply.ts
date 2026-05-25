/**
 * Apply mechanism for hostie
 *
 * Implements idempotent application of ~/.hosts to /etc/hosts.
 * Only writes if content has changed to avoid unnecessary I/O.
 *
 * This module composes:
 *   - `core/render.ts:renderEntry`  — single-line rendering of an Entry
 *   - `core/etchosts.ts:writeEtcHosts` — atomic write that preserves mode
 *     + uid + gid across the rename (hosts-cli-379.64).
 *
 * It owns three pieces of behavior that the shared modules do not:
 *   1. A *strict* `extractManagedBlock` that throws on malformed input
 *      (multiple BEGINs, unbalanced markers, wrong order, etc.) so we
 *      never silently append a duplicate block onto a half-written file
 *      (hosts-cli-379.65). `core/etchosts.ts:extractManagedBlock` is the
 *      lenient variant; callers that want fail-fast behavior use this one.
 *   2. The on-disk block layout: BEGIN/<lines>/END with no blank-line
 *      padding, and `# group: <path>` comments before each non-empty
 *      group. This differs from `core/render.ts:renderHostsFile`, which
 *      is used for the `apply --dry-run` preview (flattened, with blank
 *      padding inside the markers).
 *   3. The sudo re-exec path for EACCES on the atomic rename.
 */

import { readFileSync, realpathSync } from "fs";
import type { Entry, Group, HostsFile } from "../domain/types";
import { renderEntry } from "./render";
import { writeEtcHosts } from "./etchosts";

// Re-export HostsFile so existing test imports
// (`import { type HostsFile } from "../../src/core/apply"`) keep working.
export type { HostsFile };

export type ApplyResult = {
  changed: boolean;
  message: string;
};

const BEGIN_MARKER = "# BEGIN HOSTIE";
const END_MARKER = "# END HOSTIE";
const ETC_HOSTS_PATH = "/etc/hosts";

/**
 * Re-execute the current process with sudo.
 *
 * Called when EACCES is detected during /etc/hosts write. Re-execs the
 * entire process with sudo, preserving all arguments and stdio.
 *
 * @throws Never returns - calls process.exit() with sudo's exit code
 */
export async function reexecWithSudo(): Promise<never> {
  // Check if already running as root
  if (process.getuid && process.getuid() === 0) {
    throw new Error("Cannot write /etc/hosts even as root");
  }

  // Re-exec with sudo, passing through all original arguments.
  //
  // Use process.execPath (resolved through any symlinks) rather than
  // Bun.argv[0]. In a `bun build --compile` single-file binary, Bun.argv[0]
  // is the embedded virtual-FS path "/$bunfs/root/<name>", which sudo cannot
  // exec. process.execPath is the real on-disk binary path, which works for
  // compiled binaries, `bun run`, and node. (hosts-cli-379.72)
  let argv0: string;
  try {
    argv0 = realpathSync(process.execPath);
  } catch {
    argv0 = process.execPath;
  }

  const result = Bun.spawn(["sudo", argv0, ...Bun.argv.slice(1)], {
    stdio: ["inherit", "inherit", "inherit"],
  });

  await result.exited;
  process.exit(result.exitCode ?? 1);
}

/**
 * Render a single group and its subgroups into the on-disk block layout
 * used inside /etc/hosts: a `# group: <path>` header followed by each
 * entry (enabled or disabled — disabled are rendered as `#`-commented
 * lines by renderEntry per design.md:108), then recurse into subgroups
 * with their `parent/child` paths. (hosts-cli-379.71)
 */
function renderGroupBlockLines(group: Group, path: string = ""): string[] {
  const lines: string[] = [];
  const groupPath = path ? `${path}/${group.name}` : group.name;

  // Emit a group header when there is at least one entry (enabled or
  // disabled) directly in this group. Empty groups contribute no lines.
  if (group.entries.length > 0) {
    lines.push(`# group: ${groupPath}`);
    for (const entry of group.entries) {
      lines.push(renderEntry(entry));
    }
  }

  // Recurse into subgroups.
  for (const subgroup of group.groups) {
    const subgroupLines = renderGroupBlockLines(subgroup, groupPath);
    if (subgroupLines.length > 0) {
      lines.push(...subgroupLines);
    }
  }

  return lines;
}

/**
 * Render the managed block content from a HostsFile (without BEGIN/END
 * markers — those are added by buildNewContent).
 */
function renderManagedBlock(hostsFile: HostsFile): string {
  const lines: string[] = [];
  for (const group of hostsFile.groups) {
    const groupLines = renderGroupBlockLines(group);
    if (groupLines.length > 0) {
      lines.push(...groupLines);
    }
  }
  return lines.join("\n");
}

/**
 * Extract the managed block from /etc/hosts content (strict variant).
 *
 * Detects malformed marker layouts and throws so callers do NOT silently
 * append a duplicate block on top of a half-written file. Malformed cases:
 *   - BEGIN marker present but no matching END (truncated write)
 *   - END marker present but no preceding BEGIN (manual edit damage)
 *   - Two or more BEGIN markers (duplicated apply or nested block)
 *   - END appears before BEGIN (order corruption)
 *
 * For the lenient variant that tolerates malformed input, see
 * `core/etchosts.ts:extractManagedBlock`.
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
 * Build the new /etc/hosts content with the managed block.
 *
 * Uses the strict extractor — malformed existing content throws here,
 * before any write is attempted.
 */
function buildNewContent(currentContent: string, newBlock: string): string {
  const extracted = extractManagedBlock(currentContent);

  if (!extracted.hasBlock) {
    // First apply: append block.
    const trimmed = currentContent.trimEnd();
    return `${trimmed}\n\n${BEGIN_MARKER}\n${newBlock}\n${END_MARKER}\n`;
  }

  // Replace existing block.
  return `${extracted.before}${BEGIN_MARKER}\n${newBlock}\n${END_MARKER}${extracted.after}`;
}

/**
 * Apply a HostsFile to /etc/hosts with idempotency check.
 *
 * Only writes if the content has changed, to avoid unnecessary I/O and
 * preserve mtime when nothing changes.
 *
 * @param hostsFile - The HostsFile to apply
 * @returns ApplyResult with changed flag and status message
 */
export async function applyHostsFile(hostsFile: HostsFile): Promise<ApplyResult> {
  let currentContent: string;
  try {
    // We read synchronously (and bypass `readEtcHosts`) so the existing
    // `spyOn(fs, "readFileSync")` test surface keeps working unchanged.
    currentContent = readFileSync(ETC_HOSTS_PATH, "utf-8");
  } catch (err: any) {
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

  // Render the managed block body (no markers — buildNewContent adds them).
  const newBlock = renderManagedBlock(hostsFile);

  // Compute the would-be /etc/hosts content. This call also runs the
  // strict extractor — malformed existing content throws here, before
  // any write is attempted (hosts-cli-379.65).
  const newContent = buildNewContent(currentContent, newBlock);

  // Idempotency: skip write when content is unchanged.
  if (newContent === currentContent) {
    return {
      changed: false,
      message: "/etc/hosts is already up to date (no changes needed)",
    };
  }

  // Atomic write via the shared primitive. Preserves mode + uid + gid
  // across the rename (hosts-cli-379.64).
  try {
    await writeEtcHosts(newContent);
    return {
      changed: true,
      message: "/etc/hosts updated successfully",
    };
  } catch (writeErr: any) {
    if (writeErr.code === "EACCES") {
      // Permission denied on rename — re-exec with sudo.
      await reexecWithSudo();
    }
    throw writeErr;
  }
}

// Type alias retained for type-only consumers. (Entry is referenced
// indirectly via renderEntry above; Group is referenced by the walker.)
export type { Entry, Group };
