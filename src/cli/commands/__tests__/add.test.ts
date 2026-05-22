/**
 * Tests for add command
 */

import { describe, test, expect, beforeEach, afterEach } from "bun:test";
import { addCommand } from "../add";
import { readHostsFile, writeHostsFile } from "../../../core/file-io";
import { existsSync, unlinkSync } from "fs";
import { homedir } from "os";
import { join } from "path";

const TEST_HOSTS_FILE = join(homedir(), ".hosts-test-add");

beforeEach(async () => {
  // Clean up test file if it exists
  if (existsSync(TEST_HOSTS_FILE)) {
    unlinkSync(TEST_HOSTS_FILE);
  }
  
  // Create empty hosts file
  await writeHostsFile(TEST_HOSTS_FILE, {
    version: 1,
    groups: [],
  });
});

afterEach(() => {
  // Clean up test file
  if (existsSync(TEST_HOSTS_FILE)) {
    unlinkSync(TEST_HOSTS_FILE);
  }
});

describe("addCommand", () => {
  test("adds entry successfully with valid IP and hostname", async () => {
    const exitCode = await addCommand("192.168.1.10", "test.local", [], {
      hostsFile: TEST_HOSTS_FILE,
    });

    expect(exitCode).toBe(0);

    const hostsFile = await readHostsFile(TEST_HOSTS_FILE);
    expect(hostsFile.groups.length).toBe(1);
    
    // Entry should be in a synthetic root group with empty name
    const rootGroup = hostsFile.groups.find(g => g.name === "");
    expect(rootGroup).toBeDefined();
    expect(rootGroup?.entries.length).toBe(1);
    expect(rootGroup?.entries[0].hostname).toBe("test.local");
    expect(rootGroup?.entries[0].ip).toBe("192.168.1.10");
  });

  test("validates IP address", async () => {
    const exitCode = await addCommand("invalid-ip", "test.local", [], {
      hostsFile: TEST_HOSTS_FILE,
    });

    expect(exitCode).toBe(1);
  });

  test("validates hostname", async () => {
    const exitCode = await addCommand("192.168.1.10", "invalid..hostname", [], {
      hostsFile: TEST_HOSTS_FILE,
    });

    expect(exitCode).toBe(1);
  });

  test("adds entry with aliases", async () => {
    const exitCode = await addCommand("192.168.1.10", "test.local", ["alias1", "alias2"], {
      hostsFile: TEST_HOSTS_FILE,
    });

    expect(exitCode).toBe(0);

    const hostsFile = await readHostsFile(TEST_HOSTS_FILE);
    
    const rootGroup = hostsFile.groups.find(g => g.name === "");
    expect(rootGroup).toBeDefined();
    expect(rootGroup?.entries.length).toBe(1);
    expect(rootGroup?.entries[0].aliases).toEqual(["alias1", "alias2"]);
  });

  test("adds entry to specified group", async () => {
    const exitCode = await addCommand("192.168.1.10", "test.local", [], {
      group: "work/prod",
      hostsFile: TEST_HOSTS_FILE,
    });

    expect(exitCode).toBe(0);

    const hostsFile = await readHostsFile(TEST_HOSTS_FILE);
    expect(hostsFile.groups.length).toBeGreaterThan(0);
    
    // Find the work group
    const workGroup = hostsFile.groups.find(g => g.name === "work");
    expect(workGroup).toBeDefined();
    expect(workGroup?.groups.length).toBeGreaterThan(0);
    
    const prodGroup = workGroup?.groups.find(g => g.name === "prod");
    expect(prodGroup).toBeDefined();
    expect(prodGroup?.entries.length).toBe(1);
    expect(prodGroup?.entries[0].hostname).toBe("test.local");
  });

  test("handles duplicate hostname", async () => {
    // Add first entry
    await addCommand("192.168.1.10", "test.local", [], {
      hostsFile: TEST_HOSTS_FILE,
    });

    // Try to add duplicate
    const exitCode = await addCommand("192.168.1.20", "test.local", [], {
      hostsFile: TEST_HOSTS_FILE,
    });

    expect(exitCode).toBe(1);
  });

  test("returns exit code 2 on I/O error", async () => {
    const exitCode = await addCommand("192.168.1.10", "test.local", [], {
      hostsFile: "/nonexistent/path/hosts",
    });

    expect(exitCode).toBe(2);
  });
});
