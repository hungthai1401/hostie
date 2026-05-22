import React from "react";
import { Box, Text } from "ink";
import type { Group } from "../../domain/types";

export interface GroupTreeProps {
  /** Groups to display */
  groups: Group[];
  /** Currently selected group path */
  selectedPath: string[];
  /** Set of collapsed group paths (stored as path strings joined by '/') */
  collapsedPaths: Set<string>;
  /** Callback when a group is selected */
  onSelect: (path: string[]) => void;
  /** Callback when a group collapse state is toggled */
  onToggleCollapse: (path: string[]) => void;
}

/**
 * GroupTree component - displays groups hierarchically with navigation
 * 
 * Features:
 * - Hierarchical display of groups
 * - Entry count per group
 * - Collapsible groups with arrow indicators
 * - Highlight selected group
 * - Keyboard navigation (j/k keys handled by parent)
 */
export function GroupTree({
  groups,
  selectedPath,
  collapsedPaths,
  onSelect,
  onToggleCollapse,
}: GroupTreeProps) {
  if (groups.length === 0) {
    return (
      <Box flexDirection="column">
        <Text dimColor>No groups</Text>
      </Box>
    );
  }

  /**
   * Recursively render a group and its children
   */
  function renderGroup(group: Group, path: string[], depth: number): React.ReactNode {
    const currentPath = [...path, group.name];
    const pathKey = currentPath.join("/");
    const isSelected = arraysEqual(currentPath, selectedPath);
    const isCollapsed = collapsedPaths.has(pathKey);
    const hasSubgroups = group.groups.length > 0;
    const entryCount = group.entries.length;

    // Indentation based on depth
    const indent = "  ".repeat(depth);

    // Arrow indicator for groups with subgroups
    const arrow = hasSubgroups ? (isCollapsed ? "▸ " : "▼ ") : "  ";

    // Build the display text
    const displayText = `${indent}${arrow}${group.name} (${entryCount})`;

    return (
      <React.Fragment key={pathKey}>
        <Box>
          <Text
            color={isSelected ? "cyan" : undefined}
            bold={isSelected}
            inverse={isSelected}
          >
            {displayText}
          </Text>
        </Box>

        {/* Render nested groups if not collapsed */}
        {hasSubgroups && !isCollapsed && (
          <>
            {group.groups.map((subgroup) =>
              renderGroup(subgroup, currentPath, depth + 1)
            )}
          </>
        )}
      </React.Fragment>
    );
  }

  return (
    <Box flexDirection="column">
      {groups.map((group) => renderGroup(group, [], 0))}
    </Box>
  );
}

/**
 * Helper: Compare two arrays for equality
 */
function arraysEqual(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  return a.every((val, idx) => val === b[idx]);
}
