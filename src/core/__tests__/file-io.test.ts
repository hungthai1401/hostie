import { describe, test, expect, beforeEach, afterEach } from "bun:test";
import { readHostsFile, writeHostsFile } from "../file-io";
import { unlinkSync, existsSync, readFileSync, mkdirSync, rmdirSync } from "fs";
import { homedir } from "os";
import { join } from "path";

const TEST_HOSTS_FILE = join(homedir(), ".hosts-test");

describe("file-io", () => {
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

  describe("readHostsFile", () => {
    test("creates default file if missing", async () => {
      const result = await readHostsFile(TEST_HOSTS_FILE);

      expect(result).toEqual({
        version: 1,
        groups: [],
      });

      // Verify file was created
      expect(existsSync(TEST_HOSTS_FILE)).toBe(true);

      // Verify file content
      const content = readFileSync(TEST_HOSTS_FILE, "utf-8");
      expect(content).toContain("version: 1");
      expect(content).toContain("groups: []");
    });

    test("reads existing YAML file", async () => {
      // Create a test file
      const testContent = `version: 1
groups:
  - name: test
    entries:
      - id: 01J5ABC123
        ip: 192.168.1.10
        hostname: test.local
        aliases: []
        enabled: true
    groups: []
`;
      await Bun.write(TEST_HOSTS_FILE, testContent);

      const result = await readHostsFile(TEST_HOSTS_FILE);

      expect(result.version).toBe(1);
      expect(result.groups).toHaveLength(1);
      expect(result.groups[0].name).toBe("test");
      expect(result.groups[0].entries).toHaveLength(1);
      expect(result.groups[0].entries[0].hostname).toBe("test.local");
    });

    test("expands ~ to home directory", async () => {
      const result = await readHostsFile("~/.hosts-test");

      expect(result).toEqual({
        version: 1,
        groups: [],
      });

      // Verify file was created in home directory
      expect(existsSync(TEST_HOSTS_FILE)).toBe(true);
    });

    test("handles nested groups", async () => {
      const testContent = `version: 1
groups:
  - name: work
    entries: []
    groups:
      - name: prod
        entries:
          - id: 01J5XYZ789
            ip: 10.0.2.10
            hostname: db.prod.work
            aliases: []
            enabled: true
        groups: []
`;
      await Bun.write(TEST_HOSTS_FILE, testContent);

      const result = await readHostsFile(TEST_HOSTS_FILE);

      expect(result.groups[0].name).toBe("work");
      expect(result.groups[0].groups).toHaveLength(1);
      expect(result.groups[0].groups[0].name).toBe("prod");
      expect(result.groups[0].groups[0].entries[0].hostname).toBe("db.prod.work");
    });
  });

  describe("writeHostsFile", () => {
    test("writes HostsFile to YAML", async () => {
      const hostsFile = {
        version: 1 as const,
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

      await writeHostsFile(TEST_HOSTS_FILE, hostsFile);

      // Verify file exists
      expect(existsSync(TEST_HOSTS_FILE)).toBe(true);

      // Verify content
      const content = readFileSync(TEST_HOSTS_FILE, "utf-8");
      expect(content).toContain("version: 1");
      expect(content).toContain("name: test");
      expect(content).toContain("hostname: test.local");
      expect(content).toContain("ip: 192.168.1.10");
    });

    test("expands ~ to home directory", async () => {
      const hostsFile = {
        version: 1 as const,
        groups: [],
      };

      await writeHostsFile("~/.hosts-test", hostsFile);

      // Verify file was created in home directory
      expect(existsSync(TEST_HOSTS_FILE)).toBe(true);
    });

    test("overwrites existing file", async () => {
      // Create initial file
      const initial = {
        version: 1 as const,
        groups: [
          {
            name: "old",
            entries: [],
            groups: [],
          },
        ],
      };
      await writeHostsFile(TEST_HOSTS_FILE, initial);

      // Overwrite with new content
      const updated = {
        version: 1 as const,
        groups: [
          {
            name: "new",
            entries: [],
            groups: [],
          },
        ],
      };
      await writeHostsFile(TEST_HOSTS_FILE, updated);

      // Verify new content
      const content = readFileSync(TEST_HOSTS_FILE, "utf-8");
      expect(content).toContain("name: new");
      expect(content).not.toContain("name: old");
    });

    test("creates parent directory if missing", async () => {
      const testDir = join(homedir(), ".hostie-test-dir");
      const testFile = join(testDir, "hosts");

      // Clean up if exists
      if (existsSync(testFile)) {
        unlinkSync(testFile);
      }
      if (existsSync(testDir)) {
        rmdirSync(testDir);
      }

      const hostsFile = {
        version: 1 as const,
        groups: [],
      };

      await writeHostsFile(testFile, hostsFile);

      expect(existsSync(testFile)).toBe(true);

      // Clean up
      unlinkSync(testFile);
      rmdirSync(testDir);
    });

    test("sets file permissions to 644", async () => {
      const hostsFile = {
        version: 1 as const,
        groups: [],
      };

      await writeHostsFile(TEST_HOSTS_FILE, hostsFile);

      // Check file permissions (on Unix systems)
      const stats = Bun.file(TEST_HOSTS_FILE).stat();
      const mode = (await stats).mode & 0o777;
      expect(mode).toBe(0o644);
    });
  });

  describe("round-trip", () => {
    test("read after write returns same data", async () => {
      const original = {
        version: 1 as const,
        groups: [
          {
            name: "work",
            entries: [
              {
                id: "01J5ABC123",
                ip: "10.0.1.5",
                hostname: "jira.work",
                aliases: ["jira"],
                enabled: true,
                comment: "Work JIRA",
              },
            ],
            groups: [
              {
                name: "prod",
                entries: [
                  {
                    id: "01J5XYZ789",
                    ip: "10.0.2.10",
                    hostname: "db.prod.work",
                    aliases: [],
                    enabled: true,
                  },
                  {
                    id: "01J5XYZ790",
                    ip: "10.0.2.11",
                    hostname: "db-replica.prod.work",
                    aliases: [],
                    enabled: false,
                  },
                ],
                groups: [],
              },
            ],
          },
        ],
      };

      await writeHostsFile(TEST_HOSTS_FILE, original);
      const result = await readHostsFile(TEST_HOSTS_FILE);

      expect(result).toEqual(original);
    });
  });
});
