#!/usr/bin/env node

/**
 * Superpowers File Reservations CLI
 *
 * Local file-based coordination for parallel worker execution.
 * Replaces Agent Mail MCP server with lightweight JSON-based state.
 *
 * Commands:
 *   reserve  - Reserve file paths for exclusive or shared access
 *   release  - Release reservations held by an agent
 *   list     - Show active reservations
 *   sweep    - Remove expired reservations
 *
 * Usage:
 *   reservations.mjs reserve --agent <name> --bead <id> --paths <glob> [--ttl <seconds>] [--json]
 *   reservations.mjs release --agent <name> [--paths <glob>] [--json]
 *   reservations.mjs list [--agent <name>] [--json]
 *   reservations.mjs sweep [--json]
 *
 * Zero external dependencies — Node.js built-ins only.
 */

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url));
const RESERVATIONS_FILE = process.env.SUPERPOWERS_RESERVATIONS_FILE
  ? path.resolve(process.env.SUPERPOWERS_RESERVATIONS_FILE)
  : path.join(SCRIPT_DIR, "reservations.json");

// ============================================================================
// Inline Glob Matcher (zero dependencies)
// ============================================================================

/**
 * Convert a glob pattern to a RegExp.
 * Supports: *, **, ?, but not brace expansion or negation.
 */
function globToRegex(pattern) {
  // Escape special regex characters except glob wildcards
  let regex = pattern
    .replace(/[.+^${}()|[\]\\]/g, "\\$&")
    .replace(/\*\*/g, "\x00") // Placeholder for **
    .replace(/\*/g, "[^/]*") // * matches anything except /
    .replace(/\?/g, "[^/]") // ? matches single char except /
    .replace(/\x00/g, ".*"); // ** matches anything including /

  return new RegExp(`^${regex}$`);
}

/**
 * Check if a path matches a glob pattern.
 */
function matchesGlob(path, pattern) {
  const regex = globToRegex(pattern);
  return regex.test(path);
}

/**
 * Symmetric pattern matching: A conflicts with B if either pattern matches the other.
 * This handles cases like:
 *   - "src/foo.ts" conflicts with "src/**"
 *   - "src/**" conflicts with "src/foo.ts"
 *   - "src/*.ts" conflicts with "src/foo.ts"
 */
function patternsConflict(patternA, patternB) {
  // Exact match
  if (patternA === patternB) return true;

  // Symmetric glob matching
  return matchesGlob(patternA, patternB) || matchesGlob(patternB, patternA);
}

// ============================================================================
// Pattern Validation (security + ReDoS protection)
// ============================================================================

const MAX_PATTERN_LENGTH = 500;
const MAX_DOUBLE_STAR = 5;
const MAX_SINGLE_STAR = 20;
const MIN_TTL_SECONDS = 60;
const MAX_TTL_SECONDS = 7200;
const DEFAULT_TTL_SECONDS = 600;

/**
 * Validate a single glob pattern. Throws Error on rejection.
 * Guards against: empty patterns, length DoS, ReDoS via wildcard explosion,
 * absolute paths, parent-directory traversal, unclosed regex brackets.
 */
function validatePattern(pattern) {
  if (typeof pattern !== "string" || pattern.length === 0) {
    throw new Error(`Invalid pattern: empty or non-string`);
  }
  if (pattern.length > MAX_PATTERN_LENGTH) {
    throw new Error(`Pattern too long (>${MAX_PATTERN_LENGTH} chars): ${pattern.slice(0, 60)}...`);
  }
  // Path traversal: reject absolute paths and any '..' segment
  if (pattern.startsWith("/") || /^[A-Za-z]:[\\/]/.test(pattern)) {
    throw new Error(`Absolute paths not allowed: ${pattern}`);
  }
  const segments = pattern.split(/[\\/]/);
  if (segments.some((s) => s === "..")) {
    throw new Error(`Parent-directory traversal ('..') not allowed: ${pattern}`);
  }
  // ReDoS protection: cap wildcard density
  const doubleStarCount = (pattern.match(/\*\*/g) || []).length;
  if (doubleStarCount > MAX_DOUBLE_STAR) {
    throw new Error(`Too many '**' wildcards (>${MAX_DOUBLE_STAR}): ${pattern}`);
  }
  // Count single * (not part of **)
  const singleStarCount = (pattern.replace(/\*\*/g, "").match(/\*/g) || []).length;
  if (singleStarCount > MAX_SINGLE_STAR) {
    throw new Error(`Too many '*' wildcards (>${MAX_SINGLE_STAR}): ${pattern}`);
  }
  // Verify regex compiles (catches unclosed brackets etc.)
  try {
    globToRegex(pattern);
  } catch (err) {
    throw new Error(`Invalid glob pattern '${pattern}': ${err.message}`);
  }
}

