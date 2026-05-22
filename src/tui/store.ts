/**
 * Zustand store for TUI state management
 * 
 * Manages the application state including hosts file data, selections, search, and dirty flag.
 */
import { create } from "zustand";
import type { HostsFile, Entry, Group } from "../domain/types";
import { readHostsFile } from "../core/file-io";

/**
 * TUI mode states
 */
export type Mode = "normal" | "search" | "edit" | "modal";

/**
 * Application state
 */
export interface AppState {
  /** Current hosts file data */
  hostsFile: HostsFile;
  /** Currently selected entry ID (null if none selected) */
  selectedEntryId: string | null;
  /** Path to currently selected group (e.g., ["work", "prod"]) */
  selectedGroupPath: string[];
  /** Current search query */
  searchQuery: string;
  /** Current TUI mode */
  mode: Mode;
  /** Whether there are unsaved changes */
  dirty: boolean;

  // Actions
  /** Load hosts file data into state */
  loadHostsFile: (file: HostsFile) => void;
  /** Select an entry by ID */
  selectEntry: (id: string | null) => void;
  /** Select a group by path */
  selectGroup: (path: string[]) => void;
  /** Update search query */
  setSearchQuery: (query: string) => void;
  /** Change TUI mode */
  setMode: (mode: Mode) => void;
  /** Mark state as dirty (unsaved changes) */
  markDirty: () => void;
  /** Clear dirty flag */
  clearDirty: () => void;
  /** Add a new entry to the selected group */
  addEntry: (entry: Entry) => void;
  /** Update an existing entry */
  updateEntry: (id: string, entry: Entry) => void;
  /** Delete an entry by ID */
  deleteEntry: (id: string) => void;
  /** Toggle entry enabled state */
  toggleEntry: (id: string) => void;
}

/**
 * Helper: Find and update an entry in the hosts file structure
 */
function findAndUpdateEntry(
  groups: Group[],
  id: string,
  updater: (entry: Entry) => Entry | null
): Group[] {
  return groups.map((group) => ({
    ...group,
    entries: group.entries
      .map((entry) => (entry.id === id ? updater(entry) : entry))
      .filter((entry): entry is Entry => entry !== null),
    groups: findAndUpdateEntry(group.groups, id, updater),
  }));
}

/**
 * Helper: Find a group by path
 */
function findGroupByPath(groups: Group[], path: string[]): Group | null {
  if (path.length === 0) return null;

  const [first, ...rest] = path;
  const group = groups.find((g) => g.name === first);

  if (!group) return null;
  if (rest.length === 0) return group;

  return findGroupByPath(group.groups, rest);
}

/**
 * Helper: Add entry to a specific group path
 */
function addEntryToGroup(groups: Group[], path: string[], entry: Entry): Group[] {
  if (path.length === 0) {
    // Add to root level - not supported in current design
    return groups;
  }

  const [first, ...rest] = path;

  return groups.map((group) => {
    if (group.name !== first) return group;

    if (rest.length === 0) {
      // Add to this group
      return {
        ...group,
        entries: [...group.entries, entry],
      };
    }

    // Recurse into nested groups
    return {
      ...group,
      groups: addEntryToGroup(group.groups, rest, entry),
    };
  });
}

/**
 * Create the Zustand store with initial state loaded from ~/.hosts
 */
export const useAppStore = create<AppState>((set) => ({
  // Initial state
  hostsFile: { version: 1, groups: [] },
  selectedEntryId: null,
  selectedGroupPath: [],
  searchQuery: "",
  mode: "normal",
  dirty: false,

  // Actions
  loadHostsFile: (file) =>
    set({
      hostsFile: file,
      dirty: false,
    }),

  selectEntry: (id) =>
    set({
      selectedEntryId: id,
    }),

  selectGroup: (path) =>
    set({
      selectedGroupPath: path,
    }),

  setSearchQuery: (query) =>
    set({
      searchQuery: query,
    }),

  setMode: (mode) =>
    set({
      mode,
    }),

  markDirty: () =>
    set({
      dirty: true,
    }),

  clearDirty: () =>
    set({
      dirty: false,
    }),

  addEntry: (entry) =>
    set((state) => ({
      hostsFile: {
        ...state.hostsFile,
        groups: addEntryToGroup(state.hostsFile.groups, state.selectedGroupPath, entry),
      },
      dirty: true,
    })),

  updateEntry: (id, entry) =>
    set((state) => ({
      hostsFile: {
        ...state.hostsFile,
        groups: findAndUpdateEntry(state.hostsFile.groups, id, () => entry),
      },
      dirty: true,
    })),

  deleteEntry: (id) =>
    set((state) => ({
      hostsFile: {
        ...state.hostsFile,
        groups: findAndUpdateEntry(state.hostsFile.groups, id, () => null),
      },
      dirty: true,
      selectedEntryId: state.selectedEntryId === id ? null : state.selectedEntryId,
    })),

  toggleEntry: (id) =>
    set((state) => ({
      hostsFile: {
        ...state.hostsFile,
        groups: findAndUpdateEntry(state.hostsFile.groups, id, (entry) => ({
          ...entry,
          enabled: !entry.enabled,
        })),
      },
      dirty: true,
    })),
}));

/**
 * Initialize store by loading ~/.hosts on first mount
 * Call this once at app startup
 */
export async function initializeStore(): Promise<void> {
  const hostsFile = await readHostsFile();
  useAppStore.getState().loadHostsFile(hostsFile);
}
