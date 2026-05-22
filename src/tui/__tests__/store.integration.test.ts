/**
 * Integration test for store initialization with ~/.hosts
 */
import { describe, test, expect, beforeAll, afterAll } from "bun:test";
import { initializeStore, useAppStore } from "../store";
import { writeHostsFile } from "../../core/file-io";
import type { HostsFile } from "../../domain/types";
import { existsSync, unlinkSync } from "fs";
import { homedir } from "os";
import { join } from "path";

describe("Store initialization", () => {
  const testHostsPath = join(homedir(), ".hosts-test-store");
  const originalHostsPath = join(homedir(), ".hosts");
  let originalHostsBackup: string | null = null;

  beforeAll(async () => {
    // Backup original ~/.hosts if it exists
    if (existsSync(originalHostsPath)) {
      const file = Bun.file(originalHostsPath);
      originalHostsBackup = await file.text();
    }

    // Create test hosts file
    const testFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "test-group",
          entries: [
            {
              id: "test-entry-1",
              ip: "127.0.0.1",
              hostname: "test.local",
              aliases: ["test"],
              enabled: true,
              comment: "Test entry",
            },
          ],
          groups: [],
        },
      ],
    };

    await writeHostsFile(originalHostsPath, testFile);
  });

  afterAll(async () => {
    // Restore original ~/.hosts
    if (originalHostsBackup !== null) {
      await Bun.write(originalHostsPath, originalHostsBackup);
    } else if (existsSync(originalHostsPath)) {
      unlinkSync(originalHostsPath);
    }
  });

  test("initializeStore loads ~/.hosts into state", async () => {
    await initializeStore();
    
    const state = useAppStore.getState();
    
    expect(state.hostsFile.version).toBe(1);
    expect(state.hostsFile.groups).toHaveLength(1);
    expect(state.hostsFile.groups[0].name).toBe("test-group");
    expect(state.hostsFile.groups[0].entries).toHaveLength(1);
    expect(state.hostsFile.groups[0].entries[0].hostname).toBe("test.local");
    expect(state.dirty).toBe(false);
  });
});
