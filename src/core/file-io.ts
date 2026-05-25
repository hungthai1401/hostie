/**
 * File I/O for hostie
 * 
 * Handles reading and writing ~/.hosts in YAML format
 */

import { homedir } from "os";
import { dirname } from "path";
import { existsSync, mkdirSync, chmodSync } from "fs";
import type { HostsFile } from "../domain/types";

/**
 * Expand ~ to home directory
 */
function expandTilde(filepath: string): string {
  if (filepath.startsWith("~/")) {
    return filepath.replace("~", homedir());
  }
  return filepath;
}

/**
 * Default HostsFile structure when creating a new file
 */
const DEFAULT_HOSTS_FILE: HostsFile = {
  version: 1,
  groups: [],
};

/**
 * Read ~/.hosts file, creating it with defaults if it doesn't exist
 * 
 * @param filepath - Path to hosts file (default: ~/.hosts), supports ~ expansion
 * @returns Parsed HostsFile structure
 */
export async function readHostsFile(filepath: string = "~/.hosts"): Promise<HostsFile> {
  const expandedPath = expandTilde(filepath);
  
  // Check if file exists
  if (!existsSync(expandedPath)) {
    // Create default file
    const fresh = structuredClone(DEFAULT_HOSTS_FILE);
    await writeHostsFile(expandedPath, fresh);
    return fresh;
  }
  
  // Read and parse YAML
  const file = Bun.file(expandedPath);
  const content = await file.text();
  
  // Parse YAML using Bun's built-in YAML support
  const parsed = await import("yaml").then(yaml => yaml.parse(content));
  
  return parsed as HostsFile;
}

/**
 * Write HostsFile to disk in YAML format
 * 
 * @param filepath - Path to hosts file (default: ~/.hosts), supports ~ expansion
 * @param data - HostsFile structure to write
 */
export async function writeHostsFile(
  filepath: string = "~/.hosts",
  data: HostsFile
): Promise<void> {
  const expandedPath = expandTilde(filepath);
  
  // Ensure parent directory exists
  const dir = dirname(expandedPath);
  if (!existsSync(dir)) {
    mkdirSync(dir, { recursive: true });
  }
  
  // Serialize to YAML
  const yaml = await import("yaml");
  const content = yaml.stringify(data);
  
  // Write file
  await Bun.write(expandedPath, content);
  
  // Set file permissions to 644 (rw-r--r--)
  chmodSync(expandedPath, 0o644);
}
