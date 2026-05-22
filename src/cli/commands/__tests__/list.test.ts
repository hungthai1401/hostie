/**
 * Tests for list command
 */

import { describe, test, expect, beforeEach, afterEach } from "bun:test";
import { listCommand } from "../list";
import { writeHostsFile } from "../../../core/file-io";
import type { HostsFile } from "../../../domain/types";
import { unlinkSync, existsSync } from "fs";
import { homedir } from "os";
import { join } from "path";

const TEST_HOSTS_FILE = join(homedir(), ".hosts-test-list");

describe("listCommand", () => {
  beforeEach(async () => {
    // Clean up test file if it exists
    if (existsSync(TEST_HOSTS_FILE)) {
      unlinkSync(TEST_HOSTS_FILE);
    }
  });

  afterEach(() => {
    // Clean up test file
    if (existsSync(TEST_HOSTS_FILE)) {
      unlinkSync(TEST_HOSTS_FILE);
    }
  });

  test("lists entries in human-readable format", async () => {
    // Arrange
    const hostsFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "work",
          entries: [
            {
              id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
              ip: "192.168.1.100",
              hostname: "api.work",
              aliases: ["api"],
              enabled: true,
              comment: "Work API server",
            },
          ],
          groups: [
            {
              name: "prod",
              entries: [
                {
                  id: "01ARZ3NDEKTSV4RRFFQ69G5FAW",
                  ip: "10.0.0.50",
                  hostname: "db.prod.work",
                  aliases: [],
                  enabled: false,
                },
              ],
              groups: [],
            },
          ],
        },
        {
          name: "",
          entries: [
            {
              id: "01ARZ3NDEKTSV4RRFFQ69G5FAX",
              ip: "127.0.0.1",
              hostname: "localhost",
              aliases: ["local"],
              enabled: true,
            },
          ],
          groups: [],
        },
      ],
    };
    await writeHostsFile(TEST_HOSTS_FILE, hostsFile);

    // Act
    const exitCode = await listCommand({ hostsFile: TEST_HOSTS_FILE });

    // Assert
    expect(exitCode).toBe(0);
  });

  test("lists entries in JSON format with --json flag", async () => {
    // Arrange
    const hostsFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "work",
          entries: [
            {
              id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
              ip: "192.168.1.100",
              hostname: "api.work",
              aliases: ["api"],
              enabled: true,
              comment: "Work API server",
            },
          ],
          groups: [],
        },
      ],
    };
    await writeHostsFile(TEST_HOSTS_FILE, hostsFile);

    // Capture stdout
    const originalLog = console.log;
    let output = "";
    console.log = (msg: string) => {
      output += msg;
    };

    // Act
    const exitCode = await listCommand({ json: true, hostsFile: TEST_HOSTS_FILE });

    // Restore console.log
    console.log = originalLog;

    // Assert
    expect(exitCode).toBe(0);
    const parsed = JSON.parse(output);
    expect(Array.isArray(parsed)).toBe(true);
    expect(parsed.length).toBe(1);
    expect(parsed[0].hostname).toBe("api.work");
    expect(parsed[0].group).toBe("work");
  });

  test("returns exit code 2 on I/O error", async () => {
    // Act - try to read non-existent file
    const exitCode = await listCommand({ hostsFile: "/nonexistent/path/.hosts" });

    // Assert
    expect(exitCode).toBe(2);
  });

  test("handles empty hosts file", async () => {
    // Arrange
    const hostsFile: HostsFile = {
      version: 1,
      groups: [],
    };
    await writeHostsFile(TEST_HOSTS_FILE, hostsFile);

    // Act
    const exitCode = await listCommand({ hostsFile: TEST_HOSTS_FILE });

    // Assert
    expect(exitCode).toBe(0);
  });

  test("includes group path in output", async () => {
    // Arrange
    const hostsFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "work",
          entries: [],
          groups: [
            {
              name: "staging",
              entries: [
                {
                  id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
                  ip: "192.168.1.100",
                  hostname: "api.staging.work",
                  aliases: [],
                  enabled: true,
                },
              ],
              groups: [],
            },
          ],
        },
      ],
    };
    await writeHostsFile(TEST_HOSTS_FILE, hostsFile);

    // Capture stdout
    const originalLog = console.log;
    let output = "";
    console.log = (msg: string) => {
      output += msg;
    };

    // Act
    const exitCode = await listCommand({ json: true, hostsFile: TEST_HOSTS_FILE });

    // Restore console.log
    console.log = originalLog;

    // Assert
    expect(exitCode).toBe(0);
    const parsed = JSON.parse(output);
    expect(parsed[0].group).toBe("work/staging");
  });
});
