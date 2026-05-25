#!/usr/bin/env bun
/**
 * Build script for Hostie
 *
 * Compiles src/index.ts into a single executable binary using Bun's
 * `bun build --compile` feature. Outputs to dist/hostie for the current
 * platform, or to dist/hostie-<target> when an explicit target is supplied.
 *
 * Usage:
 *   bun run build.ts                          # current platform
 *   bun run build.ts --target=bun-darwin-arm64
 *   bun run build.ts --target=bun-darwin-x64
 *   bun run build.ts --target=bun-linux-x64
 *   bun run build.ts --target=bun-linux-arm64
 */

import { existsSync, mkdirSync, statSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";

interface BuildOptions {
  entrypoint: string;
  outfile: string;
  target?: string;
  minify: boolean;
}

const ROOT = resolve(import.meta.dir);
const DIST = resolve(ROOT, "dist");

/**
 * Path to an auto-generated no-op shim for `react-devtools-core`.
 *
 * ink optionally imports `react-devtools-core` when `process.env.DEV === "true"`.
 * The package is not in our dependency tree, so Bun's bundler fails to resolve
 * it even though the branch is unreachable in production. We point a `tsconfig`
 * path alias at this shim so resolution succeeds; the code path is never
 * executed at runtime because `process.env.DEV` is not "true".
 */
const SHIM_PATH = resolve(ROOT, "node_modules", "react-devtools-core", "index.js");
const SHIM_PKG = resolve(ROOT, "node_modules", "react-devtools-core", "package.json");

function ensureDist(): void {
  if (!existsSync(DIST)) {
    mkdirSync(DIST, { recursive: true });
  }
}

function parseTarget(argv: string[]): string | undefined {
  for (const arg of argv) {
    if (arg.startsWith("--target=")) {
      return arg.slice("--target=".length);
    }
  }
  return undefined;
}

function formatBytes(bytes: number): string {
  return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
}

/**
 * Install a no-op `react-devtools-core` package under node_modules so the
 * bundler can resolve ink's optional dev-only import. The stub is intentionally
 * never invoked at runtime — ink guards the import with
 * `process.env.DEV === "true"`.
 */
function writeDevtoolsShim(): void {
  const dir = resolve(ROOT, "node_modules", "react-devtools-core");
  if (!existsSync(dir)) {
    mkdirSync(dir, { recursive: true });
  }
  if (!existsSync(SHIM_PKG)) {
    writeFileSync(
      SHIM_PKG,
      JSON.stringify(
        {
          name: "react-devtools-core",
          version: "0.0.0-hostie-shim",
          main: "index.js",
          type: "commonjs",
        },
        null,
        2,
      ),
    );
  }
  if (!existsSync(SHIM_PATH)) {
    writeFileSync(
      SHIM_PATH,
      "// Auto-generated no-op shim for react-devtools-core (see build.ts).\n" +
        "// ink only imports this when process.env.DEV === 'true'.\n" +
        'const noop = () => {};\n' +
        'module.exports = { connectToDevTools: noop };\n' +
        'module.exports.default = module.exports;\n',
    );
  }
}

/**
 * Build a single binary for the requested target (or the current platform).
 */
export async function build(opts: BuildOptions): Promise<void> {
  ensureDist();
  writeDevtoolsShim();

  const args = [
    "build",
    opts.entrypoint,
    "--compile",
    "--outfile",
    opts.outfile,
  ];
  if (opts.minify) args.push("--minify");
  if (opts.target) args.push(`--target=${opts.target}`);

  console.log(`> bun ${args.join(" ")}`);
  const proc = Bun.spawnSync(["bun", ...args], {
    cwd: ROOT,
    stdout: "inherit",
    stderr: "inherit",
  });

  if (proc.exitCode !== 0) {
    throw new Error(`Build failed with exit code ${proc.exitCode}`);
  }

  if (existsSync(opts.outfile)) {
    const size = statSync(opts.outfile).size;
    console.log(`Built ${opts.outfile} (${formatBytes(size)})`);
  }
}

if (import.meta.main) {
  const target = parseTarget(process.argv.slice(2));
  const suffix = target ? `-${target}` : "";
  const outfile = resolve(DIST, `hostie${suffix}`);

  await build({
    entrypoint: resolve(ROOT, "src/index.ts"),
    outfile,
    target,
    minify: true,
  });
}
