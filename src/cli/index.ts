/**
 * CLI argument parser for Hostie
 * 
 * Uses commander to parse command-line arguments and return a structured result.
 */

import { Command } from "commander";

export type ParsedCLI = {
  command: string;
  subcommand?: string;
  args?: Record<string, any>;
  showHelp?: boolean;
  error?: string;
};

/**
 * Parse CLI arguments and return structured result
 * @param argv - Array of command-line arguments (without node/bun executable)
 * @returns Parsed command structure
 */
export function parseCLI(argv: string[]): ParsedCLI {
  // Handle empty args - default to TUI
  if (argv.length === 0) {
    return { command: "tui" };
  }

  const program = new Command();

  // Configure program metadata
  program
    .name("hostie")
    .version("0.1.0")
    .description("/etc/hosts TUI+CLI manager with YAML source of truth")
    .allowExcessArguments(false);

  // Track parsed result
  let result: ParsedCLI = { command: "tui" };

  // Add command: add entry
  program
    .command("add")
    .description("Add a new host entry")
    .argument("<ip>", "IP address (IPv4 or IPv6)")
    .argument("<hostname>", "Primary hostname")
    .option("--group <path>", "Group path (e.g., work/prod)")
    .option("--alias <name>", "Additional alias (can be specified multiple times)", collectArray, [])
    .option("--disabled", "Create entry as disabled")
    .option("--comment <text>", "Optional comment")
    .action((ip, hostname, options) => {
      result = {
        command: "add",
        args: {
          ip,
          hostname,
          ...(options.group && { group: options.group }),
          ...(options.alias.length > 0 && { aliases: options.alias }),
          ...(options.disabled && { disabled: true }),
          ...(options.comment && { comment: options.comment }),
        },
      };
    });

  // Rm command: remove entry
  program
    .command("rm")
    .description("Remove a host entry by hostname or ID")
    .argument("<target>", "Hostname or ULID to remove")
    .action((target) => {
      result = {
        command: "rm",
        args: { target },
      };
    });

  // Enable command: enable entry
  program
    .command("enable")
    .description("Enable a host entry")
    .argument("<hostname>", "Hostname to enable")
    .action((hostname) => {
      result = {
        command: "enable",
        args: { hostname },
      };
    });

  // Disable command: disable entry
  program
    .command("disable")
    .description("Disable a host entry")
    .argument("<hostname>", "Hostname to disable")
    .action((hostname) => {
      result = {
        command: "disable",
        args: { hostname },
      };
    });

  // List command: list entries
  program
    .command("list")
    .description("List host entries")
    .option("--group <path>", "Filter by group path")
    .option("--json", "Output as JSON")
    .action((options) => {
      result = {
        command: "list",
        args: {
          ...(options.group && { group: options.group }),
          ...(options.json && { json: true }),
        },
      };
    });

  // Apply command: write to /etc/hosts
  program
    .command("apply")
    .description("Apply changes to /etc/hosts")
    .option("--dry-run", "Show diff without writing")
    .action((options) => {
      result = {
        command: "apply",
        args: {
          ...(options.dryRun && { dryRun: true }),
        },
      };
    });

  // Group command with subcommands
  const groupCmd = program
    .command("group")
    .description("Manage groups");

  groupCmd
    .command("add")
    .description("Create a new group")
    .argument("<path>", "Group path (e.g., work/staging)")
    .action((path) => {
      result = {
        command: "group",
        subcommand: "add",
        args: { path },
      };
    });

  groupCmd
    .command("rm")
    .description("Remove a group")
    .argument("<path>", "Group path to remove")
    .option("--force", "Remove even if non-empty")
    .action((path, options) => {
      result = {
        command: "group",
        subcommand: "rm",
        args: {
          path,
          ...(options.force && { force: true }),
        },
      };
    });

  groupCmd
    .command("list")
    .description("List all groups as tree")
    .action(() => {
      result = {
        command: "group",
        subcommand: "list",
      };
    });

  groupCmd
    .command("mv")
    .description("Rename or move a group")
    .argument("<path>", "Current group path")
    .argument("<dest>", "Destination path")
    .action((path, dest) => {
      result = {
        command: "group",
        subcommand: "mv",
        args: { path, dest },
      };
    });

  // Completion command
  program
    .command("completion")
    .description("Generate shell completion script")
    .argument("<shell>", "Shell type (bash, zsh, fish)")
    .action((shell) => {
      result = {
        command: "completion",
        args: { shell },
      };
    });

  // Version command (explicit, in addition to --version flag)
  program
    .command("version")
    .description("Show version information")
    .action(() => {
      result = { command: "version" };
    });

  // Parse arguments
  try {
    program.exitOverride(); // Prevent process.exit()
    program.parse(argv, { from: "user" });
  } catch (err: any) {
    // Handle unknown commands or parsing errors
    if (err.code === "commander.unknownCommand") {
      result = {
        command: "unknown",
        error: err.message,
      };
    } else if (err.code === "commander.helpDisplayed") {
      result = { command: "help", showHelp: true };
    } else if (err.code === "commander.version") {
      result = { command: "version" };
    } else {
      result = {
        command: "error",
        error: err.message,
      };
    }
  }

  return result;
}

/**
 * Helper to collect multiple option values into an array
 */
function collectArray(value: string, previous: string[]): string[] {
  return previous.concat([value]);
}
