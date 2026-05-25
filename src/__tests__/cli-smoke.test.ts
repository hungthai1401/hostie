/**
 * CLI integration smoke tests
 *
 * Builds the binary (via `bun run build`) and invokes it for one happy
 * path per subcommand to confirm src/index.ts dispatches to the handler
 * modules (hosts-cli-379.62).
 *
 * Per D14, validation errors must exit non-zero — we also verify that
 * an unknown command produces a non-zero exit.
 */

import { describe, test, expect, beforeAll, afterAll } from "bun:test";
import { spawnSync } from "child_process";
import { existsSync, mkdtempSync, rmSync, writeFileSync } from "fs";
import { tmpdir } from "os";
import { join, resolve } from "path";

const REPO_ROOT = resolve(import.meta.dir, "..", "..");
const BINARY = join(REPO_ROOT, "dist", "hostie");

/**
 * Run the compiled hostie binary with the given args and an isolated
 * HOME so the binary picks up our fixture `~/.hosts` instead of the
 * developer's real file.
 */
function runHostie(
  args: string[],
  home: string
): { code: number; stdout: string; stderr: string } {
  const result = spawnSync(BINARY, args, {
    cwd: home,
    env: {
      ...process.env,
      HOME: home,
    },
    encoding: "utf-8",
  });
  return {
    code: result.status ?? -1,
    stdout: result.stdout ?? "",
    stderr: result.stderr ?? "",
  };
}

let TMP_HOME: string;

beforeAll(() => {
  // Build the binary once for all smoke tests.
  if (!existsSync(BINARY)) {
    const build = spawnSync("bun", ["run", "build"], {
      cwd: REPO_ROOT,
      stdio: "inherit",
    });
    if (build.status !== 0) {
      throw new Error("Failed to build hostie binary for smoke tests");
    }
  }

  TMP_HOME = mkdtempSync(join(tmpdir(), "hostie-cli-smoke-"));

  // Seed a minimal ~/.hosts fixture used by list/apply/rm/etc.
  writeFileSync(
    join(TMP_HOME, ".hosts"),
    [
      "version: 1",
      "groups:",
      "  - name: work",
      "    entries:",
      "      - id: 01ARZ3NDEKTSV4RRFFQ69G5FAV",
      "        ip: 192.168.1.100",
      "        hostname: api.work",
      "        aliases: []",
      "        enabled: true",
      "    groups: []",
      "",
    ].join("\n")
  );
});

afterAll(() => {
  if (TMP_HOME && existsSync(TMP_HOME)) {
    rmSync(TMP_HOME, { recursive: true, force: true });
  }
});

describe("hostie CLI dispatch (compiled binary)", () => {
  test("version command exits 0 with version string", () => {
    const { code, stdout } = runHostie(["version"], TMP_HOME);
    expect(code).toBe(0);
    expect(stdout).toMatch(/^hostie v\d+\.\d+\.\d+/);
  });

  test("--version flag exits 0", () => {
    const { code } = runHostie(["--version"], TMP_HOME);
    expect(code).toBe(0);
  });

  test("list reads ~/.hosts and exits 0", () => {
    const { code, stdout } = runHostie(["list"], TMP_HOME);
    expect(code).toBe(0);
    expect(stdout).toContain("api.work");
  });

  test("list --json emits parseable JSON and exits 0", () => {
    const { code, stdout } = runHostie(["list", "--json"], TMP_HOME);
    expect(code).toBe(0);
    const parsed = JSON.parse(stdout);
    expect(Array.isArray(parsed)).toBe(true);
    expect(parsed[0].hostname).toBe("api.work");
  });

  test("apply --dry-run exits 0 without modifying /etc/hosts", () => {
    const { code, stdout } = runHostie(["apply", "--dry-run"], TMP_HOME);
    expect(code).toBe(0);
    expect(stdout).toContain("Dry-run");
  });

  test("completion bash exits 0 and emits a script", () => {
    const { code, stdout } = runHostie(["completion", "bash"], TMP_HOME);
    expect(code).toBe(0);
    expect(stdout).toContain("_hostie_completion");
  });

  test("unknown command exits non-zero (D14)", () => {
    const { code, stderr } = runHostie(["does-not-exist"], TMP_HOME);
    expect(code).not.toBe(0);
    expect(stderr).toContain("Error");
  });

  test("rm with missing hostname exits non-zero (D14 validation)", () => {
    const { code } = runHostie(["rm", "no-such-host.local"], TMP_HOME);
    // Not found should produce a non-zero exit, not a silent success.
    expect(code).not.toBe(0);
  });
});
