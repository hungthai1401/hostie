import React, { useState, useMemo } from "react";
import { Box, Text, useInput } from "ink";
import type { Group } from "../../domain/types";

/**
 * MoveToGroupModal component props
 */
export interface MoveToGroupModalProps {
  /** Available groups to move entry to */
  groups: Group[];
  /** Callback when a group is selected */
  onSelect: (groupPath: string[]) => void;
  /** Callback when modal is cancelled */
  onCancel: () => void;
}

/**
 * Flattened group representation for list display
 */
interface FlatGroup {
  /** Full path to the group (e.g., ["work", "prod"]) */
  path: string[];
  /** Display name with path (e.g., "work/prod") */
  displayName: string;
  /** Indentation level for nested groups */
  level: number;
}

/**
 * Flatten nested groups into a list for navigation
 */
function flattenGroups(groups: Group[], parentPath: string[] = [], level: number = 0): FlatGroup[] {
  const result: FlatGroup[] = [];
  
  for (const group of groups) {
    const currentPath = [...parentPath, group.name];
    const displayName = currentPath.join("/");
    
    result.push({
      path: currentPath,
      displayName,
      level,
    });
    
    // Recursively add nested groups
    if (group.groups.length > 0) {
      result.push(...flattenGroups(group.groups, currentPath, level + 1));
    }
  }
  
  return result;
}

/**
 * MoveToGroupModal component - modal for moving an entry to a different group
 * 
 * Features:
 * - Shows list of available groups (flattened hierarchy)
 * - Navigate with j/k keys (vim-style)
 * - Select with Enter
 * - Cancel with Escape
 * - Shows nested groups with path notation (e.g., "work/prod")
 * 
 * @example
 * ```tsx
 * <MoveToGroupModal
 *   groups={hostsFile.groups}
 *   onSelect={(path) => moveEntry(entryId, path)}
 *   onCancel={() => closeModal()}
 * />
 * ```
 */
export function MoveToGroupModal({ groups, onSelect, onCancel }: MoveToGroupModalProps) {
  // Flatten groups for list navigation
  const flatGroups = useMemo(() => flattenGroups(groups), [groups]);
  
  // Track selected index
  const [selectedIndex, setSelectedIndex] = useState(0);
  
  // Handle keyboard input
  useInput((input, key) => {
    // Escape to cancel
    if (key.escape) {
      onCancel();
      return;
    }
    
    // Enter to select
    if (key.return) {
      if (flatGroups.length > 0) {
        onSelect(flatGroups[selectedIndex].path);
      }
      return;
    }
    
    // j or down arrow to move down
    if (input === "j" || key.downArrow) {
      setSelectedIndex((prev) => Math.min(prev + 1, flatGroups.length - 1));
      return;
    }
    
    // k or up arrow to move up
    if (input === "k" || key.upArrow) {
      setSelectedIndex((prev) => Math.max(prev - 1, 0));
      return;
    }
  });
  
  return (
    <Box
      flexDirection="column"
      borderStyle="round"
      borderColor="cyan"
      paddingX={2}
      paddingY={1}
      width={60}
    >
      {/* Title */}
      <Box marginBottom={1}>
        <Text bold color="cyan">
          Move to Group
        </Text>
      </Box>
      
      {/* Group list */}
      <Box flexDirection="column" marginBottom={1}>
        {flatGroups.length === 0 ? (
          <Text dimColor>No groups available</Text>
        ) : (
          flatGroups.map((group, index) => {
            const isSelected = index === selectedIndex;
            const indent = "  ".repeat(group.level);
            
            return (
              <Box key={group.displayName}>
                <Text
                  color={isSelected ? "black" : "white"}
                  backgroundColor={isSelected ? "cyan" : undefined}
                  bold={isSelected}
                >
                  {indent}
                  {isSelected ? "▶ " : "  "}
                  {group.displayName}
                </Text>
              </Box>
            );
          })
        )}
      </Box>
      
      {/* Help hint */}
      <Box marginTop={1}>
        <Text dimColor>
          j/k to navigate • Enter to select • Esc to cancel
        </Text>
      </Box>
    </Box>
  );
}
