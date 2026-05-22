/**
 * Test React component subscription to store
 */
import { describe, test, expect } from "bun:test";
import { useAppStore } from "../store";
import type { Entry } from "../../domain/types";

describe("Store React integration", () => {
  test("React components can subscribe to store state via getState", () => {
    // Reset state
    useAppStore.setState({
      hostsFile: { version: 1, groups: [] },
      dirty: false,
    });

    // Initial state
    expect(useAppStore.getState().hostsFile).toEqual({ version: 1, groups: [] });
    expect(useAppStore.getState().dirty).toBe(false);

    // Update state
    useAppStore.getState().markDirty();

    // State updated
    expect(useAppStore.getState().dirty).toBe(true);
  });

  test("Store supports subscription callbacks", () => {
    let callCount = 0;
    let lastSearchQuery = "";

    // Subscribe to state changes
    const unsubscribe = useAppStore.subscribe((state) => {
      callCount++;
      lastSearchQuery = state.searchQuery;
    });

    // Initial value
    expect(useAppStore.getState().searchQuery).toBe("");

    // Update triggers subscription
    useAppStore.getState().setSearchQuery("test");

    // Subscription was called
    expect(callCount).toBeGreaterThan(0);
    expect(lastSearchQuery).toBe("test");

    unsubscribe();
  });

  test("Store updates propagate to subscribers when entries are modified", () => {
    let updateCount = 0;

    // Subscribe to changes
    const unsubscribe = useAppStore.subscribe(() => {
      updateCount++;
    });

    // Set up initial state with a group
    useAppStore.getState().loadHostsFile({
      version: 1,
      groups: [
        {
          name: "work",
          entries: [],
          groups: [],
        },
      ],
    });
    useAppStore.getState().selectGroup(["work"]);

    const initialUpdateCount = updateCount;
    expect(useAppStore.getState().hostsFile.groups[0].entries).toHaveLength(0);

    // Add entry
    const newEntry: Entry = {
      id: "test-1",
      ip: "127.0.0.1",
      hostname: "test.local",
      aliases: [],
      enabled: true,
    };

    useAppStore.getState().addEntry(newEntry);

    // Subscriber was notified
    expect(updateCount).toBeGreaterThan(initialUpdateCount);
    expect(useAppStore.getState().hostsFile.groups[0].entries).toHaveLength(1);
    expect(useAppStore.getState().hostsFile.groups[0].entries[0].hostname).toBe("test.local");

    unsubscribe();
  });
});