/**
 * Validate an array of patterns; throws on first failure.
 */
function validatePatterns(patterns) {
  if (!Array.isArray(patterns) || patterns.length === 0) {
    throw new Error(`--paths must be a non-empty list (got: ${JSON.stringify(patterns)})`);
  }
  for (const p of patterns) validatePattern(p);
}

/**
 * Validate TTL is within accepted bounds.
 */
function validateTtl(ttl) {
  if (process.env.SUPERPOWERS_RESERVATIONS_NO_TTL_BOUNDS === "1") {
    if (!Number.isFinite(ttl) || ttl <= 0) {
      throw new Error(`--ttl must be a positive number (got: ${ttl})`);
    }
    return;
  }
  if (!Number.isFinite(ttl) || ttl < MIN_TTL_SECONDS || ttl > MAX_TTL_SECONDS) {
    throw new Error(
      `--ttl must be between ${MIN_TTL_SECONDS}s and ${MAX_TTL_SECONDS}s (got: ${ttl})`
    );
  }
}

// ============================================================================
// File Locking (concurrent safety)
// ============================================================================

const LOCK_FILE = `${RESERVATIONS_FILE}.lock`;
const LOCK_RETRY_MS = 50;
const LOCK_TIMEOUT_MS = 2000;
const LOCK_STALE_MS = 30000;

function sleepSync(ms) {
  const end = Date.now() + ms;
  while (Date.now() < end) {
    // Busy-wait; intentional for sync CLI semantics.
  }
}

/**
 * Acquire an exclusive advisory lock on the reservations file, run fn(), release.
 * Uses `wx` open (exclusive create) with retry. Reaps stale locks older than 30s.
 */
function withLock(fn) {
  const deadline = Date.now() + LOCK_TIMEOUT_MS;
  let fd;
  while (true) {
    try {
      fd = fs.openSync(LOCK_FILE, "wx");
      fs.writeSync(fd, String(process.pid));
      break;
    } catch (err) {
      if (err.code !== "EEXIST") throw err;
      // Check if existing lock is stale
      try {
        const stat = fs.statSync(LOCK_FILE);
        if (Date.now() - stat.mtimeMs > LOCK_STALE_MS) {
          fs.unlinkSync(LOCK_FILE);
          continue;
        }
      } catch (_) {
        // Lock vanished; loop and retry create.
      }
      if (Date.now() > deadline) {
        throw new Error(
          `Timeout acquiring reservations lock after ${LOCK_TIMEOUT_MS}ms (held by another agent?)`
        );
      }
      sleepSync(LOCK_RETRY_MS);
    }
  }
  try {
    return fn();
  } finally {
    try {
      fs.closeSync(fd);
    } catch (_) {}
    try {
      fs.unlinkSync(LOCK_FILE);
    } catch (_) {}
  }
}

// ============================================================================

/**
 * Read reservations from JSON file.
 * Returns empty array if file doesn't exist (lazy initialization per D3).
 */
function readReservations() {
  try {
    if (!fs.existsSync(RESERVATIONS_FILE)) {
      return [];
    }
    const content = fs.readFileSync(RESERVATIONS_FILE, "utf8");
    const parsed = JSON.parse(content);
    
    // Validate structure
    if (!Array.isArray(parsed)) {
      console.error("Error: reservations.json must contain an array");
      console.error("File may be corrupted. Run 'sweep' to reset or manually fix the file.");
      process.exit(1);
    }
    
    // Validate each reservation entry
    const validReservations = [];
    for (let i = 0; i < parsed.length; i++) {
      const res = parsed[i];
      
      // Check required fields
      if (!res.agent || typeof res.agent !== 'string') {
        console.warn(`Warning: Skipping reservation ${i}: missing or invalid 'agent' field`);
        continue;
      }
      if (!res.bead || typeof res.bead !== 'string') {
        console.warn(`Warning: Skipping reservation ${i}: missing or invalid 'bead' field`);
        continue;
      }
      if (!Array.isArray(res.paths) || res.paths.length === 0) {
        console.warn(`Warning: Skipping reservation ${i}: missing or invalid 'paths' field`);
        continue;
      }
      if (!res.expires_at || typeof res.expires_at !== 'string') {
        console.warn(`Warning: Skipping reservation ${i}: missing or invalid 'expires_at' field`);
        continue;
      }
      
      // Validate ISO-8601 timestamp
      const expiryDate = new Date(res.expires_at);
      if (isNaN(expiryDate.getTime())) {
        console.warn(`Warning: Skipping reservation ${i}: invalid ISO-8601 timestamp '${res.expires_at}'`);
        continue;
      }
      
      validReservations.push(res);
    }
    
    // If we filtered out invalid entries, warn the user
    if (validReservations.length < parsed.length) {
      console.warn(`Filtered out ${parsed.length - validReservations.length} invalid reservation(s)`);
    }
    
    return validReservations;
  } catch (error) {
    if (error.name === 'SyntaxError') {
      console.error(`Error: reservations.json contains invalid JSON: ${error.message}`);
      console.error("File may be corrupted. Run 'sweep' to reset or manually fix the file.");
      process.exit(1);
    }
    console.error(`Error reading reservations: ${error.message}`);
    process.exit(1);
  }
}

