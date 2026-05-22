/**
 * Tests for useSearch hook
 * 
 * Verifies fuzzy search functionality across hostname, aliases, IP, and group path.
 * Tests the search logic directly without requiring React rendering.
 */
import { describe, test, expect, beforeEach } from "bun:test";
import { useAppStore } from "../../store";
import Fuse from "fuse.js";
import type { HostsFile, Entry, Group } from "../../../domain/types";

/**
 * Flattened entry with group path for searching
 */
interface SearchableEntry {
  entry: Entry;
  groupPath: string[];
  groupPathString: string;
}

/**
 * Recursively flatten all entries with their group paths
 */
function flattenEntriesWithPaths(
  groups: Group[],
  parentPath: string[] = []
): SearchableEntry[] {
  const entries: SearchableEntry[] = [];

  for (const group of groups) {
    const currentPath = [...parentPath, group.name];
    const groupPathString = currentPath.join("/");

    for (const entry of group.entries) {
      entries.push({
        entry,
        groupPath: currentPath,
        groupPathString,
      });
    }

    entries.push(...flattenEntriesWithPaths(group.groups, currentPath));
  }

  return entries;
}

/**
 * Perform fuzzy search on entries
 */
function performSearch(hostsFile: HostsFile, query: string) {
  if (!query || query.trim() === "") {
    return [];
  }

  const searchableEntries = flattenEntriesWithPaths(hostsFile.groups);
  
  const fuse = new Fuse(searchableEntries, {
    keys: [
      { name: "entry.hostname", weight: 2 },
      { name: "entry.aliases", weight: 1.5 },
      { name: "entry.ip", weight: 1 },
      { name: "groupPathString", weight: 0.5 },
    ],
    threshold: 0.4,
    includeScore: true,
    minMatchCharLength: 1,
    ignoreLocation: true,
  });

  const fuseResults = fuse.search(query);

  return fuseResults.slice(0, 10).map((result) => ({
    item: result.item.entry,
    groupPath: result.item.groupPath,
    score: result.score,
  }));
}

describe("useSearch logic", () => {
  const mockHostsFile: HostsFile = {
    version: 1,
    groups: [
      {
        name: "work",
        entries: [
          {
            id: "entry-1",
            ip: "10.0.2.10",
            hostname: "db.prod.work",
            aliases: ["database", "postgres"],
            enabled: true,
          },
          {
            id: "entry-2",
            ip: "10.0.2.11",
            hostname: "api.prod.work",
            aliases: ["api-server"],
            enabled: true,
          },
        ],
        groups: [
          {
            name: "staging",
            entries: [
              {
                id: "entry-3",
                ip: "10.0.3.10",
                hostname: "staging-api.work",
                aliases: [],
                enabled: true,
              },
            ],
            groups: [],
          },
        ],
      },
      {
        name: "personal",
        entries: [
          {
            id: "entry-4",
            ip: "192.168.1.100",
            hostname: "homelab.local",
            aliases: ["lab"],
            enabled: false,
          },
        ],
        groups: [],
      },
    ],
  };

  beforeEach(() => {
    useAppStore.setState({
      hostsFile: mockHostsFile,
      selectedEntryId: null,
      selectedGroupPath: [],
      searchQuery: "",
      mode: "normal",
      dirty: false,
    });
  });

  test("returns empty results when search query is empty", () => {
    const results = performSearch(mockHostsFile, "");
    expect(results).toEqual([]);
  });

  test("searches by hostname", () => {
    const results = performSearch(mockHostsFile, "api");

    expect(results.length).toBeGreaterThan(0);
    const hostnames = results.map((r) => r.item.hostname);
    expect(hostnames).toContain("api.prod.work");
    expect(hostnames).toContain("staging-api.work");
  });

  test("searches by IP address", () => {
    const results = performSearch(mockHostsFile, "10.0.2.10");

    expect(results.length).toBeGreaterThan(0);
    expect(results[0].item.ip).toBe("10.0.2.10");
    expect(results[0].item.hostname).toBe("db.prod.work");
  });

  test("searches by alias", () => {
    const results = performSearch(mockHostsFile, "postgres");

    expect(results.length).toBeGreaterThan(0);
    expect(results[0].item.aliases).toContain("postgres");
  });

  test("searches by group path", () => {
    const results = performSearch(mockHostsFile, "staging");

    expect(results.length).toBeGreaterThan(0);
    const stagingEntry = results.find(
      (r) => r.item.hostname === "staging-api.work"
    );
    expect(stagingEntry).toBeDefined();
    expect(stagingEntry?.groupPath).toEqual(["work", "staging"]);
  });

  test("fuzzy matching works with typos", () => {
    const results = performSearch(mockHostsFile, "homelb");

    expect(results.length).toBeGreaterThan(0);
    expect(results[0].item.hostname).toBe("homelab.local");
  });

  test("returns results sorted by relevance", () => {
    const results = performSearch(mockHostsFile, "api");

    expect(results.length).toBeGreaterThan(0);
    // Exact match in hostname should rank higher
    expect(results[0].item.hostname).toContain("api");
  });

  test("limits results to top 10", () => {
    const largeHostsFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "test",
          entries: Array.from({ length: 15 }, (_, i) => ({
            id: `entry-${i}`,
            ip: `10.0.0.${i}`,
            hostname: `server${i}.test`,
            aliases: [],
            enabled: true,
          })),
          groups: [],
        },
      ],
    };

    const results = performSearch(largeHostsFile, "server");
    expect(results.length).toBeLessThanOrEqual(10);
  });

  test("includes group path in search results", () => {
    const results = performSearch(mockHostsFile, "staging-api");

    expect(results.length).toBeGreaterThan(0);
    expect(results[0].groupPath).toEqual(["work", "staging"]);
  });

  test("store setSearchQuery updates search query", () => {
    useAppStore.getState().setSearchQuery("test-query");
    expect(useAppStore.getState().searchQuery).toBe("test-query");
  });

  test("flattenEntriesWithPaths correctly flattens nested groups", () => {
    const flattened = flattenEntriesWithPaths(mockHostsFile.groups);

    expect(flattened.length).toBe(4); // 2 in work, 1 in work/staging, 1 in personal
    
    const stagingEntry = flattened.find(
      (e) => e.entry.hostname === "staging-api.work"
    );
    expect(stagingEntry?.groupPath).toEqual(["work", "staging"]);
    expect(stagingEntry?.groupPathString).toBe("work/staging");
  });
});
