/**
 * Unit tests for src/core/apply.ts — managed block extraction.
 *
 * Focus of this suite: extractManagedBlock must detect malformed
 * /etc/hosts content and refuse to operate on it, rather than silently
 * appending a duplicate block (the bug fixed in hosts-cli-379.65).
 *
 * Malformed cases covered:
 *   1. BEGIN marker only (no END)
 *   2. END marker only (no BEGIN)
 *   3. Two BEGIN markers (duplicate / un-closed previous block)
 *   4. Nested markers (BEGIN, BEGIN, END, END)
 *   5. END appears before BEGIN
 *
 * Plus sanity tests for the well-formed cases:
 *   - No markers at all (first apply — returns hasBlock: false)
 *   - Exactly one balanced BEGIN/END pair (returns hasBlock: true)
 *
 * End-to-end: applyHostsFile must surface the malformed error to the
 * caller instead of corrupting /etc/hosts with a duplicate block.
 */

import { describe, test, expect, spyOn, afterEach } from "bun:test";
import * as fs from "fs";

import {
  extractManagedBlock,
  applyHostsFile,
  type HostsFile,
} from "../../src/core/apply";

const BEGIN = "# BEGIN HOSTIE";
const END = "# END HOSTIE";

// ---------------------------------------------------------------------------
// extractManagedBlock — happy paths
// ---------------------------------------------------------------------------

describe("extractManagedBlock — well-formed input", () => {
  test("returns hasBlock=false when neither marker is present", () => {
    const content = "127.0.0.1 localhost\n::1 localhost\n";
    const result = extractManagedBlock(content);
    expect(result.hasBlock).toBe(false);
    expect(result.block).toBeNull();
    expect(result.before).toBe(content);
    expect(result.after).toBe("");
  });

  test("returns hasBlock=true for a single balanced BEGIN/END pair", () => {
    const content = [
      "127.0.0.1 localhost",
      "",
      BEGIN,
      "# group: work",
      "10.0.0.1 api.local",
      END,
      "",
    ].join("\n");

    const result = extractManagedBlock(content);
    expect(result.hasBlock).toBe(true);
    expect(result.block).toBe("# group: work\n10.0.0.1 api.local");
    expect(result.before).toContain("127.0.0.1 localhost");
  });

  test("returns hasBlock=true and preserves trailing content after END", () => {
    const content = [
      "127.0.0.1 localhost",
      BEGIN,
      "10.0.0.1 api.local",
      END,
      "192.168.1.1 router",
      "",
    ].join("\n");

    const result = extractManagedBlock(content);
    expect(result.hasBlock).toBe(true);
    expect(result.block).toBe("10.0.0.1 api.local");
    expect(result.after).toContain("192.168.1.1 router");
  });
});

// ---------------------------------------------------------------------------
// extractManagedBlock — malformed input must throw
// ---------------------------------------------------------------------------

describe("extractManagedBlock — malformed input", () => {
  test("throws when BEGIN marker exists but no END marker follows", () => {
    const content = [
      "127.0.0.1 localhost",
      BEGIN,
      "10.0.0.1 api.local",
      // truncated — END marker missing
    ].join("\n");

    expect(() => extractManagedBlock(content)).toThrow(/unbalanced/i);
    expect(() => extractManagedBlock(content)).toThrow(/--force/);
  });

  test("throws when END marker exists with no preceding BEGIN", () => {
    const content = [
      "127.0.0.1 localhost",
      "10.0.0.1 api.local",
      END,
      "192.168.1.1 router",
    ].join("\n");

    expect(() => extractManagedBlock(content)).toThrow(/unbalanced/i);
    expect(() => extractManagedBlock(content)).toThrow(/--force/);
  });

  test("throws when two BEGIN markers appear", () => {
    const content = [
      "127.0.0.1 localhost",
      BEGIN,
      "10.0.0.1 api.local",
      END,
      "",
      BEGIN,
      "10.0.0.2 db.local",
      END,
    ].join("\n");

    expect(() => extractManagedBlock(content)).toThrow(/multiple/i);
    expect(() => extractManagedBlock(content)).toThrow(/--force/);
  });

  test("throws when markers are nested (BEGIN, BEGIN, END, END)", () => {
    const content = [
      "127.0.0.1 localhost",
      BEGIN,
      "10.0.0.1 api.local",
      BEGIN,
      "10.0.0.2 db.local",
      END,
      END,
    ].join("\n");

    expect(() => extractManagedBlock(content)).toThrow(
      /nested|multiple/i,
    );
    expect(() => extractManagedBlock(content)).toThrow(/--force/);
  });

  test("throws when END appears before BEGIN (single of each)", () => {
    const content = [
      "127.0.0.1 localhost",
      END,
      "10.0.0.1 api.local",
      BEGIN,
      "10.0.0.2 db.local",
    ].join("\n");

    expect(() => extractManagedBlock(content)).toThrow(
      /wrong order|unbalanced/i,
    );
    expect(() => extractManagedBlock(content)).toThrow(/--force/);
  });

  test("error message mentions both manual repair and --force hint", () => {
    const content = `${BEGIN}\nfoo\n`; // BEGIN only
    try {
      extractManagedBlock(content);
      throw new Error("should have thrown");
    } catch (err) {
      const msg = (err as Error).message;
      expect(msg).toMatch(/--force/);
      // The message should give the operator something to do
      expect(msg.length).toBeGreaterThan(40);
    }
  });
});

