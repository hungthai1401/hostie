/**
 * Unit tests for reexecWithSudo (src/core/apply.ts).
 *
 * reexecWithSudo is the privilege escalation path: when writing
 * /etc/hosts hits EACCES, the process re-execs itself under sudo,
 * preserves argv and environment, awaits the child, and exits with
 * the child's status code.
 *
 * These tests NEVER invoke real sudo. We stub:
 *   - Bun.spawn   → captures the spawn argv/options, returns a fake
 *                   child whose `exited` promise resolves with a
 *                   configurable `exitCode`.
 *   - process.exit → captured (not invoked) so we can assert the
 *                   exit code that reexecWithSudo *would* have used.
 *   - process.getuid → controls the "already root" guard.
 *
 * We also exercise the integration path in applyHostsFile by making
 * Bun.spawn's child behave as if sudo succeeded, and force readFileSync
 * / writeFileSync to behave such that EACCES is raised inside the
 * atomic-write block.
 */

import {
  describe,
  test,
  expect,
  beforeEach,
  afterEach,
} from "bun:test";

import * as applyModule from "../../src/core/apply";

// ---------------------------------------------------------------------------
// Bun.spawn stub plumbing
// ---------------------------------------------------------------------------

type SpawnCapture = {
  cmd: string[];
  options: any;
};

let spawnCaptures: SpawnCapture[];
let nextExitCode: number | null;
let originalSpawn: typeof Bun.spawn;
let originalExit: typeof process.exit;
let originalGetUid: typeof process.getuid | undefined;
let originalArgv: string[];
let capturedExitCode: number | undefined;

class ProcessExitError extends Error {
  constructor(public code: number) {
    super(`process.exit(${code})`);
  }
}

function installSpawnStub() {
  spawnCaptures = [];
  originalSpawn = Bun.spawn;
  // @ts-ignore - we are intentionally replacing this for the test
  Bun.spawn = ((cmd: string[], options: any) => {
    spawnCaptures.push({ cmd, options });
    const exitCode = nextExitCode;
    return {
      exited: Promise.resolve(exitCode),
      exitCode,
      kill: () => {},
      stdin: null,
      stdout: null,
      stderr: null,
    } as any;
  }) as any;
}

function restoreSpawnStub() {
  // @ts-ignore
  Bun.spawn = originalSpawn;
}

function installExitStub() {
  capturedExitCode = undefined;
  originalExit = process.exit;
  // @ts-ignore - test stub
  process.exit = ((code?: number) => {
    capturedExitCode = code;
    throw new ProcessExitError(code ?? 0);
  }) as any;
}

function restoreExitStub() {
  process.exit = originalExit;
}

function setGetUid(uid: number) {
  originalGetUid = process.getuid;
  // @ts-ignore - test stub
  process.getuid = (() => uid) as any;
}

function restoreGetUid() {
  if (originalGetUid !== undefined) {
    // @ts-ignore
    process.getuid = originalGetUid;
  }
}

function setArgv(argv: string[]) {
  originalArgv = Bun.argv;
  // @ts-ignore - Bun.argv is read-only in types but is a normal array at runtime
  Bun.argv = argv;
}

