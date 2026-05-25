/**
 * Version command implementation
 *
 * Displays the current version of hostie.
 *
 * Reads the version from the bundled `package.json` via a static
 * `import` so the value is embedded into the compiled binary. The
 * previous filesystem-based lookup broke under `bun build --compile`
 * because `package.json` is not present next to the runtime binary.
 */

import { ExitCode } from "../exit-codes";
// eslint-disable-next-line import/no-relative-parent-imports
import pkg from "../../../package.json" with { type: "json" };

/**
 * Execute the version command
 *
 * @returns Exit code (0 = success)
 */
export async function versionCommand(): Promise<number> {
  try {
    const version = (pkg as { version: string }).version;
    console.log(`hostie v${version}`);
    return ExitCode.SUCCESS;
  } catch (err: any) {
    console.error(`Error: ${err.message}`);
    return ExitCode.IO_ERROR;
  }
}
