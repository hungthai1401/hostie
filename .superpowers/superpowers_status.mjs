#!/usr/bin/env node

/**
 * Superpowers Status Scout
 *
 * Read-only CLI that aggregates onboarding, state, and handoff files into a
 * single status snapshot. Runs from the superpowers skills directory.
 *
 * Usage: superpowers_status.mjs [--repo-root <path>] [--json]
 *
 * Zero external dependencies — Node.js built-ins only.
 */

import path from "node:path";
import { fileURLToPath } from "node:url";

import {
  readSuperpowersStatus,
  renderSuperpowersStatus,
  resolveRepoRoot,
} from "./superpowers_state.mjs";

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url));

function parseCliArgs(argv) {
  const args = {
    repoRoot: undefined,
    json: false,
  };

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (arg === "--repo-root") {
      args.repoRoot = argv[index + 1];
      index += 1;
      continue;
    }
    if (arg.startsWith("--repo-root=")) {
      args.repoRoot = arg.slice("--repo-root=".length);
      continue;
    }
    if (arg === "--json") {
      args.json = true;
      continue;
    }
    if (arg === "--help" || arg === "-h") {
      process.stdout.write(
        [
          "Usage: superpowers_status.mjs [--repo-root <path>] [--json]",
          "",
          "Shows a read-only superpowers status snapshot from onboarding, state, and handoff files.",
        ].join("\n"),
      );
      process.exit(0);
    }
    throw new Error(`Unknown argument: ${arg}`);
  }

  return args;
}

export function main(argv = process.argv.slice(2)) {
  const args = parseCliArgs(argv);
  const repoRoot = resolveRepoRoot(args.repoRoot, SCRIPT_DIR);
  const status = readSuperpowersStatus(repoRoot);

  process.stdout.write(
    args.json ? `${JSON.stringify(status, null, 2)}\n` : `${renderSuperpowersStatus(status)}\n`,
  );
  return 0;
}

if (process.argv[1]) {
  process.exitCode = main();
}
