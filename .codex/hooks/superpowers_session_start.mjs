#!/usr/bin/env node
/**
 * Codex SessionStart hook for superpowers onboarding.
 *
 * Installed by onboard_superpowers.mjs into .codex/hooks/superpowers_session_start.mjs
 *
 * Responsibilities:
 *   1. Check .superpowers/onboarding.json exists and version is current
 *   2. If current: read docs/learnings/critical-patterns.md and output for agent context
 *   3. If stale/missing: output warning so Codex agent sees it
 *
 * Protocol: Codex hooks receive no stdin. Stdout is captured as hook output.
 * Exit 0 = success.
 *
 * Decisions: D11, D12
 * Learnings: code-block-variable-binding, security-rule-inline-with-context
 */

import { readFileSync, existsSync } from 'fs';
import { join, resolve } from 'path';

// The expected onboarding version — bumped when the onboard script changes.
// This value is replaced by onboard_superpowers.mjs during installation.
const EXPECTED_VERSION = '1.1.0';

const repoRoot = process.cwd();
const onboardingPath = join(repoRoot, '.superpowers', 'onboarding.json');
const criticalPatternsPath = join(repoRoot, 'docs', 'learnings', 'critical-patterns.md');

/**
 * Read and parse onboarding.json, returning null if missing or invalid.
 */
const readOnboardingState = () => {
  if (!existsSync(onboardingPath)) {
    return null;
  }
  try {
    const raw = readFileSync(onboardingPath, 'utf8');
    const state = JSON.parse(raw);
    return state;
  } catch {
    return null;
  }
};

/**
 * Read critical-patterns.md if it exists, returning null otherwise.
 */
const readCriticalPatterns = () => {
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

const state = readOnboardingState();

if (!state) {
  process.stdout.write(
    '[superpowers] WARNING: Onboarding not found. ' +
    'Run the superpowers onboarding script before continuing:\n' +
    '  node <superpowers>/skills/using-superpowers/scripts/onboard_superpowers.mjs --apply --repo-root .\n\n' +
    'Without onboarding, superpowers skills and session-start hooks are not installed.\n'
  );
  process.exit(0);
}

if (state.version !== EXPECTED_VERSION) {
  process.stdout.write(
    `[superpowers] WARNING: Onboarding is stale (installed: ${state.version}, expected: ${EXPECTED_VERSION}). ` +
    'Run the superpowers onboarding script to update:\n' +
    '  node <superpowers>/skills/using-superpowers/scripts/onboard_superpowers.mjs --apply --repo-root .\n\n'
  );
  process.exit(0);
}

// Onboarding is current — inject critical patterns if available
const patterns = readCriticalPatterns();
if (patterns) {
  process.stdout.write(
    '# Critical Patterns (from docs/learnings/critical-patterns.md)\n\n' +
    patterns +
    '\n'
  );
}

process.exit(0);
