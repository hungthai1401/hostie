import React from "react";
import { render, type Instance } from "ink";
import { Text, Box } from "ink";

/**
 * Main TUI application component
 */
function App() {
  return (
    <Box flexDirection="column" padding={1}>
      <Text bold color="cyan">
        Hostie - /etc/hosts Manager
      </Text>
      <Text dimColor>Press Ctrl+C to exit</Text>
    </Box>
  );
}

/**
 * Renders the TUI application
 * @returns Ink instance with unmount and waitUntilExit methods
 */
export function renderTUI(): Instance {
  return render(<App />);
}
