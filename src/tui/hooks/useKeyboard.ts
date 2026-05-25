/**
 * Keyboard navigation hook for TUI
 * 
 * Handles j/k navigation, Tab focus switching, Space for toggling, and updates store state.
 */
import { useInput } from "ink";
import { useAppStore } from "../store";
import type { Entry, Group } from "../../domain/types";
import { writeHostsFile } from "../../core/file-io";
import { applyHostsFile } from "../../core/apply";

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
 * - Space for toggling entry enabled state
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
    toggleEntry,
    deleteEntry,
    moveEntry,
    openModal,
    closeModal,
    markDirty,
    setStatusMessage,
    clearDirty,
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

    // Handle Space for toggling enabled state
    if (input === " " && selectedEntryId) {
      // Toggle the selected entry
      toggleEntry(selectedEntryId);
      
      // Persist changes to ~/.hosts
      writeHostsFile("~/.hosts", hostsFile).catch((err) => {
        console.error("Failed to write hosts file:", err);
      });
      
      return;
    }

    // Handle 'd' for deleting entry
    if (input === "d" && selectedEntryId) {
      const entries = flattenEntries(hostsFile.groups);
      const currentIndex = entries.findIndex((e) => e.id === selectedEntryId);
      
      // Open confirmation modal with callbacks
      openModal("confirmation", {
        message: "Delete this entry?",
        onConfirm: () => {
          // Delete the entry
          deleteEntry(selectedEntryId);
          
          // Move selection to next entry (or previous if last)
          if (entries.length > 1) {
            if (currentIndex < entries.length - 1) {
              // Select next entry
              selectEntry(entries[currentIndex + 1].id);
            } else if (currentIndex > 0) {
              // Select previous entry (we're at the end)
              selectEntry(entries[currentIndex - 1].id);
            }
          }
          
          // Persist changes to ~/.hosts
          writeHostsFile("~/.hosts", hostsFile).catch((err) => {
            console.error("Failed to write hosts file:", err);
          });
          
          // Close modal
          closeModal();
        },
        onCancel: () => {
          // Just close the modal
          closeModal();
        },
      });
      return;
    }

    // Handle Enter or Ctrl+S for applying changes to /etc/hosts
    if (key.return || (key.ctrl && input === "s")) {
      openModal("confirmation", {
        message: "Apply changes to /etc/hosts?",
        onConfirm: () => {
          closeModal();
          applyHostsFile(hostsFile)
            .then((result) => {
              if (result.changed) {
                setStatusMessage(result.message, "success");
                clearDirty();
              } else {
                setStatusMessage(result.message, "info");
              }
            })
            .catch((err) => {
              setStatusMessage(
                `Apply failed: ${err?.message ?? String(err)}`,
                "error"
              );
            });
        },
        onCancel: () => {
          closeModal();
        },
      });
      return;
    }

    // Handle 'm' for moving entry to a different group
    if (input === "m" && selectedEntryId) {
      openModal("move-to-group", {
        entryId: selectedEntryId,
        onSelect: (targetGroupPath: string[]) => {
          moveEntry(selectedEntryId, targetGroupPath);

          // Persist changes to ~/.hosts
          writeHostsFile("~/.hosts", useAppStore.getState().hostsFile).catch((err) => {
            console.error("Failed to write hosts file:", err);
          });

          closeModal();
        },
        onCancel: () => {
          closeModal();
        },
      });
      return;
    }
  });
}
