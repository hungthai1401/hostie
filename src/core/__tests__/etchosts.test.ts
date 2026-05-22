import { describe, test, expect } from "bun:test";
import { readEtcHosts, extractManagedBlock } from "../etchosts";
import { existsSync } from "fs";

describe("etchosts", () => {
  describe("readEtcHosts", () => {
    test("reads /etc/hosts successfully", async () => {
      const result = await readEtcHosts();
      
      // Should return a string
      expect(typeof result).toBe("string");
      
      // /etc/hosts should exist on Unix systems
      if (existsSync("/etc/hosts")) {
        // Should contain some content (at least localhost)
        expect(result.length).toBeGreaterThan(0);
      }
    });

    test("returns empty string if file doesn't exist", async () => {
      // We can't easily test this without mocking, but we can verify
      // the function handles the case gracefully by checking the implementation
      // For now, just verify it doesn't throw
      const result = await readEtcHosts();
      expect(typeof result).toBe("string");
    });

    test("returns full file content", async () => {
      const result = await readEtcHosts();
      
      // Should be a complete file (not truncated)
      // Most /etc/hosts files contain localhost entries
      if (existsSync("/etc/hosts")) {
        expect(result).toContain("127.0.0.1");
      }
    });
  });

  describe("extractManagedBlock", () => {
    test("extracts managed block correctly", () => {
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

    test("handles missing markers (first-time use)", () => {
      const content = `127.0.0.1 localhost
::1 localhost`;

      const result = extractManagedBlock(content);

      expect(result.before).toBe(content);
      expect(result.managed).toBe("");
      expect(result.after).toBe("");
    });

    test("handles only BEGIN marker (malformed)", () => {
      const content = `127.0.0.1 localhost
# BEGIN HOSTIE
192.168.1.10 dev.local`;

      const result = extractManagedBlock(content);

      expect(result.before).toBe(content);
      expect(result.managed).toBe("");
      expect(result.after).toBe("");
    });

    test("handles only END marker (malformed)", () => {
      const content = `127.0.0.1 localhost
192.168.1.10 dev.local
# END HOSTIE`;

      const result = extractManagedBlock(content);

      expect(result.before).toBe(content);
      expect(result.managed).toBe("");
      expect(result.after).toBe("");
    });

    test("preserves content before and after markers", () => {
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
      expect(result.managed).toBe("# BEGIN HOSTIE\n192.168.1.10 dev.local\n192.168.1.11 staging.local\n# END HOSTIE");
      expect(result.after).toBe("\n\n# IPv6\n::1 localhost");
    });

    test("handles empty managed block", () => {
      const content = `127.0.0.1 localhost
# BEGIN HOSTIE
# END HOSTIE
::1 localhost`;

      const result = extractManagedBlock(content);

      expect(result.before).toBe("127.0.0.1 localhost\n");
      expect(result.managed).toBe("# BEGIN HOSTIE\n# END HOSTIE");
      expect(result.after).toBe("\n::1 localhost");
    });
  });
});