/**
 * Write reservations to JSON file.
 * Creates .superpowers/ directory if needed.
 */
function writeReservations(reservations) {
  try {
    fs.mkdirSync(SCRIPT_DIR, { recursive: true });
    fs.writeFileSync(RESERVATIONS_FILE, JSON.stringify(reservations, null, 2), "utf8");
  } catch (error) {
    console.error(`Error writing reservations: ${error.message}`);
    process.exit(1);
  }
}

/**
 * Filter out expired reservations.
 */
function filterActive(reservations) {
  const now = new Date();
  return reservations.filter((r) => new Date(r.expires_at) > now);
}

/**
 * Check for conflicts between new paths and existing reservations.
 * Returns array of conflict objects.
 */
function checkConflicts(agent, newPaths, existingReservations) {
  const conflicts = [];
  const activeReservations = filterActive(existingReservations);

  // Don't conflict with own reservations
  const otherReservations = activeReservations.filter((r) => r.agent !== agent);

  for (const newPath of newPaths) {
    for (const reservation of otherReservations) {
      for (const existingPath of reservation.paths) {
        if (patternsConflict(newPath, existingPath)) {
          conflicts.push({
            path: newPath,
            conflictsWith: existingPath,
            holder: reservation.agent,
            bead: reservation.bead,
            expires_at: reservation.expires_at,
          });
        }
      }
    }
  }

  return conflicts;
}

// ============================================================================
// Commands
// ============================================================================

/**
 * Reserve file paths for an agent.
 */
function cmdReserve(args) {
  const { agent, bead, paths, ttl, json } = args;

  if (!agent || !bead || !paths) {
    console.error("Error: --agent, --bead, and --paths are required");
    process.exit(1);
  }

  // Validate inputs (paths shape, length/wildcard caps, traversal, regex compile)
  try {
    validatePatterns(paths);
  } catch (err) {
    console.error(`Error: ${err.message}`);
    process.exit(1);
  }

  const ttlSeconds = ttl ?? DEFAULT_TTL_SECONDS;
  try {
    validateTtl(ttlSeconds);
  } catch (err) {
    console.error(`Error: ${err.message}`);
    process.exit(1);
  }
  const expiresAt = new Date(Date.now() + ttlSeconds * 1000).toISOString();

  const result = withLock(() => {
    const reservations = readReservations();
    const conflicts = checkConflicts(agent, paths, reservations);
    if (conflicts.length > 0) return { conflicts };
    const newReservation = {
      agent,
      bead,
      paths,
      ttl: ttlSeconds,
      expires_at: expiresAt,
      reserved_at: new Date().toISOString(),
    };
    reservations.push(newReservation);
    writeReservations(reservations);
    return { newReservation };
  });

  const conflicts = result.conflicts || [];

  if (conflicts.length > 0) {
    if (json) {
      console.log(
        JSON.stringify(
          {
            success: false,
            conflicts: conflicts,
          },
          null,
          2
        )
      );
    } else {
      console.error(`Conflict detected for agent ${agent}:`);
      for (const conflict of conflicts) {
        console.error(
          `  ${conflict.path} conflicts with ${conflict.conflictsWith} (held by ${conflict.holder}, bead ${conflict.bead}, expires ${conflict.expires_at})`
        );
      }
    }
    process.exit(1);
  }

  if (result.newReservation) {
    const newReservation = result.newReservation;
    if (json) {
      console.log(JSON.stringify({ success: true, reservation: newReservation }, null, 2));
    } else {
      console.log(`Reserved ${paths.join(", ")} for ${agent} (bead ${bead}, expires ${expiresAt})`);
    }
  }
}

/**
 * Release reservations held by an agent.
 */
function cmdRelease(args) {
  const { agent, paths, json } = args;

  if (!agent) {
    console.error("Error: --agent is required");
    process.exit(1);
  }

  const releasedCount = withLock(() => {
    let reservations = readReservations();
    const initialCount = reservations.length;

    if (paths && paths.length > 0) {
      reservations = reservations.filter((r) => {
        if (r.agent !== agent) return true;
        return !r.paths.some((rPath) =>
          paths.some((relPath) => patternsConflict(rPath, relPath))
        );
      });
    } else {
      reservations = reservations.filter((r) => r.agent !== agent);
    }

    writeReservations(reservations);
    return initialCount - reservations.length;
  });

  if (json) {
    console.log(
      JSON.stringify(
        {
          success: true,
          released: releasedCount,
        },
        null,
        2
      )
    );
  } else {
    console.log(`Released ${releasedCount} reservation(s) for ${agent}`);
  }
}

