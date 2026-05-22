/**
 * Version command implementation
 * 
 * Displays the current version of hostie
 */

import { readFileSync } from "fs";
import { join, dirname } from "path";
import { fileURLToPath } from "url";
import { ExitCode } from "../exit-codes";

/**
 * Execute the version command
 * 
 * @returns Exit code (0 = success)
 */
export async function versionCommand(): Promise<number> {
  try {
    // Read version from package.json
    const __filename = fileURLToPath(import.meta.url);
    const __dirname = dirname(__filename);
    const packageJsonPath = join(__dirname, "../../../package.json");
    const packageJson = JSON.parse(readFileSync(packageJsonPath, "utf-8"));
    const version = packageJson.version;

    // Output: 'hostie v<version>'
    console.log(`hostie v${version}`);

    return ExitCode.SUCCESS;
  } catch (err: any) {
    console.error(`Error: ${err.message}`);
    return ExitCode.IO_ERROR;
  }
}
