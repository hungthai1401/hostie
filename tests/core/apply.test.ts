/**
 * Unit tests for apply logic (src/core/apply.ts)
 *
 * These tests mock all `fs` I/O via `bun:test`'s `mock.module`, so they:
 *   - never read or write the real /etc/hosts
 *   - never require sudo
 *   - verify idempotency (no write when content is unchanged)
 *   - verify changed-detection (writes happen only when content differs)
 *   - verify error handling (ENOENT, EACCES)
 *   - verify managed-block insertion vs replacement
 *
 * The fs module is mocked BEFORE applyHostsFile is imported so that the
 * direct named imports (`import { readFileSync } from "fs"`) inside
 * src/core/apply.ts resolve to our stubs.
 */

import { describe, test, expect, beforeEach, mock } from "bun:test";

// --- Fs mock state -----------------------------------------------------------

type WriteCall = { path: string; contents: string };

const fsState = {
  currentContent: "",
  reads: 0,
  writes: [] as WriteCall[],
  renames: [] as { from: string; to: string }[],
  readError: null as (Error & { code?: string }) | null,
  writeError: null as (Error & { code?: string }) | null,
};

function resetFs(initial = "") {
  fsState.currentContent = initial;
  fsState.reads = 0;
  fsState.writes = [];
  fsState.renames = [];
  fsState.readError = null;
  fsState.writeError = null;
}

// Mock fs *before* importing apply.ts.
mock.module("fs", () => ({
  readFileSync: (_path: string, _enc?: string) => {
    fsState.reads += 1;
    if (fsState.readError) throw fsState.readError;
    return fsState.currentContent;
  },
  writeFileSync: (path: string, contents: string, _enc?: string) => {
    if (fsState.writeError) throw fsState.writeError;
    fsState.writes.push({ path, contents });
  },
  mkdtempSync: (prefix: string) => `${prefix}XXXXXX`,
  renameSync: (from: string, to: string) => {
    fsState.renames.push({ from, to });
    // Simulate atomic publish: the renamed file becomes /etc/hosts content.
    const last = fsState.writes[fsState.writes.length - 1];
    if (last && last.path === from) {
      fsState.currentContent = last.contents;
    }
  },
}));

// Import AFTER mock.module is registered.
const { applyHostsFile } = await import("../../src/core/apply");
type HostsFile = Parameters<typeof applyHostsFile>[0];

// --- Helpers -----------------------------------------------------------------

const BEGIN = "# BEGIN HOSTIE";
const END = "# END HOSTIE";

function hostsFileWith(entries: Array<{
  ip: string;
  hostname: string;
  aliases?: string[];
  enabled?: boolean;
  comment?: string;
}>): HostsFile {
  return {
    version: 1,
    groups: [
      {
        name: "test",
        entries: entries.map((e, i) => ({
          id: `01J5TEST${i.toString().padStart(18, "0")}`,
          ip: e.ip,
          hostname: e.hostname,
          aliases: e.aliases ?? [],
          enabled: e.enabled ?? true,
          comment: e.comment,
        })),
        groups: [],
      },
    ],
  };
}

// --- Tests -------------------------------------------------------------------