/**
 * List active reservations.
 */
function cmdList(args) {
  const { agent, json } = args;

  let reservations = readReservations();
  reservations = filterActive(reservations);

  if (agent) {
    reservations = reservations.filter((r) => r.agent === agent);
  }

  if (json) {
    console.log(
      JSON.stringify(
        {
          count: reservations.length,
          reservations: reservations,
        },
        null,
        2
      )
    );
  } else {
    if (reservations.length === 0) {
      console.log("No active reservations");
    } else {
      console.log(`Active reservations (${reservations.length}):`);
      for (const r of reservations) {
        console.log(`  ${r.agent} (bead ${r.bead}): ${r.paths.join(", ")} [expires ${r.expires_at}]`);
      }
    }
  }
}

/**
 * Remove expired reservations.
 */
function cmdSweep(args) {
  const { json } = args;

  const { removedCount, remainingCount } = withLock(() => {
    let reservations = readReservations();
    const initialCount = reservations.length;
    reservations = filterActive(reservations);
    const removed = initialCount - reservations.length;
    writeReservations(reservations);
    return { removedCount: removed, remainingCount: reservations.length };
  });

  if (json) {
    console.log(
      JSON.stringify(
        {
          success: true,
          removed: removedCount,
          remaining: remainingCount,
        },
        null,
        2
      )
    );
  } else {
    console.log(`Removed ${removedCount} expired reservation(s), ${remainingCount} remaining`);
  }
}

// ============================================================================
// CLI Argument Parsing
// ============================================================================

function parseArgs(argv) {
  const args = {
    command: argv[0],
    agent: undefined,
    bead: undefined,
    paths: [],
    ttl: undefined,
    json: false,
  };

  for (let i = 1; i < argv.length; i++) {
    const arg = argv[i];

    if (arg === "--agent") {
      args.agent = argv[++i];
    } else if (arg.startsWith("--agent=")) {
      args.agent = arg.slice("--agent=".length);
    } else if (arg === "--bead") {
      args.bead = argv[++i];
    } else if (arg.startsWith("--bead=")) {
      args.bead = arg.slice("--bead=".length);
    } else if (arg === "--paths") {
      args.paths = argv[++i].split(",");
    } else if (arg.startsWith("--paths=")) {
      args.paths = arg.slice("--paths=".length).split(",");
    } else if (arg === "--ttl") {
      args.ttl = parseInt(argv[++i], 10);
    } else if (arg.startsWith("--ttl=")) {
      args.ttl = parseInt(arg.slice("--ttl=".length), 10);
    } else if (arg === "--json") {
      args.json = true;
    } else if (arg === "--help" || arg === "-h") {
      printHelp();
      process.exit(0);
    } else {
      console.error(`Unknown argument: ${arg}`);
      process.exit(1);
    }
  }

  return args;
}

function printHelp() {
  console.log(`
Superpowers File Reservations CLI

Commands:
  reserve  - Reserve file paths for exclusive access
  release  - Release reservations held by an agent
  list     - Show active reservations
  sweep    - Remove expired reservations

Usage:
  reservations.mjs reserve --agent <name> --bead <id> --paths <glob> [--ttl <seconds>] [--json]
  reservations.mjs release --agent <name> [--paths <glob>] [--json]
  reservations.mjs list [--agent <name>] [--json]
  reservations.mjs sweep [--json]

Examples:
  # Reserve files for a worker
  reservations.mjs reserve --agent BlueLake --bead bd-123 --paths "src/**/*.ts,tests/**" --ttl 3600

  # List all active reservations
  reservations.mjs list --json

  # Release all reservations for an agent
  reservations.mjs release --agent BlueLake

  # Remove expired reservations
  reservations.mjs sweep
`);
}

// ============================================================================
// Main
// ============================================================================

function main(argv = process.argv.slice(2)) {
  if (argv.length === 0) {
    printHelp();
    process.exit(1);
  }

  const command = argv[0];
  const args = parseArgs(argv);

  switch (command) {
    case "reserve":
      cmdReserve(args);
      break;
    case "release":
      cmdRelease(args);
      break;
    case "list":
      cmdList(args);
      break;
    case "sweep":
      cmdSweep(args);
      break;
    case "--help":
    case "-h":
      printHelp();
      break;
    default:
      console.error(`Unknown command: ${command}`);
      console.error('Run "reservations.mjs --help" for usage');
      process.exit(1);
  }
}

if (import.meta.url === `file://${process.argv[1]}`) {
  main();
}

export { checkConflicts, filterActive, globToRegex, matchesGlob, patternsConflict };
