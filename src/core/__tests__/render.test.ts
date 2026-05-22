import { describe, test, expect } from "bun:test";
import { renderEntry, wrapManagedBlock, renderHostsFile } from "../render";
import type { Entry } from "../../domain/types";

describe("renderEntry", () => {
  test("renders entry with no aliases or comment", () => {
    const entry: Entry = {
      id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
      ip: "192.168.1.1",
      hostname: "example.com",
      aliases: [],
      enabled: true,
    };
    expect(renderEntry(entry)).toBe("192.168.1.1 example.com");
  });

  test("renders entry with single alias", () => {
    const entry: Entry = {
      id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
      ip: "192.168.1.1",
      hostname: "example.com",
      aliases: ["ex"],
      enabled: true,
    };
    expect(renderEntry(entry)).toBe("192.168.1.1 example.com ex");
  });

  test("renders entry with multiple aliases", () => {
    const entry: Entry = {
      id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
      ip: "192.168.1.1",
      hostname: "example.com",
      aliases: ["ex", "ex.local"],
      enabled: true,
    };
    expect(renderEntry(entry)).toBe("192.168.1.1 example.com ex ex.local");
  });

  test("renders entry with comment but no aliases", () => {
    const entry: Entry = {
      id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
      ip: "192.168.1.1",
      hostname: "example.com",
      aliases: [],
      enabled: true,
      comment: "My server",
    };
    expect(renderEntry(entry)).toBe("192.168.1.1 example.com # My server");
  });

  test("renders entry with aliases and comment", () => {
    const entry: Entry = {
      id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
      ip: "192.168.1.100",
      hostname: "devserver.local",
      aliases: ["devserver"],
      enabled: true,
      comment: "Development server",
    };
    expect(renderEntry(entry)).toBe("192.168.1.100 devserver.local devserver # Development server");
  });

  test("renders IPv6 address correctly", () => {
    const entry: Entry = {
      id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
      ip: "::1",
      hostname: "localhost",
      aliases: [],
      enabled: true,
    };
    expect(renderEntry(entry)).toBe("::1 localhost");
  });

  test("renders IPv6 with aliases and comment", () => {
    const entry: Entry = {
      id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
      ip: "2001:db8::1",
      hostname: "ipv6.example.com",
      aliases: ["ipv6"],
      enabled: true,
      comment: "IPv6 test",
    };
    expect(renderEntry(entry)).toBe("2001:db8::1 ipv6.example.com ipv6 # IPv6 test");
  });

  test("uses single space between all fields", () => {
    const entry: Entry = {
      id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
      ip: "10.0.0.1",
      hostname: "test.local",
      aliases: ["t1", "t2", "t3"],
      enabled: true,
      comment: "Test",
    };
    const result = renderEntry(entry);
    expect(result).toBe("10.0.0.1 test.local t1 t2 t3 # Test");
    // Verify no double spaces
    expect(result).not.toContain("  ");
  });

  test("handles empty aliases array", () => {
    const entry: Entry = {
      id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
      ip: "127.0.0.1",
      hostname: "localhost",
      aliases: [],
      enabled: true,
    };
    expect(renderEntry(entry)).toBe("127.0.0.1 localhost");
  });

  test("handles comment with special characters", () => {
    const entry: Entry = {
      id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
      ip: "192.168.1.1",
      hostname: "example.com",
      aliases: [],
      enabled: true,
      comment: "Server (production) - do not modify!",
    };
    expect(renderEntry(entry)).toBe("192.168.1.1 example.com # Server (production) - do not modify!");
  });
});

describe("wrapManagedBlock", () => {
  test("wraps content with BEGIN/END markers", () => {
    const content = "192.168.1.1 example.com";
    const result = wrapManagedBlock(content);
    expect(result).toBe("# BEGIN HOSTIE\n\n192.168.1.1 example.com\n\n# END HOSTIE");
  });

  test("wraps multi-line content", () => {
    const content = "192.168.1.1 example.com\n192.168.1.2 test.com";
    const result = wrapManagedBlock(content);
    expect(result).toBe("# BEGIN HOSTIE\n\n192.168.1.1 example.com\n192.168.1.2 test.com\n\n# END HOSTIE");
  });

  test("wraps empty content", () => {
    const content = "";
    const result = wrapManagedBlock(content);
    expect(result).toBe("# BEGIN HOSTIE\n\n\n\n# END HOSTIE");
  });
});

describe("renderHostsFile", () => {
  test("renders empty hosts file", () => {
    const hostsFile = {
      version: 1 as const,
      groups: [],
    };
    const result = renderHostsFile(hostsFile);
    expect(result).toBe("# BEGIN HOSTIE\n\n\n\n# END HOSTIE");
  });

  test("renders single enabled entry", () => {
    const hostsFile = {
      version: 1 as const,
      groups: [
        {
          name: "test",
          entries: [
            {
              id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
              ip: "192.168.1.1",
              hostname: "example.com",
              aliases: [],
              enabled: true,
            },
          ],
          groups: [],
        },
      ],
    };
    const result = renderHostsFile(hostsFile);
    expect(result).toContain("192.168.1.1 example.com");
    expect(result).toContain("# BEGIN HOSTIE");
    expect(result).toContain("# END HOSTIE");
  });

  test("filters out disabled entries", () => {
    const hostsFile = {
      version: 1 as const,
      groups: [
        {
          name: "test",
          entries: [
            {
              id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
              ip: "192.168.1.1",
              hostname: "enabled.com",
              aliases: [],
              enabled: true,
            },
            {
              id: "01ARZ3NDEKTSV4RRFFQ69G5FAW",
              ip: "192.168.1.2",
              hostname: "disabled.com",
              aliases: [],
              enabled: false,
            },
          ],
          groups: [],
        },
      ],
    };
    const result = renderHostsFile(hostsFile);
    expect(result).toContain("192.168.1.1 enabled.com");
    expect(result).not.toContain("192.168.1.2 disabled.com");
  });

  test("flattens nested groups", () => {
    const hostsFile = {
      version: 1 as const,
      groups: [
        {
          name: "parent",
          entries: [
            {
              id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
              ip: "192.168.1.1",
              hostname: "parent.com",
              aliases: [],
              enabled: true,
            },
          ],
          groups: [
            {
              name: "child",
              entries: [
                {
                  id: "01ARZ3NDEKTSV4RRFFQ69G5FAW",
                  ip: "192.168.1.2",
                  hostname: "child.com",
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
    const result = renderHostsFile(hostsFile);
    expect(result).toContain("192.168.1.1 parent.com");
    expect(result).toContain("192.168.1.2 child.com");
    expect(result).toContain("# BEGIN HOSTIE");
    expect(result).toContain("# END HOSTIE");
  });

  test("renders multiple groups", () => {
    const hostsFile = {
      version: 1 as const,
      groups: [
        {
          name: "group1",
          entries: [
            {
              id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
              ip: "192.168.1.1",
              hostname: "one.com",
              aliases: [],
              enabled: true,
            },
          ],
          groups: [],
        },
        {
          name: "group2",
          entries: [
            {
              id: "01ARZ3NDEKTSV4RRFFQ69G5FAW",
              ip: "192.168.1.2",
              hostname: "two.com",
              aliases: [],
              enabled: true,
            },
          ],
          groups: [],
        },
      ],
    };
    const result = renderHostsFile(hostsFile);
    expect(result).toContain("192.168.1.1 one.com");
    expect(result).toContain("192.168.1.2 two.com");
  });
});
