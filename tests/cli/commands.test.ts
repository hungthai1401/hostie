/**
 * CLI integration tests for hostie subcommands.
 *
 * These tests exercise each CLI subcommand end-to-end, verifying
 * exit codes, --json output, --dry-run behaviour, and file mutations
 * against an isolated hosts file inside a per-test temp directory.
 *
 * Isolation strategy:
 *   - Each test runs against a per-test temp directory holding a
 *     dedicated `.hosts` YAML file.
 *   - Commands that accept an explicit `hostsFile` option
 *     (add/list/group create/group add) receive that path directly.
 *   - Commands hard-wired to `~/.hosts` (rm/enable/disable/apply) are
 *     redirected by spying on `readHostsFile`/`writeHostsFile`:
 *     any call asking for `~/.hosts` is rerouted to the temp file.
 *     We rely on `bun:test`'s `spyOn` which mutates the module export
 *     in place — every command's `import { readHostsFile, … }` sees
 *     the redirect, so no real `~/.hosts` mutation can occur.
 *   - The `apply` command is only tested with --dry-run; we never
 *     touch /etc/hosts (and `core/apply` is spied to assert that).
 *
 * Why not override HOME?
 *   Bun's `os.homedir()` caches at startup and ignores later
 *   `process.env.HOME` mutations, so a HOME override is not a safe
 *   isolation primitive on Bun. Spying on the file-io module is.
 */

import {
  describe,
  test,
  expect,
  beforeEach,
  afterEach,
  spyOn,
} from "bun:test";
import { mkdtempSync, rmSync, writeFileSync, readFileSync } from "fs";
import { join } from "path";
import { tmpdir } from "os";

import { parseCLI } from "../../src/cli/index";
import { addCommand } from "../../src/cli/commands/add";
import { rmCommand } from "../../src/cli/commands/rm";
import { enableCommand } from "../../src/cli/commands/enable";
import { disableCommand } from "../../src/cli/commands/disable";
import { listCommand } from "../../src/cli/commands/list";
import { applyCommand } from "../../src/cli/commands/apply";
import {
  groupCreateCommand,
  groupAddCommand,
} from "../../src/cli/commands/group";
import { versionCommand } from "../../src/cli/commands/version";

import * as fileIo from "../../src/core/file-io";
import * as applyCore from "../../src/core/apply";
import type { HostsFile } from "../../src/domain/types";

// ---------------------------------------------------------------------------
// Per-test temp file + spies
// ---------------------------------------------------------------------------

let TMP_DIR: string;
let TMP_HOSTS: string;

/**
 * Spies installed in beforeEach so we can mockRestore() them in afterEach.
 * They redirect any call to readHostsFile/writeHostsFile that targets
 * "~/.hosts" (or undefined, which defaults to ~/.hosts) onto TMP_HOSTS.
 * Explicit absolute/relative paths are forwarded unchanged.
 */
let readSpy: ReturnType<typeof spyOn>;
let writeSpy: ReturnType<typeof spyOn>;

// Capture the genuine implementations ONCE at module load, before any spy
// has touched the export. This avoids the "real-points-at-stale-spy" trap
// where mockRestore between tests can leave hidden chains of mocks.
const REAL_READ = fileIo.readHostsFile;
const REAL_WRITE = fileIo.writeHostsFile;

function redirectsToTempPath(filepath?: string): string {
  if (!filepath || filepath === "~/.hosts") return TMP_HOSTS;
  return filepath;
}

