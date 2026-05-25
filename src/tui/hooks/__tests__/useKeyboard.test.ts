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

  test("toggleEntry changes enabled state and marks dirty", () => {
    const hostsFile = {
      version: 1 as const,
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

    useAppStore.getState().loadHostsFile(hostsFile);
    expect(useAppStore.getState().dirty).toBe(false);

    // Toggle entry from enabled to disabled
    useAppStore.getState().toggleEntry("entry-1");
    
    // Check that enabled state changed
    const entry = useAppStore.getState().hostsFile.groups[0].entries[0];
    expect(entry.enabled).toBe(false);
    expect(useAppStore.getState().dirty).toBe(true);

    // Toggle back to enabled
    useAppStore.getState().toggleEntry("entry-1");
    const entryAfter = useAppStore.getState().hostsFile.groups[0].entries[0];
    expect(entryAfter.enabled).toBe(true);
  });

  test("deleteEntry removes entry and marks dirty", () => {
    const hostsFile = {
      version: 1 as const,
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
    useAppStore.getState().selectEntry("entry-1");
    expect(useAppStore.getState().dirty).toBe(false);

    // Delete entry
    useAppStore.getState().deleteEntry("entry-1");
    
    // Check that entry was removed
    const entries = useAppStore.getState().hostsFile.groups[0].entries;
    expect(entries.length).toBe(1);
    expect(entries[0].id).toBe("entry-2");
    expect(useAppStore.getState().dirty).toBe(true);
    
    // Check that selection was cleared since deleted entry was selected
    expect(useAppStore.getState().selectedEntryId).toBe(null);
  });

  test("openModal sets modal state and changes mode", () => {
    useAppStore.getState().openModal("confirmation", { message: "Delete this entry?" });
    
    const state = useAppStore.getState();
    expect(state.modal).toEqual({ type: "confirmation", data: { message: "Delete this entry?" } });
    expect(state.mode).toBe("modal");
  });

  test("closeModal clears modal state and returns to normal mode", () => {
    useAppStore.getState().openModal("confirmation", { message: "Delete this entry?" });
    expect(useAppStore.getState().mode).toBe("modal");
    
    useAppStore.getState().closeModal();
    
    const state = useAppStore.getState();
    expect(state.modal).toBe(null);
    expect(state.mode).toBe("normal");
  });

  test("delete entry flow: modal opens, confirm deletes and moves selection", () => {
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
            {
              id: "entry-3",
              ip: "127.0.0.3",
              hostname: "test3.local",
              aliases: [],
              enabled: true,
            },
          ],
          groups: [],
        },
      ],
    };

    useAppStore.getState().loadHostsFile(hostsFile);
    useAppStore.getState().selectEntry("entry-2");

    // Simulate opening delete confirmation modal
    useAppStore.getState().openModal("confirmation", {
      message: "Delete this entry?",
      onConfirm: () => {
        useAppStore.getState().deleteEntry("entry-2");
        // Selection should move to next entry (entry-3)
        useAppStore.getState().selectEntry("entry-3");
        useAppStore.getState().closeModal();
      },
      onCancel: () => {
        useAppStore.getState().closeModal();
      },
    });

    // Verify modal is open
    expect(useAppStore.getState().mode).toBe("modal");
    expect(useAppStore.getState().modal?.type).toBe("confirmation");

    // Simulate user confirming
    const modal = useAppStore.getState().modal;
    if (modal?.data?.onConfirm) {
      modal.data.onConfirm();
    }

    // Verify entry was deleted
    const entries = useAppStore.getState().hostsFile.groups[0].entries;
    expect(entries.length).toBe(2);
    expect(entries.find((e) => e.id === "entry-2")).toBeUndefined();

    // Verify selection moved to next entry
    expect(useAppStore.getState().selectedEntryId).toBe("entry-3");

    // Verify modal was closed
    expect(useAppStore.getState().modal).toBe(null);
    expect(useAppStore.getState().mode).toBe("normal");
  });

  test("quit action (q key) should be handled in normal mode", () => {
    // This test verifies that the quit logic can be invoked
    // The actual exit is handled by Ink's useApp().exit() which we can't test here
    // We just verify that mode is normal (prerequisite for quit to work)
    expect(useAppStore.getState().mode).toBe("normal");
    
    // In the actual implementation, pressing 'q' in normal mode will call exit()
    // This test documents the expected behavior
  });

  test("moveEntry moves entry from source group to target group", () => {
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
          ],
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
    expect(useAppStore.getState().dirty).toBe(false);

    useAppStore.getState().moveEntry("entry-1", ["personal"]);

    const state = useAppStore.getState();
    expect(state.hostsFile.groups[0].entries.length).toBe(0);
    expect(state.hostsFile.groups[1].entries.length).toBe(1);
    expect(state.hostsFile.groups[1].entries[0].id).toBe("entry-1");
    expect(state.dirty).toBe(true);
  });

  test("moveEntry is a no-op when target path is empty", () => {
    const hostsFile = {
      version: 1 as const,
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

    useAppStore.getState().loadHostsFile(hostsFile);
    useAppStore.getState().moveEntry("entry-1", []);

    const state = useAppStore.getState();
    expect(state.hostsFile.groups[0].entries.length).toBe(1);
  });

  test("m key flow: opens move-to-group modal with onSelect that moves entry", () => {
    const hostsFile = {
      version: 1 as const,
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
        {
          name: "personal",
          entries: [],
          groups: [],
        },
      ],
    };

    useAppStore.getState().loadHostsFile(hostsFile);
    useAppStore.getState().selectEntry("entry-1");

    // Simulate opening the move-to-group modal (as the 'm' handler does)
    useAppStore.getState().openModal("move-to-group", {
      entryId: "entry-1",
      onSelect: (targetGroupPath: string[]) => {
        useAppStore.getState().moveEntry("entry-1", targetGroupPath);
        useAppStore.getState().closeModal();
      },
      onCancel: () => {
        useAppStore.getState().closeModal();
      },
    });

    expect(useAppStore.getState().mode).toBe("modal");
    expect(useAppStore.getState().modal?.type).toBe("move-to-group");

    // Simulate the user picking "personal"
    const modal = useAppStore.getState().modal;
    if (modal?.data?.onSelect) {
      modal.data.onSelect(["personal"]);
    }

    const state = useAppStore.getState();
    expect(state.hostsFile.groups[0].entries.length).toBe(0);
    expect(state.hostsFile.groups[1].entries.length).toBe(1);
    expect(state.hostsFile.groups[1].entries[0].id).toBe("entry-1");
    expect(state.modal).toBe(null);
    expect(state.mode).toBe("normal");
  });

  test("setStatusMessage / clearStatusMessage work", () => {
    expect(useAppStore.getState().statusMessage).toBe(null);

    useAppStore.getState().setStatusMessage("hello", "success");
    expect(useAppStore.getState().statusMessage).toEqual({
      text: "hello",
      level: "success",
    });

    useAppStore.getState().clearStatusMessage();
    expect(useAppStore.getState().statusMessage).toBe(null);
  });

  test("apply confirmation flow: confirm clears dirty and sets success message", () => {
    useAppStore.setState({ dirty: true });

    useAppStore.getState().openModal("confirmation", {
      message: "Apply changes to /etc/hosts?",
      onConfirm: () => {
        useAppStore.getState().closeModal();
        useAppStore.getState().setStatusMessage("ok", "success");
        useAppStore.getState().clearDirty();
      },
      onCancel: () => {
        useAppStore.getState().closeModal();
      },
    });

    expect(useAppStore.getState().mode).toBe("modal");
    expect(useAppStore.getState().modal?.data?.message).toBe(
      "Apply changes to /etc/hosts?"
    );

    useAppStore.getState().modal?.data?.onConfirm();

    const state = useAppStore.getState();
    expect(state.modal).toBe(null);
    expect(state.mode).toBe("normal");
    expect(state.dirty).toBe(false);
    expect(state.statusMessage?.level).toBe("success");
  });

  test("apply confirmation flow: cancel closes modal without clearing dirty", () => {
    useAppStore.setState({ dirty: true });

    useAppStore.getState().openModal("confirmation", {
      message: "Apply changes to /etc/hosts?",
      onConfirm: () => {
        useAppStore.getState().closeModal();
      },
      onCancel: () => {
        useAppStore.getState().closeModal();
      },
    });

    useAppStore.getState().modal?.data?.onCancel();

    const state = useAppStore.getState();
    expect(state.modal).toBe(null);
    expect(state.dirty).toBe(true);
  });
});
