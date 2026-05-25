import React, { useEffect, useState } from "react";
import { Box, Text } from "ink";
import { initializeStore, useAppStore } from "./store";
import { useKeyboard } from "./hooks/useKeyboard";
import { Layout } from "./components/Layout";
import { GroupTree } from "./components/GroupTree";
import { EntryList } from "./components/EntryList";
import { StatusBar } from "./components/StatusBar";
import { ConfirmModal } from "./components/ConfirmModal";
import { EntryEditorModal } from "./components/EntryEditorModal";
import { GroupCreatorModal } from "./components/GroupCreatorModal";
import { MoveToGroupModal } from "./components/MoveToGroupModal";
import { HelpModal } from "./components/HelpModal";
import type { Entry, Group } from "../domain/types";

/**
 * Returns true if stdin appears to be an interactive TTY that supports
 * raw mode. Used to skip Ink's useInput-based keyboard handling in
 * non-interactive environments (test runners, piped invocations) where
 * Ink would otherwise throw "Raw mode is not supported".
 */
function stdinSupportsRawMode(): boolean {
  const stdin = process.stdin as NodeJS.ReadStream & { isTTY?: boolean };
  return Boolean(stdin && stdin.isTTY);
}

/**
 * Lightweight mount point for the keyboard hook so we can conditionally
 * register Ink's useInput only when the surrounding environment supports
 * it. Returning null keeps the layout untouched.
 */
function KeyboardHandler() {
  useKeyboard();
  return null;
}

/**
 * Find a group inside the hosts-file tree by its path.
 */
function findGroupByPath(groups: Group[], path: string[]): Group | null {
  if (path.length === 0) return null;
  const [first, ...rest] = path;
  const match = groups.find((g) => g.name === first);
  if (!match) return null;
  if (rest.length === 0) return match;
  return findGroupByPath(match.groups, rest);
}

/**
 * Collect every entry in the tree (depth-first). Used when no group
 * is explicitly selected so the user still sees something useful.
 */
function collectAllEntries(groups: Group[]): Entry[] {
  const out: Entry[] = [];
  for (const group of groups) {
    out.push(...group.entries);
    out.push(...collectAllEntries(group.groups));
  }
  return out;
}

/**
 * Modal host — renders the modal that matches the current store
 * modal state, wiring callbacks back through the store.
 */
function ModalHost() {
  const modal = useAppStore((s) => s.modal);
  const hostsFile = useAppStore((s) => s.hostsFile);
  const closeModal = useAppStore((s) => s.closeModal);
  const addGroup = useAppStore((s) => s.addGroup);

  if (!modal) return null;

  switch (modal.type) {
    case "confirmation":
      return (
        <ConfirmModal
          message={modal.data?.message ?? "Are you sure?"}
          onConfirm={() => {
            if (modal.data?.onConfirm) modal.data.onConfirm();
            else closeModal();
          }}
          onCancel={() => {
            if (modal.data?.onCancel) modal.data.onCancel();
            else closeModal();
          }}
        />
      );

    case "help":
      return <HelpModal onClose={closeModal} />;

    case "move-to-group":
      return (
        <MoveToGroupModal
          groups={hostsFile.groups}
          onSelect={(path) => {
            if (modal.data?.onSelect) modal.data.onSelect(path);
            else closeModal();
          }}
          onCancel={() => {
            if (modal.data?.onCancel) modal.data.onCancel();
            else closeModal();
          }}
        />
      );

    case "entry-creator":
      return (
        <EntryEditorModal
          mode="add"
          onSave={() => closeModal()}
          onCancel={closeModal}
        />
      );

    case "entry-editor":
      return (
        <EntryEditorModal
          mode="edit"
          entry={modal.data?.entry}
          onSave={() => closeModal()}
          onCancel={closeModal}
        />
      );

    case "group-creator":
      return (
        <GroupCreatorModal
          onClose={closeModal}
          onSubmit={(name, parent) => {
            addGroup(name, parent);
            closeModal();
          }}
          parentPath={modal.data?.parentPath}
        />
      );

    default:
      return null;
  }
}

/**
 * Root TUI application component.
 *
 * Composes the three-panel Layout (group tree, entry list, status bar),
 * loads the user's hosts file on mount, registers keyboard handling
 * when stdin supports it, and overlays the current modal (if any).
 */
export function App() {
  const hostsFile = useAppStore((s) => s.hostsFile);
  const selectedGroupPath = useAppStore((s) => s.selectedGroupPath);
  const selectedEntryId = useAppStore((s) => s.selectedEntryId);
  const mode = useAppStore((s) => s.mode);
  const modal = useAppStore((s) => s.modal);
  const dirty = useAppStore((s) => s.dirty);
  const statusMessage = useAppStore((s) => s.statusMessage);
  const selectEntry = useAppStore((s) => s.selectEntry);
  const selectGroup = useAppStore((s) => s.selectGroup);

  const [collapsedPaths, setCollapsedPaths] = useState<Set<string>>(new Set());

  // Load ~/.hosts on first mount. Errors are surfaced via console but
  // do not crash the TUI — the user simply starts with an empty file.
  useEffect(() => {
    initializeStore().catch((err) => {
      // eslint-disable-next-line no-console
      console.error("Failed to load hosts file:", err);
    });
  }, []);

  const selectedGroup =
    selectedGroupPath.length > 0
      ? findGroupByPath(hostsFile.groups, selectedGroupPath)
      : null;

  const displayedEntries: Entry[] = selectedGroup
    ? selectedGroup.entries
    : collectAllEntries(hostsFile.groups);

  const helpHint = `? help • j/k nav • space toggle • enter apply • q quit${
    dirty ? " •" : ""
  }`;

  const inputEnabled = stdinSupportsRawMode();
  const statusMode: "normal" | "search" | "edit" =
    mode === "modal" ? "normal" : mode === "search" ? "search" : mode === "edit" ? "edit" : "normal";

  const handleToggleCollapse = (path: string[]) => {
    const key = path.join("/");
    setCollapsedPaths((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  return (
    <Box flexDirection="column">
      {inputEnabled && <KeyboardHandler />}

      {modal ? (
        <ModalHost />
      ) : (
        <Layout>
          <Layout.Sidebar>
            <GroupTree
              groups={hostsFile.groups}
              selectedPath={selectedGroupPath}
              collapsedPaths={collapsedPaths}
              onSelect={selectGroup}
              onToggleCollapse={handleToggleCollapse}
            />
          </Layout.Sidebar>

          <Layout.Main>
            <EntryList
              entries={displayedEntries}
              selectedId={selectedEntryId}
              onSelect={selectEntry}
            />
          </Layout.Main>

          <Layout.StatusBar>
            <Box flexDirection="column">
              <StatusBar mode={statusMode} helpHint={helpHint} />
              {statusMessage && (
                <Box paddingX={1}>
                  <Text
                    color={
                      statusMessage.level === "error"
                        ? "red"
                        : statusMessage.level === "success"
                          ? "green"
                          : undefined
                    }
                  >
                    {statusMessage.text}
                  </Text>
                </Box>
              )}
            </Box>
          </Layout.StatusBar>
        </Layout>
      )}
    </Box>
  );
}
