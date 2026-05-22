/**
 * Keyboard navigation hook for TUI
 * 
 * Handles j/k navigation, Tab focus switching, and updates store state.
 */
import { useInput } from "ink";
import { useAppStore } from "../store";
import type { Entry, Group } from "../../domain/types";

/**
 * Focus area in the TUI
 */
type FocusArea = "sidebar" | "main";

/**
 * Flattens all entries from groups recursively for navigation
 */
function flattenEntries(groups: Group[]): Entry[] {
  const entries: Entry[] = [];
  
  function traverse(group: Group) {
    entries.push(...group.entries);
    group.groups.forEach(traverse);
  }
  
  groups.forEach(traverse);
  return entries;
}

/**
 * Flattens all group paths for sidebar navigation
 */
function flattenGroupPaths(groups: Group[], parentPath: string[] = []): string[][] {
  const paths: string[][] = [];
  
  for (const group of groups) {
    const currentPath = [...parentPath, group.name];
    paths.push(currentPath);
    paths.push(...flattenGroupPaths(group.groups, currentPath));
  }
  
  return paths;
}

/**
 * Custom hook for keyboard navigation
 * 
 * Handles:
 * - j/k for up/down navigation
 * - Tab for focus switching between sidebar and main
 * - Updates store with selected entry/group
 */
export function useKeyboard() {
  const {
    hostsFile,
    selectedEntryId,
    selectedGroupPath,
    mode,
    selectEntry,
    selectGroup,
  } = useAppStore();

  // Track current focus area (sidebar or main)
  // In a real implementation, this would be in the store
  // For now, we'll determine it based on whether we have a selected group
  let currentFocus: FocusArea = selectedGroupPath.length > 0 ? "sidebar" : "main";

  useInput((input, key) => {
    // Only handle navigation in normal mode
    if (mode !== "normal") {
      return;
    }

    // Handle Tab for focus switching
    if (key.tab) {
      if (currentFocus === "sidebar") {
        // Switch to main - select first entry if none selected
        const entries = flattenEntries(hostsFile.groups);
        if (entries.length > 0 && !selectedEntryId) {
          selectEntry(entries[0].id);
        }
      } else {
        // Switch to sidebar - select first group if none selected
        const groupPaths = flattenGroupPaths(hostsFile.groups);
        if (groupPaths.length > 0 && selectedGroupPath.length === 0) {
          selectGroup(groupPaths[0]);
        }
      }
      return;
    }

    // Handle j (down) navigation
    if (input === "j") {
      if (currentFocus === "sidebar") {
        // Navigate down in sidebar (groups)
        const groupPaths = flattenGroupPaths(hostsFile.groups);
        const currentIndex = groupPaths.findIndex(
          (path) => JSON.stringify(path) === JSON.stringify(selectedGroupPath)
        );
        
        if (currentIndex < groupPaths.length - 1) {
          selectGroup(groupPaths[currentIndex + 1]);
        } else if (groupPaths.length > 0) {
          // Wrap to first group
          selectGroup(groupPaths[0]);
        }
      } else {
        // Navigate down in main (entries)
        const entries = flattenEntries(hostsFile.groups);
        const currentIndex = entries.findIndex((e) => e.id === selectedEntryId);
        
        if (currentIndex < entries.length - 1) {
          selectEntry(entries[currentIndex + 1].id);
        } else if (entries.length > 0) {
          // Wrap to first entry
          selectEntry(entries[0].id);
        }
      }
      return;
    }

    // Handle k (up) navigation
    if (input === "k") {
      if (currentFocus === "sidebar") {
        // Navigate up in sidebar (groups)
        const groupPaths = flattenGroupPaths(hostsFile.groups);
        const currentIndex = groupPaths.findIndex(
          (path) => JSON.stringify(path) === JSON.stringify(selectedGroupPath)
        );
        
        if (currentIndex > 0) {
          selectGroup(groupPaths[currentIndex - 1]);
        } else if (groupPaths.length > 0) {
          // Wrap to last group
          selectGroup(groupPaths[groupPaths.length - 1]);
        }
      } else {
        // Navigate up in main (entries)
        const entries = flattenEntries(hostsFile.groups);
        const currentIndex = entries.findIndex((e) => e.id === selectedEntryId);
        
        if (currentIndex > 0) {
          selectEntry(entries[currentIndex - 1].id);
        } else if (entries.length > 0) {
          // Wrap to last entry
          selectEntry(entries[entries.length - 1].id);
        }
      }
      return;
    }
  });
}