// ---------------------------------------------------------------------------
// applyHostsFile end-to-end — malformed /etc/hosts must surface error
// ---------------------------------------------------------------------------

describe("applyHostsFile — malformed /etc/hosts", () => {
  const emptyHostsFile: HostsFile = { version: 1, groups: [] };

  afterEach(() => {
    // bun:test restores spies between describe blocks but be explicit
  });

  test("rejects /etc/hosts with BEGIN marker but no END (no duplicate append)", async () => {
    const malformed = `127.0.0.1 localhost\n${BEGIN}\n10.0.0.1 api.local\n`;
    const readSpy = spyOn(fs, "readFileSync").mockReturnValue(malformed);
    const writeSpy = spyOn(fs, "writeFileSync").mockImplementation(() => {});

    try {
      await expect(applyHostsFile(emptyHostsFile)).rejects.toThrow(
        /unbalanced|malformed/i,
      );
      // CRITICAL: we must NOT have written anything when the input is malformed.
      expect(writeSpy).not.toHaveBeenCalled();
    } finally {
      readSpy.mockRestore();
      writeSpy.mockRestore();
    }
  });

  test("rejects /etc/hosts with two BEGIN markers", async () => {
    const malformed = [
      "127.0.0.1 localhost",
      BEGIN,
      "10.0.0.1 api.local",
      END,
      BEGIN,
      "10.0.0.2 db.local",
      END,
      "",
    ].join("\n");
    const readSpy = spyOn(fs, "readFileSync").mockReturnValue(malformed);
    const writeSpy = spyOn(fs, "writeFileSync").mockImplementation(() => {});

    try {
      await expect(applyHostsFile(emptyHostsFile)).rejects.toThrow(
        /multiple|nested|unbalanced/i,
      );
      expect(writeSpy).not.toHaveBeenCalled();
    } finally {
      readSpy.mockRestore();
      writeSpy.mockRestore();
    }
  });
});

// ---------------------------------------------------------------------------
// applyHostsFile — preserves /etc/hosts mode + uid/gid across atomic rename
// ---------------------------------------------------------------------------
//
// Regression for hosts-cli-379.64: the previous implementation wrote a temp
// file and renamed it over /etc/hosts but never chmod/chown'd the temp, so
// after `hostie apply` /etc/hosts could end up 0600 or owned by the calling
// user — which breaks system DNS lookups (must remain 0644 root:wheel on
// macOS / root:root on Linux). Mirrors src/core/etchosts.ts:writeEtcHosts.

