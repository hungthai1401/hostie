#!/usr/bin/env bun
/**
 * Main entry point for Hostie
 * Routes to TUI or CLI based on command-line arguments
 */

import { parseCLI } from "./cli/index.js";
import { renderTUI } from "./tui/index.jsx";
import { addCommand } from "./cli/commands/add.js";
import { rmCommand } from "./cli/commands/rm.js";
import { enableCommand } from "./cli/commands/enable.js";
import { disableCommand } from "./cli/commands/disable.js";
import { listCommand } from "./cli/commands/list.js";
import { applyCommand } from "./cli/commands/apply.js";
import { groupCreateCommand } from "./cli/commands/group.js";
import { completionCommand, type Shell } from "./cli/commands/completion.js";
import { versionCommand } from "./cli/commands/version.js";
import { ExitCode } from "./cli/exit-codes.js";

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
    return ExitCode.SUCCESS;
  }

  // Handle help (commander already printed it)
  if (parsed.command === "help" && parsed.showHelp) {
    return ExitCode.SUCCESS;
  }

  // Handle version
  if (parsed.command === "version") {
    return await versionCommand();
  }

  // Handle unknown command / parsing errors
  if (parsed.command === "unknown" && parsed.error) {
    console.error(`Error: ${parsed.error}`);
    console.error('Run "hostie --help" for usage information.');
    return ExitCode.VALIDATION;
  }

  if (parsed.command === "error" && parsed.error) {
    console.error(`Error: ${parsed.error}`);
    return ExitCode.VALIDATION;
  }

  // Dispatch to command handlers (D14: validation errors exit non-zero)
  const args = parsed.args ?? {};

  switch (parsed.command) {
    case "add": {
      const aliases: string[] = Array.isArray(args.aliases) ? args.aliases : [];
      return await addCommand(args.ip, args.hostname, aliases, {
        group: args.group,
        disabled: args.disabled,
        comment: args.comment,
      });
    }

    case "rm":
      return await rmCommand(args.target);

    case "enable":
      return await enableCommand(args.hostname);

    case "disable":
      return await disableCommand(args.hostname);

    case "list":
      return await listCommand({
        json: args.json,
      });

    case "apply":
      return await applyCommand({
        dryRun: args.dryRun,
      });

    case "group":
      return await dispatchGroup(parsed.subcommand, args);

    case "completion":
      return await completionCommand(args.shell as Shell);

    default:
      console.error(`Error: Unknown command '${parsed.command}'`);
      console.error('Run "hostie --help" for usage information.');
      return ExitCode.VALIDATION;
  }
}

/**
 * Dispatch group subcommands. Only `group add` is wired to a handler in
 * v1; `rm`, `list`, and `mv` are reserved subcommands without
 * implementations yet — they surface a clear "not implemented" error
 * rather than the previous silent no-op, so scripts can detect the gap
 * via the non-zero exit code (D14).
 */
async function dispatchGroup(
  subcommand: string | undefined,
  args: Record<string, any>
): Promise<number> {
  switch (subcommand) {
    case "add": {
      // "group add <path>" — path may be nested (e.g. "work/prod").
      // Split into parent path + leaf name so the handler's kebab-case
      // validator (no slashes) is satisfied.
      const path: string = args.path ?? "";
      const segments = path.split("/").filter((s: string) => s.length > 0);

      if (segments.length === 0) {
        console.error("Error: Group path cannot be empty");
        return ExitCode.VALIDATION;
      }

      const name = segments[segments.length - 1];
      const parent = segments.slice(0, -1).join("/");
      return await groupCreateCommand(name, parent ? { parent } : {});
    }

    case "rm":
    case "list":
    case "mv":
      console.error(`Error: 'group ${subcommand}' is not implemented yet.`);
      return ExitCode.VALIDATION;

    default:
      console.error(`Error: Unknown group subcommand '${subcommand ?? ""}'`);
      console.error('Run "hostie group --help" for usage information.');
      return ExitCode.VALIDATION;
  }
}

// Run if executed directly
if (import.meta.main) {
  const args = process.argv.slice(2);
  const exitCode = await main(args);
  process.exit(exitCode);
}
