/**
 * Tests for group create command
 */

import { describe, test, expect, beforeEach, afterEach } from "bun:test";
import { groupCreateCommand } from "../group";
import { readHostsFile, writeHostsFile } from "../../../core/file-io";
import { existsSync, unlinkSync } from "fs";
import { homedir } from "os";
import { join } from "path";

const TEST_HOSTS_FILE = join(homedir(), ".hosts-test-group");

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

describe("groupCreateCommand", () => {
  test("creates group successfully with valid kebab-case name", async () => {
    const exitCode = await groupCreateCommand("my-group", {
      hostsFile: TEST_HOSTS_FILE,
    });

    expect(exitCode).toBe(0);

    const hostsFile = await readHostsFile(TEST_HOSTS_FILE);
    expect(hostsFile.groups.length).toBe(1);
    expect(hostsFile.groups[0].name).toBe("my-group");
    expect(hostsFile.groups[0].entries).toEqual([]);
    expect(hostsFile.groups[0].groups).toEqual([]);
  });

  test("validates kebab-case name - rejects uppercase", async () => {
    const exitCode = await groupCreateCommand("MyGroup", {
      hostsFile: TEST_HOSTS_FILE,
    });

    expect(exitCode).toBe(1);
  });

  test("validates kebab-case name - rejects underscores", async () => {
    const exitCode = await groupCreateCommand("my_group", {
      hostsFile: TEST_HOSTS_FILE,
    });

    expect(exitCode).toBe(1);
  });

  test("validates kebab-case name - rejects spaces", async () => {
    const exitCode = await groupCreateCommand("my group", {
      hostsFile: TEST_HOSTS_FILE,
    });

    expect(exitCode).toBe(1);
  });

  test("validates kebab-case name - rejects slashes", async () => {
    const exitCode = await groupCreateCommand("my/group", {
      hostsFile: TEST_HOSTS_FILE,
    });

    expect(exitCode).toBe(1);
  });

  test("validates kebab-case name - accepts single word", async () => {
    const exitCode = await groupCreateCommand("work", {
      hostsFile: TEST_HOSTS_FILE,
    });

    expect(exitCode).toBe(0);

    const hostsFile = await readHostsFile(TEST_HOSTS_FILE);
    expect(hostsFile.groups.length).toBe(1);
    expect(hostsFile.groups[0].name).toBe("work");
  });

  test("validates kebab-case name - accepts numbers", async () => {
    const exitCode = await groupCreateCommand("server-123", {
      hostsFile: TEST_HOSTS_FILE,
    });

    expect(exitCode).toBe(0);

    const hostsFile = await readHostsFile(TEST_HOSTS_FILE);
    expect(hostsFile.groups[0].name).toBe("server-123");
  });

  test("handles duplicate group name", async () => {
    // Create first group
    await groupCreateCommand("my-group", {
      hostsFile: TEST_HOSTS_FILE,
    });

    // Try to create duplicate
    const exitCode = await groupCreateCommand("my-group", {
      hostsFile: TEST_HOSTS_FILE,
    });

    expect(exitCode).toBe(1);
  });

  test("creates nested group with --parent flag", async () => {
    // Create parent group first
    await groupCreateCommand("work", {
      hostsFile: TEST_HOSTS_FILE,
    });

    // Create nested group
    const exitCode = await groupCreateCommand("prod", {
      parent: "work",
      hostsFile: TEST_HOSTS_FILE,
    });

    expect(exitCode).toBe(0);

    const hostsFile = await readHostsFile(TEST_HOSTS_FILE);
    const workGroup = hostsFile.groups.find(g => g.name === "work");
    expect(workGroup).toBeDefined();
    expect(workGroup?.groups.length).toBe(1);
    expect(workGroup?.groups[0].name).toBe("prod");
  });

  test("creates deeply nested group with --parent path", async () => {
    // Create parent hierarchy
    await groupCreateCommand("work", {
      hostsFile: TEST_HOSTS_FILE,
    });
    await groupCreateCommand("prod", {
      parent: "work",
      hostsFile: TEST_HOSTS_FILE,
    });

    // Create deeply nested group
    const exitCode = await groupCreateCommand("servers", {
      parent: "work/prod",
      hostsFile: TEST_HOSTS_FILE,
    });

    expect(exitCode).toBe(0);

    const hostsFile = await readHostsFile(TEST_HOSTS_FILE);
    const workGroup = hostsFile.groups.find(g => g.name === "work");
    const prodGroup = workGroup?.groups.find(g => g.name === "prod");
    expect(prodGroup).toBeDefined();
    expect(prodGroup?.groups.length).toBe(1);
    expect(prodGroup?.groups[0].name).toBe("servers");
  });

  test("fails when parent group does not exist", async () => {
    const exitCode = await groupCreateCommand("prod", {
      parent: "nonexistent",
      hostsFile: TEST_HOSTS_FILE,
    });

    expect(exitCode).toBe(1);
  });

  test("returns exit code 2 on I/O error", async () => {
    const exitCode = await groupCreateCommand("my-group", {
      hostsFile: "/nonexistent/path/hosts",
    });

    expect(exitCode).toBe(2);
  });
});