beforeEach(() => {
  TMP_DIR = mkdtempSync(join(tmpdir(), "hostie-cli-it-"));
  TMP_HOSTS = join(TMP_DIR, ".hosts");

  readSpy = spyOn(fileIo, "readHostsFile").mockImplementation(
    async (filepath?: string) => REAL_READ(redirectsToTempPath(filepath))
  );
  writeSpy = spyOn(fileIo, "writeHostsFile").mockImplementation(
    async (filepath: string | undefined, data: HostsFile) =>
      REAL_WRITE(redirectsToTempPath(filepath), data)
  );

  // Seed an explicit empty hosts file at TMP_HOSTS. We intentionally avoid
  // the "file does not exist, auto-create" code path in file-io: that path
  // returns a shared module-level DEFAULT_HOSTS_FILE singleton, which is
  // mutated by subsequent add operations and would leak entries across
  // tests. Writing a fresh empty document here forces every read to go
  // through the YAML parser and return an independent object.
  writeFileSync(TMP_HOSTS, "version: 1\ngroups: []\n");
});

afterEach(() => {
  readSpy?.mockRestore();
  writeSpy?.mockRestore();
  try {
    rmSync(TMP_DIR, { recursive: true, force: true });
  } catch {
    // best-effort cleanup
  }
});

/**
 * Capture console.log / console.error output during a callback.
 * Restores originals even if the callback throws.
 */
async function captureOutput<T>(
  fn: () => Promise<T>
): Promise<{ result: T; stdout: string; stderr: string }> {
  const originalLog = console.log;
  const originalError = console.error;
  let stdout = "";
  let stderr = "";

  console.log = (...args: any[]) => {
    stdout +=
      args
        .map((a) => (typeof a === "string" ? a : JSON.stringify(a)))
        .join(" ") + "\n";
  };
  console.error = (...args: any[]) => {
    stderr +=
      args
        .map((a) => (typeof a === "string" ? a : JSON.stringify(a)))
        .join(" ") + "\n";
  };

  try {
    const result = await fn();
    return { result, stdout, stderr };
  } finally {
    console.log = originalLog;
    console.error = originalError;
  }
}

/**
 * Read the temp hosts file directly (bypassing the redirected spy)
 * via the real fileIo function — the spy's mockImplementation also
 * dispatches to the real function, so we just call it through the
 * spy with TMP_HOSTS as an explicit path.
 */
async function readTempHosts(): Promise<HostsFile> {
  return fileIo.readHostsFile(TMP_HOSTS);
}

// ---------------------------------------------------------------------------
// parseCLI: dispatch surface for every subcommand
// ---------------------------------------------------------------------------

describe("CLI parsing dispatches every subcommand", () => {
  test("add", () => {
    const parsed = parseCLI([
      "add",
      "10.0.0.1",
      "api.local",
      "--group",
      "work/prod",
      "--alias",
      "api",
      "--comment",
      "primary",
    ]);
    expect(parsed.command).toBe("add");
    expect(parsed.args?.ip).toBe("10.0.0.1");
    expect(parsed.args?.hostname).toBe("api.local");
    expect(parsed.args?.group).toBe("work/prod");
    expect(parsed.args?.aliases).toEqual(["api"]);
    expect(parsed.args?.comment).toBe("primary");
  });

  test("rm", () => {
    const parsed = parseCLI(["rm", "api.local"]);
    expect(parsed.command).toBe("rm");
    expect(parsed.args?.target).toBe("api.local");
  });

  test("enable / disable", () => {
    expect(parseCLI(["enable", "api.local"]).command).toBe("enable");
    expect(parseCLI(["disable", "api.local"]).command).toBe("disable");
  });

  test("list with --json", () => {
    const parsed = parseCLI(["list", "--json"]);
    expect(parsed.command).toBe("list");
    expect(parsed.args?.json).toBe(true);
  });

  test("apply with --dry-run", () => {
    const parsed = parseCLI(["apply", "--dry-run"]);
    expect(parsed.command).toBe("apply");
    expect(parsed.args?.dryRun).toBe(true);
  });

  test("group add subcommand", () => {
    const parsed = parseCLI(["group", "add", "staging"]);
    expect(parsed.command).toBe("group");
    expect(parsed.subcommand).toBe("add");
    expect(parsed.args?.path).toBe("staging");
  });

  test("version", () => {
    const parsed = parseCLI(["version"]);
    expect(parsed.command).toBe("version");
  });

  test("unknown command surfaces error", async () => {
    // Suppress commander's stderr noise; parseCLI catches the error itself.
    const { result: parsed } = await captureOutput(async () =>
      parseCLI(["totally-not-a-command"])
    );
    expect(parsed.command).toBe("unknown");
    expect(parsed.error).toBeDefined();
  });
});