describe("applyHostsFile", () => {
  beforeEach(() => {
    resetFs();
  });

  test("first-time application: appends managed block and reports changed", async () => {
    resetFs("127.0.0.1 localhost\n");

    const hostsFile = hostsFileWith([
      { ip: "192.168.1.10", hostname: "dev.local" },
    ]);

    const result = await applyHostsFile(hostsFile);

    expect(result.changed).toBe(true);
    expect(result.message).toContain("updated");
    expect(fsState.writes.length).toBe(1);
    expect(fsState.renames.length).toBe(1);
    expect(fsState.renames[0]!.to).toBe("/etc/hosts");

    const written = fsState.writes[0]!.contents;
    expect(written).toContain("127.0.0.1 localhost");
    expect(written).toContain(BEGIN);
    expect(written).toContain("192.168.1.10 dev.local");
    expect(written).toContain(END);
  });

  test("idempotency: no write when rendered content matches existing managed block", async () => {
    // Format matches what buildNewContent's replace branch produces:
    // `${before}${BEGIN}\n${block}\n${END}${after}` — note no trailing
    // newline after END when there is no `after` content.
    const initial =
      "127.0.0.1 localhost\n" +
      "\n" +
      `${BEGIN}\n` +
      "# group: test\n" +
      "192.168.1.10 dev.local\n" +
      `${END}`;
    resetFs(initial);

    const hostsFile = hostsFileWith([
      { ip: "192.168.1.10", hostname: "dev.local" },
    ]);

    const result = await applyHostsFile(hostsFile);

    expect(result.changed).toBe(false);
    expect(result.message.toLowerCase()).toContain("up to date");
    expect(fsState.writes.length).toBe(0);
    expect(fsState.renames.length).toBe(0);
    expect(fsState.reads).toBe(1);
  });

  test("changed detection: writes when entry content differs", async () => {
    const initial =
      "127.0.0.1 localhost\n" +
      `${BEGIN}\n` +
      "# group: test\n" +
      "192.168.1.10 old.local\n" +
      `${END}\n`;
    resetFs(initial);

    const hostsFile = hostsFileWith([
      { ip: "192.168.1.20", hostname: "new.local" },
    ]);

    const result = await applyHostsFile(hostsFile);

    expect(result.changed).toBe(true);
    expect(fsState.writes.length).toBe(1);
    const written = fsState.writes[0]!.contents;
    expect(written).toContain("192.168.1.20 new.local");
    expect(written).not.toContain("192.168.1.10 old.local");
    expect(written).toContain("127.0.0.1 localhost"); // outside content preserved
  });

  test("second apply on already-stable content performs no write", async () => {
    // Start from the stable canonical form (no trailing newline after END,
    // which is what the replace branch of buildNewContent produces).
    const stable =
      "127.0.0.1 localhost\n" +
      "\n" +
      `${BEGIN}\n` +
      "# group: test\n" +
      "10.0.0.1 svc.local svc\n" +
      `${END}`;
    resetFs(stable);

    const hostsFile = hostsFileWith([
      { ip: "10.0.0.1", hostname: "svc.local", aliases: ["svc"] },
    ]);

    const result = await applyHostsFile(hostsFile);

    expect(result.changed).toBe(false);
    expect(fsState.writes.length).toBe(0);
  });

  test("disabled entries are not rendered into managed block", async () => {
    resetFs("127.0.0.1 localhost\n");

    const hostsFile = hostsFileWith([
      { ip: "192.168.1.10", hostname: "enabled.local", enabled: true },
      { ip: "192.168.1.11", hostname: "disabled.local", enabled: false },
    ]);

    const result = await applyHostsFile(hostsFile);

    expect(result.changed).toBe(true);
    const written = fsState.writes[0]!.contents;
    expect(written).toContain("enabled.local");
    expect(written).not.toContain("192.168.1.11 disabled.local");
  });

  test("preserves content outside the managed block verbatim", async () => {
    const initial =
      "# system header\n" +
      "127.0.0.1 localhost\n" +
      `${BEGIN}\n` +
      "stale\n" +
      `${END}\n` +
      "# trailing comment\n" +
      "::1 localhost\n";
    resetFs(initial);

    const hostsFile = hostsFileWith([
      { ip: "192.168.1.10", hostname: "fresh.local" },
    ]);

    await applyHostsFile(hostsFile);

    const written = fsState.writes[0]!.contents;
    expect(written).toContain("# system header");
    expect(written).toContain("127.0.0.1 localhost");
    expect(written).toContain("# trailing comment");
    expect(written).toContain("::1 localhost");
    expect(written).toContain("fresh.local");
    expect(written).not.toContain("stale");
  });

  test("ENOENT on read returns changed=false with a descriptive message (no throw)", async () => {
    resetFs("");
    const err: Error & { code?: string } = new Error("no such file");
    err.code = "ENOENT";
    fsState.readError = err;

    const result = await applyHostsFile(hostsFileWith([]));

    expect(result.changed).toBe(false);
    expect(result.message.toLowerCase()).toContain("not found");
    expect(fsState.writes.length).toBe(0);
  });

  test("EACCES on read returns changed=false with a permission-denied message", async () => {
    resetFs("");
    const err: Error & { code?: string } = new Error("permission denied");
    err.code = "EACCES";
    fsState.readError = err;

    const result = await applyHostsFile(hostsFileWith([]));

    expect(result.changed).toBe(false);
    expect(result.message.toLowerCase()).toContain("permission denied");
    expect(fsState.writes.length).toBe(0);
  });

  test("atomic write path: writes to temp file then renames to /etc/hosts", async () => {
    resetFs("127.0.0.1 localhost\n");

    await applyHostsFile(
      hostsFileWith([{ ip: "10.0.0.1", hostname: "a.local" }]),
    );

    expect(fsState.writes.length).toBe(1);
    expect(fsState.renames.length).toBe(1);
    // Temp file path is the one we wrote to; rename target is /etc/hosts.
    expect(fsState.renames[0]!.from).toBe(fsState.writes[0]!.path);
    expect(fsState.renames[0]!.to).toBe("/etc/hosts");
    expect(fsState.writes[0]!.path).not.toBe("/etc/hosts");
  });

  test("entry with aliases is rendered into managed block", async () => {
    resetFs("127.0.0.1 localhost\n");

    await applyHostsFile(
      hostsFileWith([
        {
          ip: "192.168.1.10",
          hostname: "server.local",
          aliases: ["alias1.local", "alias2.local"],
        },
      ]),
    );

    const written = fsState.writes[0]!.contents;
    expect(written).toContain("192.168.1.10 server.local alias1.local alias2.local");
  });

  test("entry with comment is rendered with trailing `# comment`", async () => {
    resetFs("127.0.0.1 localhost\n");

    await applyHostsFile(
      hostsFileWith([
        {
          ip: "192.168.1.10",
          hostname: "server.local",
          comment: "Production server",
        },
      ]),
    );

    const written = fsState.writes[0]!.contents;
    expect(written).toContain("192.168.1.10 server.local # Production server");
  });

  test("empty hosts file with no existing block: appends an empty managed block and reports changed", async () => {
    resetFs("127.0.0.1 localhost\n");

    const result = await applyHostsFile({ version: 1, groups: [] });

    expect(result.changed).toBe(true);
    const written = fsState.writes[0]!.contents;
    expect(written).toContain(BEGIN);
    expect(written).toContain(END);
    expect(written).toContain("127.0.0.1 localhost");
  });
});
