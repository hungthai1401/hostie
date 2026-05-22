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

/**
 * Extract managed block from /etc/hosts content
 * 
 * Finds content between '# BEGIN HOSTIE' and '# END HOSTIE' markers.
 * Returns content before, inside, and after the managed block.
 * 
 * @param content - Full /etc/hosts file content
 * @returns Object with before, managed, and after sections
 */
export function extractManagedBlock(content: string): {
  before: string;
  managed: string;
  after: string;
} {
  const beginMarker = "# BEGIN HOSTIE";
  const endMarker = "# END HOSTIE";

  const beginIndex = content.indexOf(beginMarker);
  const endIndex = content.indexOf(endMarker);

  // If markers not found or malformed (only one marker), return original content
  if (beginIndex === -1 || endIndex === -1 || endIndex <= beginIndex) {
    return {
      before: content,
      managed: "",
      after: "",
    };
  }

  // Extract the three sections
  const before = content.substring(0, beginIndex);
  const managed = content.substring(beginIndex, endIndex + endMarker.length);
  const after = content.substring(endIndex + endMarker.length);

  return {
    before,
    managed,
    after,
  };
}
