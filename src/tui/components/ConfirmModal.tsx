import React, { useState } from "react";
import { Box, Text, useInput } from "ink";

/**
 * ConfirmModal component props
 */
export interface ConfirmModalProps {
  /** Message to display in the modal */
  message: string;
  /** Callback when user confirms (Yes) */
  onConfirm: () => void;
  /** Callback when user cancels (No or Escape) */
  onCancel: () => void;
}

/**
 * ConfirmModal component - displays a confirmation dialog for destructive actions
 * 
 * Features:
 * - Shows message with Yes/No buttons
 * - Navigate with arrow keys (left/right)
 * - Quick confirm with 'y' key
 * - Quick cancel with 'n' key or Escape
 * - Confirm selection with Enter
 * - Centered modal with border
 * 
 * @example
 * ```tsx
 * <ConfirmModal
 *   message="Delete this entry?"
 *   onConfirm={() => handleDelete()}
 *   onCancel={() => setShowModal(false)}
 * />
 * ```
 */
export function ConfirmModal({ message, onConfirm, onCancel }: ConfirmModalProps) {
  // Track which button is selected: 0 = Yes, 1 = No
  const [selectedButton, setSelectedButton] = useState<0 | 1>(0);

  useInput((input, key) => {
    // Handle Escape key
    if (key.escape) {
      onCancel();
      return;
    }

    // Handle Enter key - confirm current selection
    if (key.return) {
      if (selectedButton === 0) {
        onConfirm();
      } else {
        onCancel();
      }
      return;
    }

    // Handle arrow key navigation
    if (key.leftArrow) {
      setSelectedButton(0);
      return;
    }

    if (key.rightArrow) {
      setSelectedButton(1);
      return;
    }

    // Handle 'y' key for quick Yes
    if (input === "y" || input === "Y") {
      onConfirm();
      return;
    }

    // Handle 'n' key for quick No
    if (input === "n" || input === "N") {
      onCancel();
      return;
    }
  });

  return (
    <Box
      flexDirection="column"
      alignItems="center"
      justifyContent="center"
      paddingX={2}
      paddingY={1}
    >
      {/* Modal container with border */}
      <Box
        flexDirection="column"
        borderStyle="round"
        borderColor="yellow"
        paddingX={3}
        paddingY={1}
        minWidth={40}
      >
        {/* Message */}
        <Box justifyContent="center" marginBottom={1}>
          <Text bold>{message}</Text>
        </Box>

        {/* Buttons */}
        <Box justifyContent="center" gap={2}>
          <Box paddingX={2}>
            <Text
              color={selectedButton === 0 ? "black" : "green"}
              backgroundColor={selectedButton === 0 ? "green" : undefined}
              bold={selectedButton === 0}
            >
              Yes
            </Text>
          </Box>

          <Box paddingX={2}>
            <Text
              color={selectedButton === 1 ? "black" : "red"}
              backgroundColor={selectedButton === 1 ? "red" : undefined}
              bold={selectedButton === 1}
            >
              No
            </Text>
          </Box>
        </Box>

        {/* Help hint */}
        <Box justifyContent="center" marginTop={1}>
          <Text dimColor>
            ← → to navigate • Enter to confirm • Esc to cancel
          </Text>
        </Box>
      </Box>
    </Box>
  );
}
