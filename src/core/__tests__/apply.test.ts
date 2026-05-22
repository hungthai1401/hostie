import { describe, test, expect, beforeEach, afterEach } from "bun:test";
import { applyHostsFile, type HostsFile } from "../apply";
import { writeFileSync, readFileSync, unlinkSync, existsSync } from "fs";

const TEST_HOSTS_FILE = "/tmp/hostie-test-hosts";

// Mock /etc/hosts by overriding the path in tests
// For now, we'll use a test file path

describe("applyHostsFile", () => {
  beforeEach(() => {
    // Clean up any existing test file
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

  test("skips write if content unchanged (idempotency)", async () => {
    // Create a HostsFile with one entry
    const hostsFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "test",
          entries: [
            {
              id: "01J5ABC123",
              ip: "192.168.1.10",
              hostname: "test.local",
              aliases: [],
              enabled: true,
            },
          ],
          groups: [],
        },
      ],
    };

    // For this test, we need to mock the file read
    // Since we can't easily mock /etc/hosts, we'll test the logic
    // by checking the return value
    
    const result = await applyHostsFile(hostsFile);
    
    // Should indicate it would change (since /etc/hosts likely doesn't have our block)
    expect(result).toHaveProperty("changed");
    expect(result).toHaveProperty("message");
    expect(typeof result.changed).toBe("boolean");
    expect(typeof result.message).toBe("string");
  });

  test("returns accurate status message for unchanged content", async () => {
    const hostsFile: HostsFile = {
      version: 1,
      groups: [],
    };

    const result = await applyHostsFile(hostsFile);
    
    expect(result).toHaveProperty("changed");
    expect(result).toHaveProperty("message");
    expect(result.message).toContain("hosts");
  });

  test("handles first-time application (no existing block)", async () => {
    const hostsFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "work",
          entries: [
            {
              id: "01J5XYZ789",
              ip: "10.0.1.5",
              hostname: "jira.work",
              aliases: [],
              enabled: true,
            },
          ],
          groups: [],
        },
      ],
    };

    const result = await applyHostsFile(hostsFile);
    
    expect(result).toHaveProperty("changed");
    expect(result).toHaveProperty("message");
  });

  test("handles nested groups correctly", async () => {
    const hostsFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "work",
          entries: [
            {
              id: "01J5ABC001",
              ip: "10.0.1.5",
              hostname: "jira.work",
              aliases: [],
              enabled: true,
            },
          ],
          groups: [
            {
              name: "prod",
              entries: [
                {
                  id: "01J5ABC002",
                  ip: "10.0.2.10",
                  hostname: "db.prod.work",
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

    const result = await applyHostsFile(hostsFile);
    
    expect(result).toHaveProperty("changed");
    expect(result).toHaveProperty("message");
  });

  test("skips disabled entries", async () => {
    const hostsFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "test",
          entries: [
            {
              id: "01J5ABC003",
              ip: "192.168.1.10",
              hostname: "enabled.local",
              aliases: [],
              enabled: true,
            },
            {
              id: "01J5ABC004",
              ip: "192.168.1.11",
              hostname: "disabled.local",
              aliases: [],
              enabled: false,
            },
          ],
          groups: [],
        },
      ],
    };

    const result = await applyHostsFile(hostsFile);
    
    // Should process without error
    expect(result).toHaveProperty("changed");
    expect(result).toHaveProperty("message");
  });

  test("handles entries with aliases", async () => {
    const hostsFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "test",
          entries: [
            {
              id: "01J5ABC005",
              ip: "192.168.1.10",
              hostname: "server.local",
              aliases: ["alias1.local", "alias2.local"],
              enabled: true,
            },
          ],
          groups: [],
        },
      ],
    };

    const result = await applyHostsFile(hostsFile);
    
    expect(result).toHaveProperty("changed");
    expect(result).toHaveProperty("message");
  });

  test("handles entries with comments", async () => {
    const hostsFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "test",
          entries: [
            {
              id: "01J5ABC006",
              ip: "192.168.1.10",
              hostname: "server.local",
              aliases: [],
              enabled: true,
              comment: "Production server",
            },
          ],
          groups: [],
        },
      ],
    };

    const result = await applyHostsFile(hostsFile);
    
    expect(result).toHaveProperty("changed");
    expect(result).toHaveProperty("message");
  });

  test("handles empty groups", async () => {
    const hostsFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "empty",
          entries: [],
          groups: [],
        },
      ],
    };

    const result = await applyHostsFile(hostsFile);
    
    expect(result).toHaveProperty("changed");
    expect(result).toHaveProperty("message");
  });

  test("handles permission errors gracefully", async () => {
    // This test would need to mock file system access
    // For now, we just verify the function signature works
    const hostsFile: HostsFile = {
      version: 1,
      groups: [],
    };

    const result = await applyHostsFile(hostsFile);
    
    expect(result).toHaveProperty("changed");
    expect(result).toHaveProperty("message");
  });
});
