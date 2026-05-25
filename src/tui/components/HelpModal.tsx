/**
 * Help Modal Component
 * 
 * Displays all available keybindings organized by category.
 * Can be opened with '?' and closed with Esc or '?'.
 */
import React from "react";
import { Box, Text, useInput } from "ink";

/**
 * Props for HelpModal
 */
export interface HelpModalProps {
  /** Callback when modal is closed */
  onClose: () => void;
}

/**
 * Keybinding definition
 */
interface Keybinding {
  key: string;
  description: string;
}

/**
 * Keybinding category
 */
interface KeybindingCategory {
  title: string;
  bindings: Keybinding[];
}

/**
 * All keybindings organized by category
 */
const KEYBINDINGS: KeybindingCategory[] = [
  {
    title: "Navigation",
    bindings: [
      { key: "j", description: "Move down" },
      { key: "k", description: "Move up" },
      { key: "Tab", description: "Switch focus (sidebar ↔ main)" },
    ],
  },
  {
    title: "Actions",
    bindings: [
      { key: "Space", description: "Toggle entry enabled/disabled" },
      { key: "d", description: "Delete selected entry" },
      { key: "Enter", description: "Confirm action" },
    ],
  },
  {
    title: "Modals",
    bindings: [
      { key: "?", description: "Show/hide this help" },
      { key: "Esc", description: "Close modal or cancel" },
      { key: "y / n", description: "Quick Yes/No in confirmations" },
      { key: "← →", description: "Navigate buttons in modals" },
    ],
  },
  {
    title: "Entry Editor",
    bindings: [
      { key: "Tab", description: "Move to next field" },
      { key: "Shift+Tab", description: "Move to previous field" },
      { key: "Space", description: "Toggle enabled checkbox" },
      { key: "Ctrl+U", description: "Clear current field" },
      { key: "Backspace", description: "Delete character" },
    ],
  },
  {
    title: "General",
    bindings: [
      { key: "Ctrl+C", description: "Exit application" },
    ],
  },
];

/**
 * Help Modal
 * 
 * Displays a modal with all available keybindings organized by category.
 * Closes when user presses Escape or '?'.
 */
export function HelpModal({ onClose }: HelpModalProps) {
  useInput((input, key) => {
    // Handle Escape to close
    if (key.escape) {
      onClose();
      return;
    }

    // Handle '?' to toggle (close)
    if (input === "?") {
      onClose();
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
        borderColor="cyan"
        paddingX={3}
        paddingY={1}
        minWidth={60}
      >
        {/* Title */}
        <Box justifyContent="center" marginBottom={1}>
          <Text bold color="cyan">
            Help - Keyboard Shortcuts
          </Text>
        </Box>

        {/* Keybinding categories */}
        {KEYBINDINGS.map((category, categoryIndex) => (
          <Box key={categoryIndex} flexDirection="column" marginBottom={1}>
            {/* Category title */}
            <Text bold color="yellow">
              {category.title}
            </Text>

            {/* Keybindings in this category */}
            {category.bindings.map((binding, bindingIndex) => (
              <Box key={bindingIndex} marginLeft={2}>
                <Box width={15}>
                  <Text color="green">{binding.key}</Text>
                </Box>
                <Text dimColor>{binding.description}</Text>
              </Box>
            ))}
          </Box>
        ))}

        {/* Footer hint */}
        <Box justifyContent="center" marginTop={1} borderTop borderColor="gray">
          <Box marginTop={1}>
            <Text dimColor>Press </Text>
            <Text color="cyan">?</Text>
            <Text dimColor> or </Text>
            <Text color="cyan">Esc</Text>
            <Text dimColor> to close</Text>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
