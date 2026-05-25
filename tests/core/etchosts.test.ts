import { describe, test, expect } from "bun:test";
import { extractManagedBlock, replaceManagedBlock } from "../../src/core/etchosts";

describe("etchosts: managed block operations", () => {
  describe("extractManagedBlock", () => {
    test("extracts managed block with content before and after", () => {
      const content = `127.0.0.1 localhost
# BEGIN HOSTIE
192.168.1.10 dev.local
# END HOSTIE
::1 localhost`;

      const result = extractManagedBlock(content);

      expect(result.before).toBe("127.0.0.1 localhost\n");
      expect(result.managed).toBe("# BEGIN HOSTIE\n192.168.1.10 dev.local\n# END HOSTIE");
      expect(result.after).toBe("\n::1 localhost");
    });

    test("returns full content as 'before' when no markers exist", () => {
      const content = `127.0.0.1 localhost\n::1 localhost\n`;

      const result = extractManagedBlock(content);

      expect(result.before).toBe(content);
      expect(result.managed).toBe("");
      expect(result.after).toBe("");
    });

    test("handles empty input", () => {
      const result = extractManagedBlock("");

      expect(result.before).toBe("");
      expect(result.managed).toBe("");
      expect(result.after).toBe("");
    });

    test("treats lone BEGIN marker as malformed (no extraction)", () => {
      const content = `127.0.0.1 localhost
# BEGIN HOSTIE
192.168.1.10 dev.local`;

      const result = extractManagedBlock(content);

      expect(result.before).toBe(content);
      expect(result.managed).toBe("");
      expect(result.after).toBe("");
    });

    test("treats lone END marker as malformed (no extraction)", () => {
      const content = `127.0.0.1 localhost
192.168.1.10 dev.local
# END HOSTIE`;

      const result = extractManagedBlock(content);

      expect(result.before).toBe(content);
      expect(result.managed).toBe("");
      expect(result.after).toBe("");
    });

    test("treats END before BEGIN as malformed", () => {
      const content = `# END HOSTIE
some entry
# BEGIN HOSTIE`;

      const result = extractManagedBlock(content);

      expect(result.before).toBe(content);
      expect(result.managed).toBe("");
      expect(result.after).toBe("");
    });

    test("handles an empty managed block", () => {
      const content = `127.0.0.1 localhost
# BEGIN HOSTIE
# END HOSTIE
::1 localhost`;

      const result = extractManagedBlock(content);

      expect(result.before).toBe("127.0.0.1 localhost\n");
      expect(result.managed).toBe("# BEGIN HOSTIE\n# END HOSTIE");
      expect(result.after).toBe("\n::1 localhost");
    });

    test("preserves multi-line content outside markers verbatim", () => {
      const content = `# System hosts
127.0.0.1 localhost

# BEGIN HOSTIE
192.168.1.10 dev.local
192.168.1.11 staging.local
# END HOSTIE

# IPv6
::1 localhost`;

      const result = extractManagedBlock(content);

      expect(result.before).toBe("# System hosts\n127.0.0.1 localhost\n\n");
      expect(result.managed).toBe(
        "# BEGIN HOSTIE\n192.168.1.10 dev.local\n192.168.1.11 staging.local\n# END HOSTIE",
      );
      expect(result.after).toBe("\n\n# IPv6\n::1 localhost");
    });

    test("round-trip: before + managed + after equals original", () => {
      const content = `alpha
# BEGIN HOSTIE
managed line 1
managed line 2
# END HOSTIE
omega`;

      const result = extractManagedBlock(content);

      expect(result.before + result.managed + result.after).toBe(content);
    });
  });

  describe("replaceManagedBlock", () => {
    test("replaces an existing managed block in place", () => {
      const original = `127.0.0.1 localhost
# BEGIN HOSTIE
192.168.1.10 old.local
# END HOSTIE
::1 localhost`;
      const newBlock = `# BEGIN HOSTIE
192.168.1.20 new.local
192.168.1.21 another.local
# END HOSTIE`;

      const result = replaceManagedBlock(original, newBlock);

      expect(result).toBe(`127.0.0.1 localhost
# BEGIN HOSTIE
192.168.1.20 new.local
192.168.1.21 another.local
# END HOSTIE
::1 localhost`);
    });

    test("preserves content before and after the markers", () => {
      const original = `# System hosts
127.0.0.1 localhost

# BEGIN HOSTIE
192.168.1.10 dev.local
# END HOSTIE

# IPv6
::1 localhost`;
      const newBlock = `# BEGIN HOSTIE
192.168.1.30 prod.local
# END HOSTIE`;

      const result = replaceManagedBlock(original, newBlock);

      expect(result).toBe(`# System hosts
127.0.0.1 localhost

# BEGIN HOSTIE
192.168.1.30 prod.local
# END HOSTIE

# IPv6
::1 localhost`);
    });

    test("first-time insertion appends with a blank-line separator", () => {
      const original = `127.0.0.1 localhost\n::1 localhost\n`;
      const newBlock = `# BEGIN HOSTIE
192.168.1.10 dev.local
# END HOSTIE`;

      const result = replaceManagedBlock(original, newBlock);

      expect(result).toBe(
        `127.0.0.1 localhost\n::1 localhost\n\n# BEGIN HOSTIE\n192.168.1.10 dev.local\n# END HOSTIE`,
      );
    });

    test("first-time insertion adds newline when original lacks trailing newline", () => {
      const original = `127.0.0.1 localhost`;
      const newBlock = `# BEGIN HOSTIE\n192.168.1.10 dev.local\n# END HOSTIE`;

      const result = replaceManagedBlock(original, newBlock);

      expect(result).toBe(
        `127.0.0.1 localhost\n\n# BEGIN HOSTIE\n192.168.1.10 dev.local\n# END HOSTIE`,
      );
    });

    test("empty original returns just the new block", () => {
      const newBlock = `# BEGIN HOSTIE\n192.168.1.10 dev.local\n# END HOSTIE`;

      const result = replaceManagedBlock("", newBlock);

      expect(result).toBe(newBlock);
    });

    test("malformed original (lone BEGIN) is treated as first-time insertion", () => {
      const original = `127.0.0.1 localhost\n# BEGIN HOSTIE\nstray line`;
      const newBlock = `# BEGIN HOSTIE\n10.0.0.1 svc\n# END HOSTIE`;

      const result = replaceManagedBlock(original, newBlock);

      // Outside content is preserved verbatim; new block is appended.
      expect(result.startsWith(original)).toBe(true);
      expect(result.endsWith(newBlock)).toBe(true);
      expect(result).toContain("stray line");
    });

    test("malformed original (lone END) is treated as first-time insertion", () => {
      const original = `127.0.0.1 localhost\n# END HOSTIE\n`;
      const newBlock = `# BEGIN HOSTIE\n10.0.0.1 svc\n# END HOSTIE`;

      const result = replaceManagedBlock(original, newBlock);

      expect(result).toContain("127.0.0.1 localhost");
      expect(result).toContain("# END HOSTIE");
      expect(result.endsWith(newBlock)).toBe(true);
    });

    test("preserves blank lines around the managed block", () => {
      const original = `127.0.0.1 localhost

# BEGIN HOSTIE
192.168.1.10 dev.local
# END HOSTIE

::1 localhost`;
      const newBlock = `# BEGIN HOSTIE\n192.168.1.20 new.local\n# END HOSTIE`;

      const result = replaceManagedBlock(original, newBlock);

      expect(result).toBe(`127.0.0.1 localhost

# BEGIN HOSTIE
192.168.1.20 new.local
# END HOSTIE

::1 localhost`);
    });

    test("does not touch content outside the managed block", () => {
      const original = `### custom header ###
127.0.0.1 localhost
10.0.0.5 internal.svc
# BEGIN HOSTIE
old.entry
# END HOSTIE
# trailing comment
::1 localhost`;
      const newBlock = `# BEGIN HOSTIE\nnew.entry\n# END HOSTIE`;

      const result = replaceManagedBlock(original, newBlock);

      expect(result).toContain("### custom header ###");
      expect(result).toContain("10.0.0.5 internal.svc");
      expect(result).toContain("# trailing comment");
      expect(result).toContain("new.entry");
      expect(result).not.toContain("old.entry");
    });

    test("replacement is idempotent when applied twice", () => {
      const original = `127.0.0.1 localhost
# BEGIN HOSTIE
old
# END HOSTIE
::1 localhost`;
      const newBlock = `# BEGIN HOSTIE\nnew\n# END HOSTIE`;

      const once = replaceManagedBlock(original, newBlock);
      const twice = replaceManagedBlock(once, newBlock);

      expect(twice).toBe(once);
    });
  });
});
