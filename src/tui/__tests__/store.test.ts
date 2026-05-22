/**
 * Tests for TUI Zustand store
 */
import { describe, test, expect, beforeEach } from "bun:test";
import { useAppStore } from "../store";
import type { HostsFile, Entry, Group } from "../../domain/types";

describe("AppStore", () => {
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

  test("initializes with default state", () => {
    const state = useAppStore.getState();
    expect(state.hostsFile).toEqual({ version: 1, groups: [] });
    expect(state.selectedEntryId).toBeNull();
    expect(state.selectedGroupPath).toEqual([]);
    expect(state.searchQuery).toEqual("");
    expect(state.mode).toEqual("normal");
    expect(state.dirty).toBe(false);
  });

  test("loadHostsFile updates hostsFile state", () => {
    const mockFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "work",
          entries: [],
          groups: [],
        },
      ],
    };

    useAppStore.getState().loadHostsFile(mockFile);
    const state = useAppStore.getState();
    
    expect(state.hostsFile).toEqual(mockFile);
    expect(state.dirty).toBe(false);
  });

  test("selectEntry updates selectedEntryId", () => {
    useAppStore.getState().selectEntry("test-id-123");
    expect(useAppStore.getState().selectedEntryId).toBe("test-id-123");
  });

  test("selectEntry with null clears selection", () => {
    useAppStore.getState().selectEntry("test-id-123");
    useAppStore.getState().selectEntry(null);
    expect(useAppStore.getState().selectedEntryId).toBeNull();
  });

  test("setSearchQuery updates searchQuery", () => {
    useAppStore.getState().setSearchQuery("localhost");
    expect(useAppStore.getState().searchQuery).toBe("localhost");
  });

  test("setMode updates mode", () => {
    useAppStore.getState().setMode("search");
    expect(useAppStore.getState().mode).toBe("search");
  });

  test("selectGroup updates selectedGroupPath", () => {
    useAppStore.getState().selectGroup(["work", "prod"]);
    expect(useAppStore.getState().selectedGroupPath).toEqual(["work", "prod"]);
  });

  test("markDirty sets dirty flag", () => {
    useAppStore.getState().markDirty();
    expect(useAppStore.getState().dirty).toBe(true);
  });

  test("clearDirty clears dirty flag", () => {
    useAppStore.getState().markDirty();
    useAppStore.getState().clearDirty();
    expect(useAppStore.getState().dirty).toBe(false);
  });

  test("addEntry adds entry to selected group and marks dirty", () => {
    const mockFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "work",
          entries: [],
          groups: [],
        },
      ],
    };

    useAppStore.getState().loadHostsFile(mockFile);
    useAppStore.getState().selectGroup(["work"]);

    const newEntry: Entry = {
      id: "test-entry-1",
      ip: "127.0.0.1",
      hostname: "test.local",
      aliases: [],
      enabled: true,
    };

    useAppStore.getState().addEntry(newEntry);
    
    const state = useAppStore.getState();
    expect(state.hostsFile.groups[0].entries).toHaveLength(1);
    expect(state.hostsFile.groups[0].entries[0]).toEqual(newEntry);
    expect(state.dirty).toBe(true);
  });

  test("updateEntry updates existing entry and marks dirty", () => {
    const mockFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "work",
          entries: [
            {
              id: "entry-1",
              ip: "127.0.0.1",
              hostname: "old.local",
              aliases: [],
              enabled: true,
            },
          ],
          groups: [],
        },
      ],
    };

    useAppStore.getState().loadHostsFile(mockFile);

    const updatedEntry: Entry = {
      id: "entry-1",
      ip: "192.168.1.1",
      hostname: "new.local",
      aliases: ["alias1"],
      enabled: false,
      comment: "Updated",
    };

    useAppStore.getState().updateEntry("entry-1", updatedEntry);
    
    const state = useAppStore.getState();
    expect(state.hostsFile.groups[0].entries[0]).toEqual(updatedEntry);
    expect(state.dirty).toBe(true);
  });

  test("deleteEntry removes entry and marks dirty", () => {
    const mockFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "work",
          entries: [
            {
              id: "entry-1",
              ip: "127.0.0.1",
              hostname: "test.local",
              aliases: [],
              enabled: true,
            },
          ],
          groups: [],
        },
      ],
    };

    useAppStore.getState().loadHostsFile(mockFile);
    useAppStore.getState().deleteEntry("entry-1");
    
    const state = useAppStore.getState();
    expect(state.hostsFile.groups[0].entries).toHaveLength(0);
    expect(state.dirty).toBe(true);
  });

  test("toggleEntry toggles enabled state and marks dirty", () => {
    const mockFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "work",
          entries: [
            {
              id: "entry-1",
              ip: "127.0.0.1",
              hostname: "test.local",
              aliases: [],
              enabled: true,
            },
          ],
          groups: [],
        },
      ],
    };

    useAppStore.getState().loadHostsFile(mockFile);
    useAppStore.getState().toggleEntry("entry-1");
    
    let state = useAppStore.getState();
    expect(state.hostsFile.groups[0].entries[0].enabled).toBe(false);
    expect(state.dirty).toBe(true);

    // Toggle back
    useAppStore.getState().toggleEntry("entry-1");
    state = useAppStore.getState();
    expect(state.hostsFile.groups[0].entries[0].enabled).toBe(true);
  });
});
