#!/usr/bin/env node
/**
 * Claude Code SessionStart hook for superpowers onboarding.
 *
 * Installed by onboard_superpowers.mjs into .claude/hooks/superpowers_session_start.js
 *
 * Protocol (from spike cc-hooks-protocol):
 *   - Receives JSON on stdin: { session_id, transcript_path, cwd, hook_event_name, source, model }
 *   - source: "startup" | "resume" | "compact"
 *   - Exit 0: stdout content is injected into agent context
 *   - Exit 2: non-blocking for SessionStart (stderr shown to user only)
 *   - Always exit 0 so Claude sees our messages in context
 *
 * Responsibilities:
 *   1. Check .superpowers/onboarding.json exists and version is current
 *   2. If current: read docs/learnings/critical-patterns.md and output for agent context
 *   3. If stale/missing: output warning so Claude sees it
 *   4. On source=compact: inject post-compaction recovery instructions
 *
 * Decisions: D13, D14
 * Learnings: code-block-variable-binding, security-rule-inline-with-context
 */

'use strict';

const { readFileSync, existsSync } = require('fs');
const { join } = require('path');

// The expected onboarding version — replaced by onboard_superpowers.mjs during installation.
const EXPECTED_VERSION = '1.1.0';

/**
 * Read all stdin as a string, then parse as JSON.
 */
const readStdin = () =>
  new Promise((resolve, reject) => {
    const chunks = [];
    process.stdin.setEncoding('utf8');
    process.stdin.on('data', (chunk) => chunks.push(chunk));
    process.stdin.on('end', () => {
      try {
        resolve(JSON.parse(chunks.join('')));
      } catch (err) {
        reject(err);
      }
    });
    process.stdin.on('error', reject);
  });

/**
 * Read and parse onboarding.json from the given repo root.
 */
const readOnboardingState = (repoRoot) => {
  const onboardingPath = join(repoRoot, '.superpowers', 'onboarding.json');
  if (!existsSync(onboardingPath)) {
    return null;
  }
  try {
    const raw = readFileSync(onboardingPath, 'utf8');
    return JSON.parse(raw);
  } catch {
    return null;
  }
};

/**
 * Read critical-patterns.md if it exists.
 */
const readCriticalPatterns = (repoRoot) => {
  const criticalPatternsPath = join(repoRoot, 'docs', 'learnings', 'critical-patterns.md');
  if (!existsSync(criticalPatternsPath)) {
    return null;
  }
  try {
    const content = readFileSync(criticalPatternsPath, 'utf8');
    return content.trim() || null;
  } catch {
    return null;
  }
};

// --- Main ---

readStdin()
  .then((input) => {
    const repoRoot = input.cwd || process.cwd();
    const source = input.source || 'startup';
    const output = [];

    const state = readOnboardingState(repoRoot);

    if (!state) {
      output.push(
        '[superpowers] WARNING: Onboarding not found. ' +
        'Run the superpowers onboarding script before continuing:\n' +
        '  node <superpowers>/skills/using-superpowers/scripts/onboard_superpowers.mjs --apply --repo-root .\n\n' +
        'Without onboarding, superpowers skills and session-start hooks are not installed.'
      );
      process.stdout.write(output.join('\n'));
      process.exit(0);
    }

    if (state.version !== EXPECTED_VERSION) {
      output.push(
        `[superpowers] WARNING: Onboarding is stale (installed: ${state.version}, expected: ${EXPECTED_VERSION}). ` +
        'Run the superpowers onboarding script to update:\n' +
        '  node <superpowers>/skills/using-superpowers/scripts/onboard_superpowers.mjs --apply --repo-root .\n'
      );
      process.stdout.write(output.join('\n'));
      process.exit(0);
    }

    // Onboarding is current

    // On post-compaction, inject recovery instructions
    if (source === 'compact') {
      output.push(
        '# Post-Compaction Recovery (superpowers)\n\n' +
        'Context was compacted. STOP and re-read these files before continuing:\n' +
        '1. `.superpowers/STATE.md` — current chain position and active work\n' +
        '2. `docs/<feature>/design.md` — locked decisions for the current feature\n' +
        '3. Current bead specification (run `br show <bead-id>`)\n' +
        '4. Check active file reservations (run `node .superpowers/superpowers_reservations.mjs list`)\n\n' +
        'Do NOT continue implementing until all four are re-read.'
      );
    }

    // Inject critical patterns if available
    const patterns = readCriticalPatterns(repoRoot);
    if (patterns) {
      output.push(
        '# Critical Patterns (from docs/learnings/critical-patterns.md)\n\n' +
        patterns
      );
    }

    if (output.length > 0) {
      process.stdout.write(output.join('\n\n') + '\n');
    }

    process.exit(0);
  })
  .catch(() => {
    // If stdin parsing fails, still exit 0 — don't block the agent
    process.stdout.write(
      '[superpowers] WARNING: Failed to parse hook input. Onboarding check skipped.\n'
    );
    process.exit(0);
  });
