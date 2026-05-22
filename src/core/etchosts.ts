/**
 * /etc/hosts file operations for hostie
 * 
 * Handles reading and writing /etc/hosts with proper error handling
 */

import { readFileSync } from "fs";

const ETC_HOSTS_PATH = "/etc/hosts";

/**
 * Read /etc/hosts file content
 * 
 * @returns Full content of /etc/hosts as a string, or empty string if file doesn't exist
 */
export async function readEtcHosts(): Promise<string> {
  try {
    const content = readFileSync(ETC_HOSTS_PATH, "utf-8");
    return content;
  } catch (err: any) {
    // Handle ENOENT (file doesn't exist) gracefully
    if (err.code === "ENOENT") {
      return "";
    }
    // Re-throw other errors (like EACCES)
    throw err;
  }
}
