import React from "react";
import { Box, Text } from "ink";
import type { Entry } from "../../domain/types";

/**
 * EntryList component props
 */
export interface EntryListProps {
  /** Array of entries to display */
  entries: Entry[];
  /** ID of the currently selected entry */
  selectedId: string | null;
  /** Callback when an entry is selected */
  onSelect: (id: string) => void;
}

/**
 * EntryList component - displays host entries in a table format
 * 
 * Shows entries with columns for enabled status (checkbox), hostname, IP, and aliases.
 * Highlights the selected entry and provides keyboard navigation with j/k keys.
 * 
 * @example
 * ```tsx
 * <EntryList
 *   entries={entries}
 *   selectedId={selectedId}
 *   onSelect={(id) => setSelectedId(id)}
 * />
 * ```
 */
export function EntryList({ entries, selectedId, onSelect }: EntryListProps) {
  // Empty state
  if (entries.length === 0) {
    return (
      <Box flexDirection="column" paddingX={2} paddingY={1}>
        <Text dimColor>No entries in this group</Text>
        <Text dimColor>Press 'a' to add a new entry</Text>
      </Box>
    );
  }

  return (
    <Box flexDirection="column">
      {/* Column headers */}
      <Box paddingX={1} paddingY={0}>
        <Box width={4}>
          <Text bold dimColor>
            ✓
          </Text>
        </Box>
        <Box width={30}>
          <Text bold dimColor>
            Hostname
          </Text>
        </Box>
        <Box width={20}>
          <Text bold dimColor>
            IP
          </Text>
        </Box>
        <Box flexGrow={1}>
          <Text bold dimColor>
            Aliases
          </Text>
        </Box>
      </Box>

      {/* Entry rows */}
      {entries.map((entry) => {
        const isSelected = entry.id === selectedId;
        const enabledIndicator = entry.enabled ? "✓" : "✗";
        const aliasesText = entry.aliases.length > 0 ? entry.aliases.join(", ") : "";

        return (
          <Box key={entry.id} paddingX={1}>
            <Box width={4}>
              <Text color={entry.enabled ? "green" : "red"} inverse={isSelected}>
                {enabledIndicator}
              </Text>
            </Box>
            <Box width={30}>
              <Text inverse={isSelected} bold={isSelected}>
                {entry.hostname}
              </Text>
            </Box>
            <Box width={20}>
              <Text inverse={isSelected} dimColor={!isSelected}>
                {entry.ip}
              </Text>
            </Box>
            <Box flexGrow={1}>
              <Text inverse={isSelected} dimColor={!isSelected}>
                {aliasesText}
              </Text>
            </Box>
          </Box>
        );
      })}
    </Box>
  );
}