describe("applyHostsFile — preserves mode + uid + gid", () => {
  const hostsFileWithEntry: HostsFile = {
    version: 1,
    groups: [
      {
        name: "work",
        entries: [
          {
            id: "e1",
            ip: "10.0.0.1",
            hostname: "api.local",
            aliases: [],
            enabled: true,
          },
        ],
        groups: [],
      },
    ],
  };

  test("chmods + chowns temp file to original /etc/hosts mode/uid/gid before rename", async () => {
    const ORIGINAL_MODE = 0o644;
    const ORIGINAL_UID = 0;
    const ORIGINAL_GID = 0;

    // No existing managed block → buildNewContent will append.
    const currentEtcHosts = "127.0.0.1 localhost\n";

    const readSpy = spyOn(fs, "readFileSync").mockReturnValue(currentEtcHosts);
    const writeSpy = spyOn(fs, "writeFileSync").mockImplementation(() => {});
    const statSpy = spyOn(fs, "statSync").mockImplementation((p: any) => {
      // Only intercept the /etc/hosts stat. Return a minimal Stats-like obj
      // with the file-type bit set so the mask in apply.ts is exercised.
      if (String(p) === "/etc/hosts") {
        return {
          mode: 0o100000 | ORIGINAL_MODE, // S_IFREG | 0644
          uid: ORIGINAL_UID,
          gid: ORIGINAL_GID,
        } as any;
      }
      // Fall through to a generic stat for any other path bun internals hit.
      return { mode: 0o100644, uid: 0, gid: 0 } as any;
    });
    const chmodSpy = spyOn(fs, "chmodSync").mockImplementation(() => {});
    const chownSpy = spyOn(fs, "chownSync").mockImplementation(() => {});
    const renameSpy = spyOn(fs, "renameSync").mockImplementation(() => {});

    try {
      const result = await applyHostsFile(hostsFileWithEntry);
      expect(result.changed).toBe(true);

      // The temp file must have been chmod'd to 0644 (mask of the original).
      expect(chmodSpy).toHaveBeenCalledTimes(1);
      const [chmodPath, chmodMode] = chmodSpy.mock.calls[0];
      expect(chmodMode).toBe(ORIGINAL_MODE);
      expect(String(chmodPath)).toContain("hostie-"); // temp dir prefix

      // The temp file must have been chown'd to the original uid/gid.
      expect(chownSpy).toHaveBeenCalledTimes(1);
      const [chownPath, uid, gid] = chownSpy.mock.calls[0];
      expect(uid).toBe(ORIGINAL_UID);
      expect(gid).toBe(ORIGINAL_GID);
      expect(String(chownPath)).toContain("hostie-");

      // chmod + chown must both happen BEFORE the rename so the destination
      // inherits the correct perms atomically.
      expect(renameSpy).toHaveBeenCalledTimes(1);
      const chmodOrder = chmodSpy.mock.invocationCallOrder[0];
      const chownOrder = chownSpy.mock.invocationCallOrder[0];
      const renameOrder = renameSpy.mock.invocationCallOrder[0];
      expect(chmodOrder).toBeLessThan(renameOrder);
      expect(chownOrder).toBeLessThan(renameOrder);

      // The rename target must be /etc/hosts.
      const [, renameTarget] = renameSpy.mock.calls[0];
      expect(String(renameTarget)).toBe("/etc/hosts");
    } finally {
      readSpy.mockRestore();
      writeSpy.mockRestore();
      statSpy.mockRestore();
      chmodSpy.mockRestore();
      chownSpy.mockRestore();
      renameSpy.mockRestore();
    }
  });

  test("masks off file-type bits so chmod receives only permission bits", async () => {
    // If /etc/hosts is 0644 with S_IFREG, stats.mode is 0o100644.
    // apply.ts must mask to 0o7777 before chmod, otherwise chmod would set
    // bogus high bits.
    const readSpy = spyOn(fs, "readFileSync").mockReturnValue("127.0.0.1 localhost\n");
    const writeSpy = spyOn(fs, "writeFileSync").mockImplementation(() => {});
    const statSpy = spyOn(fs, "statSync").mockImplementation(() => ({
      mode: 0o100644,
      uid: 0,
      gid: 0,
    } as any));
    const chmodSpy = spyOn(fs, "chmodSync").mockImplementation(() => {});
    const chownSpy = spyOn(fs, "chownSync").mockImplementation(() => {});
    const renameSpy = spyOn(fs, "renameSync").mockImplementation(() => {});

    try {
      await applyHostsFile(hostsFileWithEntry);
      const [, chmodMode] = chmodSpy.mock.calls[0];
      // Must be 0o644, NOT 0o100644.
      expect(chmodMode).toBe(0o644);
      expect(chmodMode! & 0o170000).toBe(0); // no file-type bits leaked
    } finally {
      readSpy.mockRestore();
      writeSpy.mockRestore();
      statSpy.mockRestore();
      chmodSpy.mockRestore();
      chownSpy.mockRestore();
      renameSpy.mockRestore();
    }
  });

  test("tolerates EPERM on chown (non-root caller) and still completes rename", async () => {
    const readSpy = spyOn(fs, "readFileSync").mockReturnValue("127.0.0.1 localhost\n");
    const writeSpy = spyOn(fs, "writeFileSync").mockImplementation(() => {});
    const statSpy = spyOn(fs, "statSync").mockImplementation(() => ({
      mode: 0o100644,
      uid: 0,
      gid: 0,
    } as any));
    const chmodSpy = spyOn(fs, "chmodSync").mockImplementation(() => {});
    const chownSpy = spyOn(fs, "chownSync").mockImplementation(() => {
      const err: any = new Error("operation not permitted");
      err.code = "EPERM";
      throw err;
    });
    const renameSpy = spyOn(fs, "renameSync").mockImplementation(() => {});

    try {
      const result = await applyHostsFile(hostsFileWithEntry);
      expect(result.changed).toBe(true);
      // Rename must still happen — EPERM on chown is non-fatal.
      expect(renameSpy).toHaveBeenCalledTimes(1);
    } finally {
      readSpy.mockRestore();
      writeSpy.mockRestore();
      statSpy.mockRestore();
      chmodSpy.mockRestore();
      chownSpy.mockRestore();
      renameSpy.mockRestore();
    }
  });
});