// ---------------------------------------------------------------------------
// add: end-to-end against the redirected temp hosts file
// ---------------------------------------------------------------------------

describe("hostie add (end-to-end)", () => {
  test("adds a new entry and exits 0 (default ~/.hosts redirected)", async () => {
    const { result: exitCode } = await captureOutput(() =>
      addCommand("192.168.10.5", "svc.local", [])
    );
    expect(exitCode).toBe(0);

    const hostsFile = await readTempHosts();
    const allEntries = hostsFile.groups.flatMap((g) => g.entries);
    const found = allEntries.find((e) => e.hostname === "svc.local");
    expect(found).toBeDefined();
    expect(found?.ip).toBe("192.168.10.5");
    expect(found?.enabled).toBe(true);
  });

  test("adds entry with aliases and --group placement", async () => {
    const { result: exitCode } = await captureOutput(() =>
      addCommand("10.0.0.50", "db.work", ["db", "database"], {
        group: "work/prod",
      })
    );
    expect(exitCode).toBe(0);

    const hostsFile = await readTempHosts();
    const work = hostsFile.groups.find((g) => g.name === "work");
    expect(work).toBeDefined();
    const prod = work?.groups.find((g) => g.name === "prod");
    expect(prod).toBeDefined();
    expect(prod?.entries[0].hostname).toBe("db.work");
    expect(prod?.entries[0].aliases).toEqual(["db", "database"]);
  });

  test("rejects invalid IP with exit code 1", async () => {
    const { result: exitCode, stderr } = await captureOutput(() =>
      addCommand("not-an-ip", "svc.local", [])
    );
    expect(exitCode).toBe(1);
    expect(stderr).toContain("Error");
  });

  test("rejects duplicate hostname with exit code 1", async () => {
    await captureOutput(() => addCommand("10.0.0.1", "dup.local", []));
    const { result: exitCode } = await captureOutput(() =>
      addCommand("10.0.0.2", "dup.local", [])
    );
    expect(exitCode).toBe(1);
  });

  test("explicit hostsFile option also works (bypasses default)", async () => {
    const alt = join(TMP_DIR, ".hosts-alt");
    const { result: exitCode } = await captureOutput(() =>
      addCommand("10.1.1.1", "alt.local", [], { hostsFile: alt })
    );
    expect(exitCode).toBe(0);

    const hostsFile = await fileIo.readHostsFile(alt);
    const found = hostsFile.groups
      .flatMap((g) => g.entries)
      .find((e) => e.hostname === "alt.local");
    expect(found).toBeDefined();
  });
});

// ---------------------------------------------------------------------------
// rm: end-to-end via spy redirection
// ---------------------------------------------------------------------------

describe("hostie rm (end-to-end)", () => {
  test("removes an existing entry and exits 0", async () => {
    await captureOutput(() => addCommand("10.0.0.1", "removeme.local", []));
    const { result: exitCode } = await captureOutput(() =>
      rmCommand("removeme.local")
    );
    expect(exitCode).toBe(0);

    const hostsFile = await readTempHosts();
    const allEntries = hostsFile.groups.flatMap((g) => g.entries);
    expect(
      allEntries.find((e) => e.hostname === "removeme.local")
    ).toBeUndefined();
  });

  test("returns exit code 1 when hostname does not exist", async () => {
    await fileIo.writeHostsFile(TMP_HOSTS, { version: 1, groups: [] });
    const { result: exitCode, stderr } = await captureOutput(() =>
      rmCommand("nonexistent.local")
    );
    expect(exitCode).toBe(1);
    expect(stderr).toContain("not found");
  });
});

