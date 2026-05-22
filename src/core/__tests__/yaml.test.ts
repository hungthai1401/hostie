import { describe, test, expect } from "bun:test";
import { deserializeHostsFile, serializeHostsFile } from "../yaml";
import type { HostsFile } from "../../domain/types";

describe("deserializeHostsFile", () => {
  test("deserializes valid YAML to HostsFile", () => {
    const yaml = `version: 1
groups:
  - name: dev
    entries:
      - id: 01HZXYZ123
        ip: 127.0.0.1
        hostname: localhost.dev
        aliases: []
        enabled: true
    groups: []
`;

    const result = deserializeHostsFile(yaml);

    expect(result.version).toBe(1);
    expect(result.groups).toHaveLength(1);
    expect(result.groups[0].name).toBe("dev");
    expect(result.groups[0].entries).toHaveLength(1);
    expect(result.groups[0].entries[0].hostname).toBe("localhost.dev");
  });

  test("deserializes nested groups", () => {
    const yaml = `version: 1
groups:
  - name: work
    entries: []
    groups:
      - name: prod
        entries:
          - id: 01HZXYZ456
            ip: 10.0.0.1
            hostname: api.prod
            aliases: ["api"]
            enabled: true
        groups: []
`;

    const result = deserializeHostsFile(yaml);

    expect(result.groups[0].name).toBe("work");
    expect(result.groups[0].groups).toHaveLength(1);
    expect(result.groups[0].groups[0].name).toBe("prod");
    expect(result.groups[0].groups[0].entries[0].hostname).toBe("api.prod");
  });

  test("deserializes entries with optional fields", () => {
    const yaml = `version: 1
groups:
  - name: test
    entries:
      - id: 01HZXYZ789
        ip: 192.168.1.1
        hostname: test.local
        aliases: ["test", "testing"]
        enabled: false
        comment: "Disabled for testing"
    groups: []
`;

    const result = deserializeHostsFile(yaml);

    const entry = result.groups[0].entries[0];
    expect(entry.enabled).toBe(false);
    expect(entry.comment).toBe("Disabled for testing");
    expect(entry.aliases).toEqual(["test", "testing"]);
  });

  test("throws error for invalid YAML", () => {
    const invalidYaml = `version: 1
groups: [
  - name: broken
`;

    expect(() => deserializeHostsFile(invalidYaml)).toThrow();
  });

  test("throws error for missing version field", () => {
    const yaml = `groups:
  - name: test
    entries: []
    groups: []
`;

    expect(() => deserializeHostsFile(yaml)).toThrow(
      "Invalid ~/.hosts: missing version field"
    );
  });

  test("throws error for unsupported version", () => {
    const yaml = `version: 2
groups: []
`;

    expect(() => deserializeHostsFile(yaml)).toThrow(
      "Unsupported ~/.hosts version 2. This tool supports version 1. Please upgrade hostie."
    );
  });

  test("throws error for version less than 1", () => {
    const yaml = `version: 0
groups: []
`;

    expect(() => deserializeHostsFile(yaml)).toThrow(
      "Invalid ~/.hosts: missing version field"
    );
  });

  test("throws error for non-numeric version", () => {
    const yaml = `version: "1"
groups: []
`;

    expect(() => deserializeHostsFile(yaml)).toThrow(
      "Invalid ~/.hosts: missing version field"
    );
  });

  test("throws error for invalid schema", () => {
    const yaml = `version: 1
groups: "not an array"
`;

    expect(() => deserializeHostsFile(yaml)).toThrow();
  });
});

describe("serializeHostsFile", () => {
  test("serializes HostsFile to valid YAML", () => {
    const hostsFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "dev",
          entries: [
            {
              id: "01HZXYZ123",
              ip: "127.0.0.1",
              hostname: "localhost.dev",
              aliases: [],
              enabled: true,
            },
          ],
          groups: [],
        },
      ],
    };

    const yaml = serializeHostsFile(hostsFile);

    expect(yaml).toContain("version: 1");
    expect(yaml).toContain("groups:");
    expect(yaml).toContain("name: dev");
    expect(yaml).toContain("hostname: localhost.dev");
  });

  test("serializes nested groups", () => {
    const hostsFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "work",
          entries: [],
          groups: [
            {
              name: "prod",
              entries: [
                {
                  id: "01HZXYZ456",
                  ip: "10.0.0.1",
                  hostname: "api.prod",
                  aliases: ["api"],
                  enabled: true,
                },
              ],
              groups: [],
            },
          ],
        },
      ],
    };

    const yaml = serializeHostsFile(hostsFile);

    expect(yaml).toContain("name: work");
    expect(yaml).toContain("name: prod");
    expect(yaml).toContain("hostname: api.prod");
  });

  test("serializes entries with optional fields", () => {
    const hostsFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "test",
          entries: [
            {
              id: "01HZXYZ789",
              ip: "192.168.1.1",
              hostname: "test.local",
              aliases: ["test", "testing"],
              enabled: false,
              comment: "Disabled for testing",
            },
          ],
          groups: [],
        },
      ],
    };

    const yaml = serializeHostsFile(hostsFile);

    expect(yaml).toContain("enabled: false");
    expect(yaml).toContain("comment: Disabled for testing");
    expect(yaml).toContain("aliases:");
  });

  test("round-trip: serialize → deserialize → serialize produces same output", () => {
    const original: HostsFile = {
      version: 1,
      groups: [
        {
          name: "dev",
          entries: [
            {
              id: "01HZXYZ123",
              ip: "127.0.0.1",
              hostname: "localhost.dev",
              aliases: ["local"],
              enabled: true,
              comment: "Development host",
            },
          ],
          groups: [
            {
              name: "nested",
              entries: [],
              groups: [],
            },
          ],
        },
      ],
    };

    const yaml1 = serializeHostsFile(original);
    const deserialized = deserializeHostsFile(yaml1);
    const yaml2 = serializeHostsFile(deserialized);

    expect(yaml1).toBe(yaml2);
  });

  test("uses 2-space indentation", () => {
    const hostsFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "test",
          entries: [],
          groups: [],
        },
      ],
    };

    const yaml = serializeHostsFile(hostsFile);

    // Check that indentation is 2 spaces (not 4 or tabs)
    expect(yaml).toMatch(/\n  - name: test/);
  });
});
