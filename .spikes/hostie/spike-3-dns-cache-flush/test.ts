#!/usr/bin/env bun

/**
 * Spike 3: DNS Cache Flush Platform Detection
 * 
 * Tests platform-specific DNS cache flush commands:
 * - macOS: dscacheutil -flushcache && killall -HUP mDNSResponder
 * - Linux: systemd-resolved, nscd, dnsmasq (varies by distro)
 */

import { spawnSync } from "bun";
import { existsSync } from "fs";

type Platform = "darwin" | "linux" | "unknown";

function detectPlatform(): Platform {
  const platform = process.platform;
  if (platform === "darwin") return "darwin";
  if (platform === "linux") return "linux";
  return "unknown";
}

function flushDNSCacheDarwin(): { success: boolean; commands: string[]; error?: string } {
  const commands: string[] = [];
  
  // macOS DNS cache flush
  const cmd1 = ["sudo", "dscacheutil", "-flushcache"];
  const cmd2 = ["sudo", "killall", "-HUP", "mDNSResponder"];
  
  commands.push(cmd1.join(" "));
  commands.push(cmd2.join(" "));
  
  console.log(`Running: ${cmd1.join(" ")}`);
  const result1 = spawnSync({
    cmd: cmd1,
    stdout: "pipe",
    stderr: "pipe",
  });
  
  if (result1.exitCode !== 0) {
    return {
      success: false,
      commands,
      error: `dscacheutil failed: ${result1.stderr?.toString()}`,
    };
  }
  
  console.log(`Running: ${cmd2.join(" ")}`);
  const result2 = spawnSync({
    cmd: cmd2,
    stdout: "pipe",
    stderr: "pipe",
  });
  
  if (result2.exitCode !== 0) {
    return {
      success: false,
      commands,
      error: `killall failed: ${result2.stderr?.toString()}`,
    };
  }
  
  return { success: true, commands };
}

function flushDNSCacheLinux(): { success: boolean; commands: string[]; error?: string } {
  const commands: string[] = [];
  
  // Check for systemd-resolved
  if (existsSync("/run/systemd/resolve/stub-resolv.conf")) {
    const cmd = ["sudo", "systemctl", "restart", "systemd-resolved"];
    commands.push(cmd.join(" "));
    
    console.log(`Running: ${cmd.join(" ")}`);
    const result = spawnSync({
      cmd,
      stdout: "pipe",
      stderr: "pipe",
    });
    
    if (result.exitCode === 0) {
      return { success: true, commands };
    }
  }
  
  // Check for nscd
  const nscdCheck = spawnSync({
    cmd: ["which", "nscd"],
    stdout: "pipe",
  });
  
  if (nscdCheck.exitCode === 0) {
    const cmd = ["sudo", "systemctl", "restart", "nscd"];
    commands.push(cmd.join(" "));
    
    console.log(`Running: ${cmd.join(" ")}`);
    const result = spawnSync({
      cmd,
      stdout: "pipe",
      stderr: "pipe",
    });
    
    if (result.exitCode === 0) {
      return { success: true, commands };
    }
  }
  
  // Check for dnsmasq
  const dnsmasqCheck = spawnSync({
    cmd: ["which", "dnsmasq"],
    stdout: "pipe",
  });
  
  if (dnsmasqCheck.exitCode === 0) {
    const cmd = ["sudo", "systemctl", "restart", "dnsmasq"];
    commands.push(cmd.join(" "));
    
    console.log(`Running: ${cmd.join(" ")}`);
    const result = spawnSync({
      cmd,
      stdout: "pipe",
      stderr: "pipe",
    });
    
    if (result.exitCode === 0) {
      return { success: true, commands };
    }
  }
  
  return {
    success: false,
    commands,
    error: "No known DNS cache service found (systemd-resolved, nscd, dnsmasq)",
  };
}

// Main test
console.log("Testing DNS cache flush platform detection...\n");

const platform = detectPlatform();
console.log(`Detected platform: ${platform}\n`);

if (platform === "unknown") {
  console.log("✗ Unsupported platform");
  process.exit(1);
}

let result: { success: boolean; commands: string[]; error?: string };

if (platform === "darwin") {
  result = flushDNSCacheDarwin();
} else {
  result = flushDNSCacheLinux();
}

console.log("\n--- Results ---");
console.log(`Commands attempted: ${result.commands.length}`);
result.commands.forEach((cmd) => console.log(`  - ${cmd}`));

if (result.success) {
  console.log("\n✓ DNS cache flush successful");
  process.exit(0);
} else {
  console.log(`\n✗ DNS cache flush failed: ${result.error}`);
  console.log("\nNote: This may require sudo access or the service may not be running");
  process.exit(1);
}
