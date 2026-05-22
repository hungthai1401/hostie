#!/usr/bin/env bun

/**
 * Spike 4: Bun Compile Multi-Platform Builds
 * 
 * Tests whether Bun can cross-compile for multiple platforms:
 * - darwin-arm64 (Apple Silicon)
 * - darwin-x64 (Intel Mac)
 * - linux-x64 (x86_64 Linux)
 * - linux-arm64 (ARM64 Linux)
 */

import { spawnSync } from "bun";
import { existsSync, mkdirSync, statSync } from "fs";

const targets = [
  "bun-darwin-arm64",
  "bun-darwin-x64",
  "bun-linux-x64",
  "bun-linux-arm64",
];

// Create a minimal test script
const testScript = `
console.log("Hello from compiled binary!");
console.log("Platform:", process.platform);
console.log("Arch:", process.arch);
`;

const testScriptPath = "/tmp/hostie-spike-test.ts";
const outputDir = "/tmp/hostie-spike-builds";

// Write test script
await Bun.write(testScriptPath, testScript);

// Create output directory
if (!existsSync(outputDir)) {
  mkdirSync(outputDir, { recursive: true });
}

console.log("Testing Bun multi-platform compilation...\n");
console.log(`Test script: ${testScriptPath}`);
console.log(`Output directory: ${outputDir}\n`);

const results: Array<{
  target: string;
  success: boolean;
  outputPath?: string;
  size?: number;
  error?: string;
}> = [];

for (const target of targets) {
  const outputPath = `${outputDir}/hostie-${target}`;
  
  console.log(`Building for ${target}...`);
  
  const result = spawnSync({
    cmd: [
      "bun",
      "build",
      testScriptPath,
      "--compile",
      "--target",
      target,
      "--outfile",
      outputPath,
    ],
    stdout: "pipe",
    stderr: "pipe",
  });
  
  if (result.exitCode === 0 && existsSync(outputPath)) {
    const stats = statSync(outputPath);
    const sizeMB = (stats.size / 1024 / 1024).toFixed(2);
    
    console.log(`  ✓ Success (${sizeMB} MB)\n`);
    
    results.push({
      target,
      success: true,
      outputPath,
      size: stats.size,
    });
  } else {
    const error = result.stderr?.toString() || "Unknown error";
    console.log(`  ✗ Failed: ${error}\n`);
    
    results.push({
      target,
      success: false,
      error,
    });
  }
}

// Summary
console.log("--- Summary ---");
const successful = results.filter((r) => r.success);
const failed = results.filter((r) => !r.success);

console.log(`Successful builds: ${successful.length}/${targets.length}`);
successful.forEach((r) => {
  const sizeMB = r.size ? (r.size / 1024 / 1024).toFixed(2) : "?";
  console.log(`  ✓ ${r.target} (${sizeMB} MB)`);
});

if (failed.length > 0) {
  console.log(`\nFailed builds: ${failed.length}`);
  failed.forEach((r) => {
    console.log(`  ✗ ${r.target}: ${r.error}`);
  });
}

// Check if we can cross-compile
const currentPlatform = process.platform;
const currentArch = process.arch;
const currentTarget = `bun-${currentPlatform}-${currentArch}`;

console.log(`\nCurrent platform: ${currentPlatform}-${currentArch}`);

const crossCompiled = successful.filter(
  (r) => r.target !== currentTarget
);

if (crossCompiled.length > 0) {
  console.log("✓ Cross-compilation works!");
  console.log(`  Built ${crossCompiled.length} non-native targets from ${currentTarget}`);
} else if (successful.length === 1 && successful[0].target === currentTarget) {
  console.log("⚠️  Cross-compilation not supported");
  console.log("  Only native target can be built");
  console.log("  CI pipeline needed for multi-platform builds");
} else {
  console.log("⚠️  Unexpected results");
}

process.exit(failed.length > 0 ? 1 : 0);
