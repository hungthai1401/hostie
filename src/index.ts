#!/usr/bin/env bun
/**
 * Main entry point for Hostie
 * Routes to TUI or CLI based on command-line arguments
 */

import { parseCLI } from "./cli/index.js";
import { renderTUI } from "./tui/index.jsx";

/**
 * Main entry point function
 * @param argv - Command-line arguments (without node/bun executable)
 * @returns Exit code
 */
export async function main(argv: string[]): Promise<number> {
  const parsed = parseCLI(argv);

  // Handle TUI mode (no args or explicit tui command)
  if (parsed.command === "tui") {
    const instance = renderTUI();
    await instance.waitUntilExit();
    return 0;
  }

  // Handle help
  if (parsed.command === "help" && parsed.showHelp) {
    // Help is already displayed by commander
    return 0;
  }

  // Handle version
  if (parsed.command === "version") {
    console.log("hostie version 0.1.0");
    return 0;
  }

  // Handle unknown command
  if (parsed.command === "unknown" && parsed.error) {
    console.error(`Error: ${parsed.error}`);
    console.error('Run "hostie --help" for usage information.');
    return 1;
  }

  // Handle parsing errors
  if (parsed.command === "error" && parsed.error) {
    console.error(`Error: ${parsed.error}`);
    return 1;
  }

  // Handle CLI commands
  // For now, just acknowledge the command was parsed
  // Actual command implementations will be wired in later beads
  console.log(`Command '${parsed.command}' parsed successfully`);
  if (parsed.subcommand) {
    console.log(`Subcommand: ${parsed.subcommand}`);
  }
  if (parsed.args) {
    console.log(`Arguments:`, parsed.args);
  }
  console.log("(Command execution not yet implemented)");
  
  return 0;
}

// Run if executed directly
if (import.meta.main) {
  const args = process.argv.slice(2);
  const exitCode = await main(args);
  process.exit(exitCode);
}
