/**
 * Group Creator Modal Component
 * 
 * Modal for creating new groups with name validation.
 */
import React, { useState } from "react";
import { Box, Text, useInput } from "ink";

/**
 * Props for GroupCreatorModal
 */
export interface GroupCreatorModalProps {
  /** Callback when modal is closed without saving */
  onClose: () => void;
  /** Callback when group is created with valid name */
  onSubmit: (name: string, parentPath?: string[]) => void;
  /** Optional parent group path (e.g., ["work", "prod"]) */
  parentPath?: string[];
}

/**
 * Validates a group name (must be kebab-case)
 * 
 * Rules:
 * - Lowercase letters, numbers, and hyphens only
 * - Cannot start or end with a hyphen
 * - Cannot contain consecutive hyphens
 * - No slashes (slashes are path separators, not part of a single group name)
 * - At least 1 character
 */
function validateGroupName(name: string): { valid: boolean; error?: string } {
  // Check for empty string
  if (!name || name.length === 0) {
    return {
      valid: false,
      error: "Group name cannot be empty",
    };
  }

  // Check for slashes (path separators)
  if (name.includes("/")) {
    return {
      valid: false,
      error: "Group name cannot contain slashes (use parent group for nesting)",
    };
  }

  // Check for leading or trailing hyphen
  if (name.startsWith("-")) {
    return {
      valid: false,
      error: "Group name cannot start with a hyphen",
    };
  }

  if (name.endsWith("-")) {
    return {
      valid: false,
      error: "Group name cannot end with a hyphen",
    };
  }

  // Check for consecutive hyphens
  if (name.includes("--")) {
    return {
      valid: false,
      error: "Group name cannot contain consecutive hyphens",
    };
  }

  // Check for valid kebab-case characters (lowercase letters, numbers, hyphens)
  if (!/^[a-z0-9-]+$/.test(name)) {
    return {
      valid: false,
      error: "Group name must be kebab-case (lowercase letters, numbers, and hyphens only)",
    };
  }

  return {
    valid: true,
  };
}

/**
 * Group Creator Modal
 * 
 * Displays a modal form for creating a new group with:
 * - Name input field (validates kebab-case)
 * - Optional parent group display
 * - Submit on Enter (validates first)
 * - Cancel on Escape
 */
export function GroupCreatorModal({
  onClose,
  onSubmit,
  parentPath = [],
}: GroupCreatorModalProps) {
  const [name, setName] = useState("");
  const [error, setError] = useState<string | undefined>(undefined);

  useInput((input, key) => {
    // Handle Escape to cancel
    if (key.escape) {
      onClose();
      return;
    }

    // Handle Enter to submit
    if (key.return) {
      const validation = validateGroupName(name);
      if (!validation.valid) {
        setError(validation.error);
        return;
      }

      // Valid name - submit
      setError(undefined);
      onSubmit(name, parentPath.length > 0 ? parentPath : undefined);
      return;
    }

    // Handle Backspace
    if (key.backspace || key.delete) {
      setName((prev) => prev.slice(0, -1));
      setError(undefined);
      return;
    }

    // Handle regular character input
    if (input && !key.ctrl && !key.meta) {
      setName((prev) => prev + input);
      setError(undefined);
    }
  });

  const parentPathStr = parentPath.length > 0 ? parentPath.join("/") : null;

  return (
    <Box
      flexDirection="column"
      borderStyle="round"
      borderColor="cyan"
      paddingX={2}
      paddingY={1}
      width={60}
    >
      <Text bold color="cyan">
        Create Group
      </Text>

      <Box marginTop={1} />

      {parentPathStr && (
        <>
          <Box>
            <Text dimColor>Parent: </Text>
            <Text color="yellow">{parentPathStr}</Text>
          </Box>
          <Box marginTop={1} />
        </>
      )}

      <Box>
        <Text>Name: </Text>
        <Text color="green">{name}</Text>
        <Text dimColor>█</Text>
      </Box>

      {error && (
        <>
          <Box marginTop={1} />
          <Text color="red">✗ {error}</Text>
        </>
      )}

      <Box marginTop={1} />

      <Box>
        <Text dimColor>[Enter] Save  [Esc] Cancel</Text>
      </Box>
    </Box>
  );
}
