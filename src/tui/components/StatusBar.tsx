import React from "react";
import { Box, Text } from "ink";

export type Mode = "normal" | "search" | "edit";

export interface StatusBarProps {
  mode: Mode;
  helpHint: string;
}

/**
 * Status bar component displayed at the bottom of the TUI
 * Shows current mode on the left and help hint on the right
 */
export function StatusBar({ mode, helpHint }: StatusBarProps) {
  return (
    <Box justifyContent="space-between" paddingX={1}>
      <Text inverse> {mode.toUpperCase()} </Text>
      <Text inverse dimColor> {helpHint} </Text>
    </Box>
  );
}
