/**
 * Apply command implementation
 * 
 * Reads ~/.hosts and applies changes to /etc/hosts
 * Supports --dry-run flag for preview without writing
 */

import { readHostsFile } from "../../core/file-io";
import { applyHostsFile } from "../../core/apply";
import { renderHostsFile } from "../../core/render";

export type ApplyOptions = {
  dryRun?: boolean;
};

/**
 * Execute the apply command
 * 
 * @param options - Command options
 * @returns Exit code (0 = success, 2 = I/O error, 3 = permission error)
 */
export async function applyCommand(options: ApplyOptions): Promise<number> {
  try {
    // Read ~/.hosts
    const hostsFile = await readHostsFile("~/.hosts");

    // Dry-run mode: show preview without writing
    if (options.dryRun) {
      console.log("Dry-run mode: showing preview without writing\n");
      
      const preview = renderHostsFile(hostsFile);
      
      if (preview.trim() === "") {
        console.log("No entries to apply (empty managed block)");
      } else {
        console.log("Managed block that would be written to /etc/hosts:");
        console.log("─".repeat(60));
        console.log(preview);
        console.log("─".repeat(60));
      }
      
      return 0;
    }

    // Apply changes to /etc/hosts
    const result = await applyHostsFile(hostsFile);

    if (result.changed) {
      console.log("✓ " + result.message);
    } else {
      console.log("○ " + result.message);
    }

    return 0;

  } catch (err: any) {
    // Handle permission errors
    if (err.code === "EACCES") {
      console.error("Error: Permission denied");
      console.error("");
      console.error("Cannot write to /etc/hosts without elevated privileges.");
      console.error("Please run with sudo:");
      console.error("  sudo hostie apply");
      console.error("");
      console.error("In CI/scripts: Configure passwordless sudo or run 'sudo -v' before hostie");
      return 3;
    }

    // Handle I/O errors
    if (err.code === "ENOENT") {
      console.error(`Error: File not found - ${err.message}`);
      return 2;
    }

    // Handle other I/O errors
    console.error(`Error: ${err.message}`);
    return 2;
  }
}