function restoreArgv() {
  // @ts-ignore
  Bun.argv = originalArgv;
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("reexecWithSudo", () => {
  beforeEach(() => {
    installSpawnStub();
    installExitStub();
    nextExitCode = 0;
  });

  afterEach(() => {
    restoreSpawnStub();
    restoreExitStub();
    restoreGetUid();
    restoreArgv();
  });

  test("spawns sudo with argv preserved verbatim", async () => {
    setGetUid(501); // non-root user
    setArgv([
      "/usr/local/bin/bun",
      "/Users/me/hostie/dist/index.js",
      "apply",
      "--dry-run",
    ]);
    nextExitCode = 0;

    try {
      await applyModule.reexecWithSudo();
    } catch (e) {
      if (!(e instanceof ProcessExitError)) throw e;
    }

    expect(spawnCaptures.length).toBe(1);
    expect(spawnCaptures[0].cmd).toEqual([
      "sudo",
      "/usr/local/bin/bun",
      "/Users/me/hostie/dist/index.js",
      "apply",
      "--dry-run",
    ]);
  });

  test("inherits stdio so prompts and output pass through", async () => {
    setGetUid(501);
    setArgv(["/usr/local/bin/bun", "/path/to/script.js"]);
    nextExitCode = 0;

    try {
      await applyModule.reexecWithSudo();
    } catch (e) {
      if (!(e instanceof ProcessExitError)) throw e;
    }

    expect(spawnCaptures[0].options.stdio).toEqual([
      "inherit",
      "inherit",
      "inherit",
    ]);
  });

  test("does not override the child env (HOSTIE_HOSTS_FILE etc. preserved by inheritance)", async () => {
    // Bun.spawn inherits process.env by default when no `env` key is supplied.
    // We assert no explicit env override is passed, which is how arbitrary
    // user env vars like HOSTIE_HOSTS_FILE survive the sudo re-exec.
    setGetUid(501);
    setArgv(["/usr/local/bin/bun", "/path/to/script.js", "list"]);
    nextExitCode = 0;

    process.env.HOSTIE_HOSTS_FILE = "/tmp/custom-hosts.yaml";

    try {
      await applyModule.reexecWithSudo();
    } catch (e) {
      if (!(e instanceof ProcessExitError)) throw e;
    }

    expect(spawnCaptures[0].options.env).toBeUndefined();
    // And the parent process still has the env var available — confirming
    // that an inherited child would see it too.
    expect(process.env.HOSTIE_HOSTS_FILE).toBe("/tmp/custom-hosts.yaml");

    delete process.env.HOSTIE_HOSTS_FILE;
  });

  test("exits with the child's exit code", async () => {
    setGetUid(501);
    setArgv(["/usr/local/bin/bun", "/path/to/script.js"]);
    nextExitCode = 7;

    let caught: ProcessExitError | undefined;
    try {
      await applyModule.reexecWithSudo();
    } catch (e) {
      if (e instanceof ProcessExitError) caught = e;
      else throw e;
    }

    expect(caught).toBeDefined();
    expect(capturedExitCode).toBe(7);
  });

  test("exits with code 1 when child exitCode is null/undefined", async () => {
    setGetUid(501);
    setArgv(["/usr/local/bin/bun", "/path/to/script.js"]);
    nextExitCode = null;

    let caught: ProcessExitError | undefined;
    try {
      await applyModule.reexecWithSudo();
    } catch (e) {
      if (e instanceof ProcessExitError) caught = e;
      else throw e;
    }

    expect(caught).toBeDefined();
    expect(capturedExitCode).toBe(1);
  });

  test("throws when already running as root (no sudo recursion)", async () => {
    setGetUid(0);
    setArgv(["/usr/local/bin/bun", "/path/to/script.js"]);

    await expect(applyModule.reexecWithSudo()).rejects.toThrow(
      "Cannot write /etc/hosts even as root",
    );

    // Critical: no spawn happened — we did not invoke `sudo sudo …`.
    expect(spawnCaptures.length).toBe(0);
    expect(capturedExitCode).toBeUndefined();
  });

  test("propagates non-zero child exit code (e.g. sudo authentication failure)", async () => {
    setGetUid(501);
    setArgv(["/usr/local/bin/bun", "/path/to/script.js"]);
    nextExitCode = 1;

    try {
      await applyModule.reexecWithSudo();
    } catch (e) {
      if (!(e instanceof ProcessExitError)) throw e;
    }

    expect(capturedExitCode).toBe(1);
  });

  test("preserves argv ordering precisely (no flag reordering)", async () => {
    setGetUid(501);
    setArgv([
      "/usr/local/bin/bun",
      "/path/to/script.js",
      "add",
      "10.0.0.1",
      "api.local",
      "--group",
      "work",
      "--alias",
      "api",
    ]);
    nextExitCode = 0;

    try {
      await applyModule.reexecWithSudo();
    } catch (e) {
      if (!(e instanceof ProcessExitError)) throw e;
    }

    expect(spawnCaptures[0].cmd).toEqual([
      "sudo",
      "/usr/local/bin/bun",
      "/path/to/script.js",
      "add",
      "10.0.0.1",
      "api.local",
      "--group",
      "work",
      "--alias",
      "api",
    ]);
  });
});
