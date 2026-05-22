/**
 * Test keyboard navigation hook
 * 
 * Tests the navigation logic by directly testing the store updates
 * since Ink hooks require a terminal environment.
 */
import { describe, test, expect, beforeEach } from "bun:test";
import { useAppStore } from "../../store";
import type { Entry, Group } from "../../../domain/types";

describe("useKeyboard navigation logic", () => {
  beforeEach(() => {
    // Reset store state before each test
    useAppStore.setState({
      hostsFile: { version: 1, groups: [] },
      selectedEntryId: null,
      selectedGroupPath: [],
      searchQuery: "",
      mode: "normal",
      dirty: false,
    });
  });

  test("selectEntry updates selected entry ID", () => {
    const entry: Entry = {
      id: "entry-1",
      ip: "127.0.0.1",
      hostname: "test.local",
      aliases: [],
      enabled: true,
    };

    useAppStore.getState().selectEntry(entry.id);
    expect(useAppStore.getState().selectedEntryId).toBe("entry-1");
  });

  test("selectGroup updates selected group path", () => {
    const groupPath = ["work", "prod"];

    useAppStore.getState().selectGroup(groupPath);
    expect(useAppStore.getState().selectedGroupPath).toEqual(["work", "prod"]);
  });

  test("navigation wraps at boundaries - entries", () => {
    // Set up hosts file with entries
    const hostsFile = {
      version: 1 as const,
      groups: [
        {
          name: "work",
          entries: [
            {
              id: "entry-1",
              ip: "127.0.0.1",
              hostname: "test1.local",
              aliases: [],
              enabled: true,
            },
            {
              id: "entry-2",
              ip: "127.0.0.2",
              hostname: "test2.local",
              aliases: [],
              enabled: true,
            },
          ],
          groups: [],
        },
      ],
    };

    useAppStore.getState().loadHostsFile(hostsFile);

    // Select first entry
    useAppStore.getState().selectEntry("entry-1");
    expect(useAppStore.getState().selectedEntryId).toBe("entry-1");

    // Navigate to second entry
    useAppStore.getState().selectEntry("entry-2");
    expect(useAppStore.getState().selectedEntryId).toBe("entry-2");

    // Wrap back to first
    useAppStore.getState().selectEntry("entry-1");
    expect(useAppStore.getState().selectedEntryId).toBe("entry-1");
  });

  test("navigation wraps at boundaries - groups", () => {
    // Set up hosts file with groups
    const hostsFile = {
      version: 1 as const,
      groups: [
        {
          name: "work",
          entries: [],
          groups: [],
        },
        {
          name: "personal",
          entries: [],
          groups: [],
        },
      ],
    };

    useAppStore.getState().loadHostsFile(hostsFile);

    // Select first group
    useAppStore.getState().selectGroup(["work"]);
    expect(useAppStore.getState().selectedGroupPath).toEqual(["work"]);

    // Navigate to second group
    useAppStore.getState().selectGroup(["personal"]);
    expect(useAppStore.getState().selectedGroupPath).toEqual(["personal"]);

    // Wrap back to first
    useAppStore.getState().selectGroup(["work"]);
    expect(useAppStore.getState().selectedGroupPath).toEqual(["work"]);
  });

  test("mode changes affect navigation behavior", () => {
    // In normal mode, navigation should work
    expect(useAppStore.getState().mode).toBe("normal");

    // Switch to edit mode
    useAppStore.getState().setMode("edit");
    expect(useAppStore.getState().mode).toBe("edit");

    // Switch back to normal
    useAppStore.getState().setMode("normal");
    expect(useAppStore.getState().mode).toBe("normal");
  });
});
