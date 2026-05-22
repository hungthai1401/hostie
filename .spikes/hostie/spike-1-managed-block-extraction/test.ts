#!/usr/bin/env bun

/**
 * Spike 1: Managed Block Extraction with Malformed Input
 * 
 * Tests the managed block extraction logic with various edge cases:
 * - Missing BEGIN marker
 * - Missing END marker
 * - Multiple blocks
 * - Malformed markers
 * - Empty blocks
 */

const BEGIN_MARKER = "# BEGIN HOSTIE";
const END_MARKER = "# END HOSTIE";

function extractManagedBlock(content: string): {
  before: string;
  block: string | null;
  after: string;
  hasBlock: boolean;
} {
  const lines = content.split("\n");
  let beginIdx = -1;
  let endIdx = -1;

  // Find first BEGIN marker
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].trim() === BEGIN_MARKER) {
      beginIdx = i;
      break;
    }
  }

  // Find first END marker after BEGIN
  if (beginIdx !== -1) {
    for (let i = beginIdx + 1; i < lines.length; i++) {
      if (lines[i].trim() === END_MARKER) {
        endIdx = i;
        break;
      }
    }
  }

  // No block found
  if (beginIdx === -1 || endIdx === -1) {
    return {
      before: content,
      block: null,
      after: "",
      hasBlock: false,
    };
  }

  // Extract parts
  const before = lines.slice(0, beginIdx).join("\n");
  const block = lines.slice(beginIdx + 1, endIdx).join("\n");
  const after = lines.slice(endIdx + 1).join("\n");

  return {
    before: before ? before + "\n" : "",
    block,
    after: after ? "\n" + after : "",
    hasBlock: true,
  };
}

function replaceManagedBlock(content: string, newBlock: string): string {
  const extracted = extractManagedBlock(content);

  if (!extracted.hasBlock) {
    // First apply: append block
    const trimmed = content.trimEnd();
    return `${trimmed}\n\n${BEGIN_MARKER}\n${newBlock}\n${END_MARKER}\n`;
  }

  // Replace existing block
  return `${extracted.before}${BEGIN_MARKER}\n${newBlock}\n${END_MARKER}${extracted.after}`;
}

// Test cases
const tests = [
  {
    name: "No existing block",
    input: "127.0.0.1 localhost\n::1 localhost\n",
    newBlock: "192.168.1.10 myserver",
    expected: "127.0.0.1 localhost\n::1 localhost\n\n# BEGIN HOSTIE\n192.168.1.10 myserver\n# END HOSTIE\n",
  },
  {
    name: "Existing block",
    input: "127.0.0.1 localhost\n# BEGIN HOSTIE\n192.168.1.10 oldserver\n# END HOSTIE\n::1 localhost\n",
    newBlock: "192.168.1.10 newserver",
    expected: "127.0.0.1 localhost\n# BEGIN HOSTIE\n192.168.1.10 newserver\n# END HOSTIE\n::1 localhost\n",
  },
  {
    name: "Missing END marker",
    input: "127.0.0.1 localhost\n# BEGIN HOSTIE\n192.168.1.10 myserver\n::1 localhost\n",
    newBlock: "192.168.1.10 newserver",
    expected: "127.0.0.1 localhost\n# BEGIN HOSTIE\n192.168.1.10 myserver\n::1 localhost\n\n# BEGIN HOSTIE\n192.168.1.10 newserver\n# END HOSTIE\n",
  },
  {
    name: "Missing BEGIN marker",
    input: "127.0.0.1 localhost\n192.168.1.10 myserver\n# END HOSTIE\n::1 localhost\n",
    newBlock: "192.168.1.10 newserver",
    expected: "127.0.0.1 localhost\n192.168.1.10 myserver\n# END HOSTIE\n::1 localhost\n\n# BEGIN HOSTIE\n192.168.1.10 newserver\n# END HOSTIE\n",
  },
  {
    name: "Empty block",
    input: "127.0.0.1 localhost\n# BEGIN HOSTIE\n# END HOSTIE\n::1 localhost\n",
    newBlock: "192.168.1.10 newserver",
    expected: "127.0.0.1 localhost\n# BEGIN HOSTIE\n192.168.1.10 newserver\n# END HOSTIE\n::1 localhost\n",
  },
  {
    name: "Multiple blocks (only first is managed)",
    input: "127.0.0.1 localhost\n# BEGIN HOSTIE\n192.168.1.10 first\n# END HOSTIE\n# BEGIN HOSTIE\n192.168.1.11 second\n# END HOSTIE\n",
    newBlock: "192.168.1.10 replaced",
    expected: "127.0.0.1 localhost\n# BEGIN HOSTIE\n192.168.1.10 replaced\n# END HOSTIE\n# BEGIN HOSTIE\n192.168.1.11 second\n# END HOSTIE\n",
  },
  {
    name: "Markers with extra whitespace",
    input: "127.0.0.1 localhost\n  # BEGIN HOSTIE  \n192.168.1.10 myserver\n  # END HOSTIE  \n::1 localhost\n",
    newBlock: "192.168.1.10 newserver",
    expected: "127.0.0.1 localhost\n  # BEGIN HOSTIE  \n192.168.1.10 newserver\n  # END HOSTIE  \n::1 localhost\n",
  },
];

console.log("Running managed block extraction spike...\n");

let passed = 0;
let failed = 0;

for (const test of tests) {
  const result = replaceManagedBlock(test.input, test.newBlock);
  const success = result === test.expected;

  if (success) {
    console.log(`✓ ${test.name}`);
    passed++;
  } else {
    console.log(`✗ ${test.name}`);
    console.log(`  Expected:\n${JSON.stringify(test.expected)}`);
    console.log(`  Got:\n${JSON.stringify(result)}`);
    failed++;
  }
}

console.log(`\n${passed} passed, ${failed} failed`);

process.exit(failed > 0 ? 1 : 0);