// ---------------------------------------------------------------------------
// enable / disable: end-to-end via spy redirection
// ---------------------------------------------------------------------------

describe("hostie enable / disable (end-to-end)", () => {
  test("disable flips enabled to false, enable flips back", async () => {
    await captureOutput(() => addCommand("10.0.0.1", "toggle.local", []));

    const { result: disableExit } = await captureOutput(() =>
      disableCommand("toggle.local")
    );
    expect(disableExit).toBe(0);

    let hostsFile = await readTempHosts();
    let entry = hostsFile.groups
      .flatMap((g) => g.entries)
      .find((e) => e.hostname === "toggle.local");
    expect(entry?.enabled).toBe(false);

    const { result: enableExit } = await captureOutput(() =>
      enableCommand("toggle.local")
    );
    expect(enableExit).toBe(0);

    hostsFile = await readTempHosts();
    entry = hostsFile.groups
      .flatMap((g) => g.entries)
      .find((e) => e.hostname === "toggle.local");
    expect(entry?.enabled).toBe(true);
  });

  test("enable returns exit 1 when hostname is not found", async () => {
    await fileIo.writeHostsFile(TMP_HOSTS, { version: 1, groups: [] });
    const { result: exitCode } = await captureOutput(() =>
      enableCommand("ghost.local")
    );
    expect(exitCode).toBe(1);
  });

  test("disable returns exit 1 when hostname is not found", async () => {
    await fileIo.writeHostsFile(TMP_HOSTS, { version: 1, groups: [] });
    const { result: exitCode } = await captureOutput(() =>
      disableCommand("ghost.local")
    );
    expect(exitCode).toBe(1);
  });
});

// ---------------------------------------------------------------------------
// list: human-readable and --json
// ---------------------------------------------------------------------------

describe("hostie list (end-to-end)", () => {
  test("returns 0 with human-readable table output", async () => {
    await captureOutput(() => addCommand("10.0.0.1", "one.local", []));
    await captureOutput(() =>
      addCommand("10.0.0.2", "two.local", [], { group: "work" })
    );

    const { result: exitCode, stdout } = await captureOutput(() =>
      listCommand({})
    );
    expect(exitCode).toBe(0);
    expect(stdout).toContain("one.local");
    expect(stdout).toContain("two.local");
    expect(stdout).toContain("Status"); // table header
  });

  test("--json emits parseable JSON array with group paths", async () => {
    await captureOutput(() => addCommand("10.0.0.1", "one.local", []));
    await captureOutput(() =>
      addCommand("10.0.0.2", "two.local", ["dub"], { group: "work/prod" })
    );

    const { result: exitCode, stdout } = await captureOutput(() =>
      listCommand({ json: true })
    );
    expect(exitCode).toBe(0);

    const parsed = JSON.parse(stdout);
    expect(Array.isArray(parsed)).toBe(true);
    expect(parsed.length).toBe(2);

    const two = parsed.find((e: any) => e.hostname === "two.local");
    expect(two).toBeDefined();
    // list builds group path as "work/prod" (joined by "/") and we just
    // assert it contains the nested segment.
    expect(two.group).toContain("prod");
    expect(two.aliases).toEqual(["dub"]);
  });

  test("returns exit code 2 on I/O error (bad explicit hostsFile)", async () => {
    const { result: exitCode } = await captureOutput(() =>
      listCommand({ hostsFile: "/nonexistent/dir-that-does-not-exist/.hosts" })
    );
    expect(exitCode).toBe(2);
  });
});

// ---------------------------------------------------------------------------
// apply --dry-run: never touches /etc/hosts
// ---------------------------------------------------------------------------

