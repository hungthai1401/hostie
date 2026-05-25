/**
 * Search hook for TUI
 * 
 * Provides fuzzy search across entries using fuse.js.
 * Searches hostname, aliases, IP, and group path.
 */
import { useMemo } from "react";
import Fuse from "fuse.js";
import { useAppStore } from "../store";
import type { Entry, Group } from "../../domain/types";

/**
 * Search result with entry and its group path
 */
export interface SearchResult {
  item: Entry;
  groupPath: string[];
  score?: number;
}

/**
 * Flattened entry with group path for searching
 */
interface SearchableEntry {
  entry: Entry;
  groupPath: string[];
  groupPathString: string; // For searching
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

    // Add entries from this group
    for (const entry of group.entries) {
      entries.push({
        entry,
        groupPath: currentPath,
        groupPathString,
      });
    }

    // Recurse into nested groups
    entries.push(...flattenEntriesWithPaths(group.groups, currentPath));
  }

  return entries;
}

/**
 * Custom hook for fuzzy search
 * 
 * Searches across hostname, aliases, IP, and group path.
 * Returns top 10 results sorted by relevance.
 */
export function useSearch() {
  const { hostsFile, searchQuery } = useAppStore();

  // Flatten entries with their group paths
  const searchableEntries = useMemo(
    () => flattenEntriesWithPaths(hostsFile.groups),
    [hostsFile.groups]
  );

  // Configure Fuse.js for fuzzy search
  const fuse = useMemo(
    () =>
      new Fuse(searchableEntries, {
        keys: [
          { name: "entry.hostname", weight: 2 }, // Hostname is most important
          { name: "entry.aliases", weight: 1.5 }, // Aliases are also important
          { name: "entry.ip", weight: 1 },
          { name: "groupPathString", weight: 0.5 }, // Group path is least important
        ],
        threshold: 0.3, // Tighter fuzziness — avoids spurious single-char matches
        includeScore: true,
        minMatchCharLength: 2,
        ignoreLocation: true, // Don't care where in the string the match is
      }),
    [searchableEntries]
  );

  // Perform search
  const results = useMemo(() => {
    if (!searchQuery || searchQuery.trim() === "") {
      return [];
    }

    const fuseResults = fuse.search(searchQuery);

    // Map to SearchResult format and limit to top 10
    return fuseResults.slice(0, 10).map((result) => ({
      item: result.item.entry,
      groupPath: result.item.groupPath,
      score: result.score,
    }));
  }, [fuse, searchQuery]);

  return {
    results,
    isSearching: searchQuery.trim() !== "",
  };
}
