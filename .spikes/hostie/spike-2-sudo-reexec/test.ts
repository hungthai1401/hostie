#!/usr/bin/env bun

/**
 * Spike 2: Sudo Re-exec in Non-Interactive Contexts
 * 
 * Tests whether sudo re-exec works in:
 * - Interactive terminal (normal case)
 * - Non-interactive context (CI, cron, scripts)
 * - When sudo requires password
 * - When sudo is passwordless
 */

import { spawnSync } from "bun";
import { writeFileSync, unlinkSync } from "fs";

const testFile = "/tmp/hostie-spike-sudo-test.txt";

function testSudoWrite(): { success: boolean; error?: string } {
  try {
    // Try to write to a sudo-protected location
    writeFileSync(testFile, "test content");
    unlinkSync(testFile);
    return { success: true };
  } catch (err: any) {
    if (err.code === "EACCES") {
      // Need sudo - try re-exec
      console.log("EACCES detected, attempting sudo re-exec...");
      
      const result = spawnSync({
        cmd: ["sudo", "bun", import.meta.path, "--sudo-mode"],
        stdout: "inherit",
        stderr: "inherit",
        stdin: "inherit",
      });

      if (result.exitCode === 0) {
        return { success: true };
      } else {
        return { 
          success: false, 
          error: `Sudo re-exec failed with exit code ${result.exitCode}` 
        };
      }
    }
    
    return { success: false, error: err.message };
  }
}

function testSudoWriteWithSudo(): { success: boolean; error?: string } {
  try {
    writeFileSync(testFile, "test content from sudo");
    unlinkSync(testFile);
    return { success: true };
  } catch (err: any) {
    return { success: false, error: err.message };
  }
}

// Check if we're in sudo mode
if (process.argv.includes("--sudo-mode")) {
  console.log("Running in sudo mode...");
  const result = testSudoWriteWithSudo();
  if (result.success) {
    console.log("✓ Sudo write successful");
    process.exit(0);
  } else {
    console.log(`✗ Sudo write failed: ${result.error}`);
    process.exit(1);
  }
}

// Main test
console.log("Testing sudo re-exec...\n");

// Test 1: Check if we're already root
const isRoot = process.getuid?.() === 0;
console.log(`Current UID: ${process.getuid?.() ?? "unknown"}`);
console.log(`Running as root: ${isRoot}\n`);

if (isRoot) {
  console.log("⚠️  Already running as root, cannot test sudo re-exec");
  console.log("Run this spike as a normal user to test sudo re-exec");
  process.exit(0);
}

// Test 2: Check if sudo is available
const sudoCheck = spawnSync({
  cmd: ["which", "sudo"],
  stdout: "pipe",
});

if (sudoCheck.exitCode !== 0) {
  console.log("✗ sudo not found in PATH");
  process.exit(1);
}

console.log("✓ sudo is available\n");

// Test 3: Check sudo access (will prompt if needed)
console.log("Checking sudo access (may prompt for password)...");
const sudoTest = spawnSync({
  cmd: ["sudo", "-v"],
  stdout: "inherit",
  stderr: "inherit",
  stdin: "inherit",
});

if (sudoTest.exitCode !== 0) {
  console.log("✗ sudo access denied or password incorrect");
  process.exit(1);
}

console.log("✓ sudo access confirmed\n");

// Test 4: Attempt write with re-exec
console.log("Testing write with sudo re-exec...");
const result = testSudoWrite();

if (result.success) {
  console.log("✓ Sudo re-exec successful");
  console.log("\n✓ All tests passed");
  process.exit(0);
} else {
  console.log(`✗ Sudo re-exec failed: ${result.error}`);
  process.exit(1);
}