describe("hostie apply --dry-run (end-to-end)", () => {
  test("dry-run shows preview, exits 0, does not call applyHostsFile", async () => {
    await captureOutput(() => addCommand("10.0.0.1", "preview.local", []));

    const applySpy = spyOn(applyCore, "applyHostsFile");

    const { result: exitCode, stdout } = await captureOutput(() =>
      applyCommand({ dryRun: true })
    );

    expect(exitCode).toBe(0);
    expect(applySpy).not.toHaveBeenCalled();
    expect(stdout).toContain("Dry-run");
    // The render layer should emit our hostname somewhere in the preview.
    expect(stdout).toContain("preview.local");

    applySpy.mockRestore();
  });

  test("dry-run on an empty managed block still exits 0", async () => {
    await fileIo.writeHostsFile(TMP_HOSTS, { version: 1, groups: [] });
    const applySpy = spyOn(applyCore, "applyHostsFile");

    const { result: exitCode } = await captureOutput(() =>
      applyCommand({ dryRun: true })
    );

    expect(exitCode).toBe(0);
    expect(applySpy).not.toHaveBeenCalled();

    applySpy.mockRestore();
  });
});

// ---------------------------------------------------------------------------
// group create / group add: end-to-end with isolated hosts file
// ---------------------------------------------------------------------------

describe("hostie group (end-to-end)", () => {
  test("group create adds a new root group and exits 0", async () => {
    const { result: exitCode } = await captureOutput(() =>
      groupCreateCommand("staging")
    );
    expect(exitCode).toBe(0);

    const hostsFile = await readTempHosts();
    expect(hostsFile.groups.find((g) => g.name === "staging")).toBeDefined();
  });

  test("group create with invalid name exits 1", async () => {
    const { result: exitCode } = await captureOutput(() =>
      groupCreateCommand("BadName")
    );
    expect(exitCode).toBe(1);
  });

  test("group create nested via --parent", async () => {
    await captureOutput(() => groupCreateCommand("work"));
    const { result: exitCode } = await captureOutput(() =>
      groupCreateCommand("prod", { parent: "work" })
    );
    expect(exitCode).toBe(0);

    const hostsFile = await readTempHosts();
    const work = hostsFile.groups.find((g) => g.name === "work");
    expect(work?.groups.find((g) => g.name === "prod")).toBeDefined();
  });

  test("group add moves an existing entry into a group and exits 0", async () => {
    await captureOutput(() => addCommand("10.0.0.1", "mover.local", []));
    await captureOutput(() => groupCreateCommand("work"));

    const { result: exitCode } = await captureOutput(() =>
      groupAddCommand("work", "mover.local")
    );
    expect(exitCode).toBe(0);

    const hostsFile = await readTempHosts();
    const work = hostsFile.groups.find((g) => g.name === "work");
    expect(
      work?.entries.find((e) => e.hostname === "mover.local")
    ).toBeDefined();

    // Entry should no longer live in the ungrouped/root bucket.
    const root = hostsFile.groups.find((g) => g.name === "");
    expect(
      root?.entries.find((e) => e.hostname === "mover.local")
    ).toBeUndefined();
  });

  test("group add returns exit 1 when entry is missing", async () => {
    await captureOutput(() => groupCreateCommand("work"));
    const { result: exitCode } = await captureOutput(() =>
      groupAddCommand("work", "ghost.local")
    );
    expect(exitCode).toBe(1);
  });

  test("group add returns exit 1 when target group is missing", async () => {
    await captureOutput(() => addCommand("10.0.0.1", "lonely.local", []));
    const { result: exitCode } = await captureOutput(() =>
      groupAddCommand("nonexistent-group", "lonely.local")
    );
    expect(exitCode).toBe(1);
  });
});

// ---------------------------------------------------------------------------
// version
// ---------------------------------------------------------------------------

describe("hostie version (end-to-end)", () => {
  test("prints version string and exits 0", async () => {
    const { result: exitCode, stdout } = await captureOutput(() =>
      versionCommand()
    );
    expect(exitCode).toBe(0);
    expect(stdout).toMatch(/hostie v\d+\.\d+\.\d+/);
  });
});
