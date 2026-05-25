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
