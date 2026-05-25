/**
 * Unit tests for YAML I/O (serialize, deserialize, round-trip, schema validation)
 *
 * Covers:
 *   - serializeHostsFile
 *   - deserializeHostsFile
 *   - round-trip (serialize → deserialize → serialize)
 *   - schema version validation
 *   - error handling for invalid YAML / invalid schema
 */

import { describe, test, expect } from "bun:test";
import {
  deserializeHostsFile,
  serializeHostsFile,
} from "../../src/core/yaml";
import type { HostsFile } from "../../src/domain/types";

// ---------------------------------------------------------------------------
// Shared fixtures
// ---------------------------------------------------------------------------

const simpleHostsFile: HostsFile = {
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

const nestedHostsFile: HostsFile = {
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

const complexHostsFile: HostsFile = {
  version: 1,
  groups: [
    {
      name: "dev",
      entries: [
        {
          id: "01HZXYZ123",
          ip: "127.0.0.1",
          hostname: "localhost.dev",
          aliases: ["local", "loopback"],
          enabled: true,
          comment: "Development host",
        },
        {
          id: "01HZXYZ124",
          ip: "127.0.0.2",
          hostname: "disabled.dev",
          aliases: [],
          enabled: false,
          comment: "Temporarily disabled",
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
    {
      name: "staging",
      entries: [
        {
          id: "01HZXYZ789",
          ip: "192.168.1.10",
          hostname: "stage.example.com",
          aliases: ["stage"],
          enabled: true,
        },
      ],
      groups: [],
    },
  ],
};

// ---------------------------------------------------------------------------
// serializeHostsFile
// ---------------------------------------------------------------------------

describe("serializeHostsFile", () => {
  test("emits YAML containing version and groups", () => {
    const yaml = serializeHostsFile(simpleHostsFile);

    expect(yaml).toContain("version: 1");
    expect(yaml).toContain("groups:");
    expect(yaml).toContain("name: dev");
    expect(yaml).toContain("hostname: localhost.dev");
  });

  test("uses 2-space indentation", () => {
    const yaml = serializeHostsFile(simpleHostsFile);

    // Top-level group list item must appear with two-space indent.
    expect(yaml).toMatch(/\n {2}- name: dev/);
    // No tab characters.
    expect(yaml).not.toMatch(/\t/);
  });

  test("serializes nested groups", () => {
    const yaml = serializeHostsFile(nestedHostsFile);

    expect(yaml).toContain("name: work");
    expect(yaml).toContain("name: prod");
    expect(yaml).toContain("hostname: api.prod");
  });

  test("serializes entries with optional fields (comment, disabled)", () => {
    const yaml = serializeHostsFile(complexHostsFile);

    expect(yaml).toContain("enabled: false");
    expect(yaml).toContain("comment: Development host");
    expect(yaml).toContain("aliases:");
  });

  test("returns a non-empty string", () => {
    const yaml = serializeHostsFile(simpleHostsFile);
    expect(typeof yaml).toBe("string");
    expect(yaml.length).toBeGreaterThan(0);
  });
});

// ---------------------------------------------------------------------------
// deserializeHostsFile
// ---------------------------------------------------------------------------

describe("deserializeHostsFile", () => {
  test("parses a minimal valid document", () => {
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
    expect(result.groups[0].entries[0].enabled).toBe(true);
  });

  test("parses nested groups recursively", () => {
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
    expect(result.groups[0].groups[0].entries[0].aliases).toEqual(["api"]);
  });

  test("parses optional comment field on entries", () => {
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

  test("omits comment when not present in YAML", () => {
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
    expect(result.groups[0].entries[0].comment).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// Schema / version validation
// ---------------------------------------------------------------------------

describe("schema version validation", () => {
  test("rejects missing version field", () => {
    const yaml = `groups:
  - name: test
    entries: []
    groups: []
`;
    expect(() => deserializeHostsFile(yaml)).toThrow(
      "Invalid ~/.hosts: missing version field"
    );
  });

  test("rejects unsupported version (v2)", () => {
    const yaml = `version: 2
groups: []
`;
    expect(() => deserializeHostsFile(yaml)).toThrow(
      "Unsupported ~/.hosts version 2. This tool supports version 1. Please upgrade hostie."
    );
  });

  test("rejects version 0 / less than 1", () => {
    const yaml = `version: 0
groups: []
`;
    expect(() => deserializeHostsFile(yaml)).toThrow(
      "Invalid ~/.hosts: missing version field"
    );
  });

  test("rejects non-numeric (string) version", () => {
    const yaml = `version: "1"
groups: []
`;
    expect(() => deserializeHostsFile(yaml)).toThrow(
      "Invalid ~/.hosts: missing version field"
    );
  });

  test("accepts version 1 (the only supported version)", () => {
    const yaml = `version: 1
groups: []
`;
    const result = deserializeHostsFile(yaml);
    expect(result.version).toBe(1);
    expect(result.groups).toEqual([]);
  });
});

// ---------------------------------------------------------------------------
// Invalid YAML / invalid schema errors
// ---------------------------------------------------------------------------

describe("error handling", () => {
  test("throws on syntactically invalid YAML", () => {
    const invalidYaml = `version: 1
groups: [
  - name: broken
`;
    expect(() => deserializeHostsFile(invalidYaml)).toThrow();
  });

  test("throws when root is not an object (scalar)", () => {
    expect(() => deserializeHostsFile("just a string\n")).toThrow(
      /expected an object at root/
    );
  });

  test("throws when groups is not an array", () => {
    const yaml = `version: 1
groups: "not an array"
`;
    expect(() => deserializeHostsFile(yaml)).toThrow(
      /'groups' must be an array/
    );
  });

  test("throws when groups field is missing", () => {
    const yaml = `version: 1
`;
    expect(() => deserializeHostsFile(yaml)).toThrow(
      /missing required 'groups' field/
    );
  });

  test("throws when a group is missing 'name'", () => {
    const yaml = `version: 1
groups:
  - entries: []
    groups: []
`;
    expect(() => deserializeHostsFile(yaml)).toThrow(
      /missing or invalid 'name' field/
    );
  });

  test("throws when an entry is missing a required field", () => {
    const yaml = `version: 1
groups:
  - name: dev
    entries:
      - id: 01HZXYZ123
        ip: 127.0.0.1
        hostname: localhost.dev
        aliases: []
    groups: []
`;
    expect(() => deserializeHostsFile(yaml)).toThrow(
      /missing required 'enabled' field/
    );
  });

  test("throws when entry.enabled is not a boolean", () => {
    const yaml = `version: 1
groups:
  - name: dev
    entries:
      - id: 01HZXYZ123
        ip: 127.0.0.1
        hostname: localhost.dev
        aliases: []
        enabled: "yes"
    groups: []
`;
    expect(() => deserializeHostsFile(yaml)).toThrow(
      /'enabled' must be a boolean/
    );
  });

  test("throws when entry.aliases contains a non-string", () => {
    const yaml = `version: 1
groups:
  - name: dev
    entries:
      - id: 01HZXYZ123
        ip: 127.0.0.1
        hostname: localhost.dev
        aliases: [1, 2]
        enabled: true
    groups: []
`;
    expect(() => deserializeHostsFile(yaml)).toThrow(
      /all 'aliases' must be strings/
    );
  });
});

// ---------------------------------------------------------------------------
// Round-trip
// ---------------------------------------------------------------------------

describe("round-trip serialize ⇄ deserialize", () => {
  test("simple file: data preserved across one round-trip", () => {
    const yaml = serializeHostsFile(simpleHostsFile);
    const parsed = deserializeHostsFile(yaml);
    expect(parsed).toEqual(simpleHostsFile);
  });

  test("nested file: data preserved across one round-trip", () => {
    const yaml = serializeHostsFile(nestedHostsFile);
    const parsed = deserializeHostsFile(yaml);
    expect(parsed).toEqual(nestedHostsFile);
  });

  test("complex file with comments + disabled entries: data preserved", () => {
    const yaml = serializeHostsFile(complexHostsFile);
    const parsed = deserializeHostsFile(yaml);
    expect(parsed).toEqual(complexHostsFile);
  });

  test("serialize → deserialize → serialize produces identical YAML", () => {
    const yaml1 = serializeHostsFile(complexHostsFile);
    const parsed = deserializeHostsFile(yaml1);
    const yaml2 = serializeHostsFile(parsed);
    expect(yaml2).toBe(yaml1);
  });

  test("round-trip is stable across multiple iterations", () => {
    let current: HostsFile = complexHostsFile;
    let previousYaml = serializeHostsFile(current);
    for (let i = 0; i < 3; i++) {
      current = deserializeHostsFile(previousYaml);
      const nextYaml = serializeHostsFile(current);
      expect(nextYaml).toBe(previousYaml);
      previousYaml = nextYaml;
    }
    expect(current).toEqual(complexHostsFile);
  });
});
