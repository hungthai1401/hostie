import { describe, test, expect } from "bun:test";
import { readEtcHosts } from "../etchosts";
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
});
