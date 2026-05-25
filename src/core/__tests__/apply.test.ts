/**
 * Tests for applyHostsFile - uses spyOn to avoid touching real /etc/hosts.
 *
 * NOTE: We do NOT use mock.module() here — it is process-global in Bun and
 * leaks across test files, breaking the entire suite. spyOn is scoped and
 * automatically restored by Bun between tests.
 */

import { describe, test, expect, spyOn, beforeEach, afterEach } from "bun:test";
import * as fs from "fs";
import { applyHostsFile, type HostsFile } from "../apply";

const BEGIN_MARKER = "# BEGIN HOSTIE";
const END_MARKER = "# END HOSTIE";

type FsState = {
  initial: string;
  written: string | null;
  writeCount: number;
};

let state: FsState;
const spies: Array<{ mockRestore: () => void }> = [];

beforeEach(() => {
  state = {
    initial: "127.0.0.1 localhost\n",
    written: null,
    writeCount: 0,
  };

  spies.push(
    spyOn(fs, "readFileSync").mockImplementation(((path: any, _opts: any) => {
      if (typeof path === "string" && path === "/etc/hosts") {
        return state.initial;
      }
      if (typeof path === "string" && state.written && path.includes("hostie-")) {
        return state.written;
      }
      return "";
    }) as any)
  );

  spies.push(
    spyOn(fs, "writeFileSync").mockImplementation(((_path: any, data: any) => {
      state.written = String(data);
      state.writeCount++;
    }) as any)
  );

  spies.push(
    spyOn(fs, "renameSync").mockImplementation(((_src: any, _dst: any) => {
      if (state.written !== null) {
        state.initial = state.written;
      }
    }) as any)
  );

  spies.push(
    spyOn(fs, "mkdtempSync").mockImplementation((() => "/tmp/hostie-fake") as any)
  );

  // The atomic-write path in apply.ts also calls statSync/chmodSync/chownSync
  // on the temp file to preserve /etc/hosts ownership across the rename
  // (hosts-cli-379.64). Mock them so the temp file never has to exist on disk.
  spies.push(
    spyOn(fs, "statSync").mockImplementation((() => ({
      mode: 0o100644,
      uid: 0,
      gid: 0,
    })) as any)
  );
  spies.push(
    spyOn(fs, "chmodSync").mockImplementation((() => {}) as any)
  );
  spies.push(
    spyOn(fs, "chownSync").mockImplementation((() => {}) as any)
  );
});

afterEach(() => {
  while (spies.length) {
    spies.pop()!.mockRestore();
  }
});

function makeFile(groups: HostsFile["groups"]): HostsFile {
  return { version: 1, groups };
}

describe("applyHostsFile", () => {
  test("first apply: inserts managed block into existing /etc/hosts", async () => {
    const result = await applyHostsFile(
      makeFile([
        {
          name: "work",
          entries: [
            { id: "1", ip: "10.0.0.1", hostname: "jira.work", aliases: [], enabled: true },
          ],
          groups: [],
        },
      ])
    );

    expect(result.changed).toBe(true);
    expect(state.writeCount).toBe(1);
    expect(state.written).toContain(BEGIN_MARKER);
    expect(state.written).toContain(END_MARKER);
    expect(state.written).toContain("10.0.0.1 jira.work");
    expect(state.written).toContain("127.0.0.1 localhost");
  });

  test("idempotency: no write when content unchanged", async () => {
    // Pre-populate /etc/hosts with the exact block apply would render.
    state.initial =
      "127.0.0.1 localhost\n\n" +
      BEGIN_MARKER +
      "\n" +
      "# group: work\n10.0.0.1 jira.work\n" +
      END_MARKER;

    const result = await applyHostsFile(
      makeFile([
        {
          name: "work",
          entries: [
            { id: "1", ip: "10.0.0.1", hostname: "jira.work", aliases: [], enabled: true },
          ],
          groups: [],
        },
      ])
    );

    expect(result.changed).toBe(false);
    expect(state.writeCount).toBe(0);
    expect(result.message).toContain("up to date");
  });

  test("change detection: replaces existing block when content differs", async () => {
    state.initial =
      "127.0.0.1 localhost\n\n" +
      BEGIN_MARKER +
      "\n# group: old\n1.1.1.1 old.host\n" +
      END_MARKER +
      "\n";

    const result = await applyHostsFile(
      makeFile([
        {
          name: "new",
          entries: [
            { id: "1", ip: "2.2.2.2", hostname: "new.host", aliases: [], enabled: true },
          ],
          groups: [],
        },
      ])
    );

    expect(result.changed).toBe(true);
    expect(state.written).toContain("2.2.2.2 new.host");
    expect(state.written).not.toContain("1.1.1.1 old.host");
    expect(state.written).toContain("127.0.0.1 localhost");
  });

  test("renders disabled entries as #-commented lines (design.md:108)", async () => {
    const result = await applyHostsFile(
      makeFile([
        {
          name: "g",
          entries: [
            { id: "1", ip: "1.1.1.1", hostname: "on.host", aliases: [], enabled: true },
            { id: "2", ip: "2.2.2.2", hostname: "off.host", aliases: [], enabled: false },
          ],
          groups: [],
        },
      ])
    );

    expect(result.changed).toBe(true);
    // Enabled entry: bare line.
    expect(state.written).toContain("1.1.1.1 on.host");
    // Disabled entry: same line prefixed with `# ` per design.md:108.
    // (hosts-cli-379.71 — reverses the .66 regression where disabled
    // entries were dropped entirely.)
    expect(state.written).toContain("# 2.2.2.2 off.host");
  });

  test("renders aliases", async () => {
    await applyHostsFile(
      makeFile([
        {
          name: "g",
          entries: [
            {
              id: "1",
              ip: "10.0.0.5",
              hostname: "srv.local",
              aliases: ["a1.local", "a2.local"],
              enabled: true,
            },
          ],
          groups: [],
        },
      ])
    );

    expect(state.written).toContain("10.0.0.5 srv.local a1.local a2.local");
  });

  test("renders comments", async () => {
    await applyHostsFile(
      makeFile([
        {
          name: "g",
          entries: [
            {
              id: "1",
              ip: "10.0.0.5",
              hostname: "srv.local",
              aliases: [],
              enabled: true,
              comment: "prod box",
            },
          ],
          groups: [],
        },
      ])
    );

    expect(state.written).toContain("10.0.0.5 srv.local # prod box");
  });

  test("handles nested groups with path prefixes", async () => {
    await applyHostsFile(
      makeFile([
        {
          name: "work",
          entries: [
            { id: "1", ip: "10.0.0.1", hostname: "jira.work", aliases: [], enabled: true },
          ],
          groups: [
            {
              name: "prod",
              entries: [
                { id: "2", ip: "10.0.0.2", hostname: "db.prod", aliases: [], enabled: true },
              ],
              groups: [],
            },
          ],
        },
      ])
    );

    expect(state.written).toContain("# group: work");
    expect(state.written).toContain("# group: work/prod");
    expect(state.written).toContain("10.0.0.2 db.prod");
  });

  test("empty groups produce no group header", async () => {
    // Starting from a clean /etc/hosts (no managed block) with empty groups
    // means newBlock is "" — buildNewContent still appends BEGIN/END markers
    // around an empty body, which differs from the original. That's a "change".
    const result = await applyHostsFile(
      makeFile([{ name: "empty", entries: [], groups: [] }])
    );

    expect(result.changed).toBe(true);
    expect(state.written).not.toContain("# group: empty");
    expect(state.written).toContain(BEGIN_MARKER);
    expect(state.written).toContain(END_MARKER);
  });

  test("ENOENT on /etc/hosts returns graceful failure", async () => {
    spyOn(fs, "readFileSync").mockImplementation((() => {
      const err: any = new Error("not found");
      err.code = "ENOENT";
      throw err;
    }) as any);

    const result = await applyHostsFile(makeFile([]));
    expect(result.changed).toBe(false);
    expect(result.message).toContain("not found");
  });

  test("EACCES on read returns graceful failure (no sudo prompt)", async () => {
    spyOn(fs, "readFileSync").mockImplementation((() => {
      const err: any = new Error("permission denied");
      err.code = "EACCES";
      throw err;
    }) as any);

    const result = await applyHostsFile(makeFile([]));
    expect(result.changed).toBe(false);
    expect(result.message).toContain("Permission denied");
  });
  test("design.md:140-147 fixture round-trips: nested groups + disabled entries (hosts-cli-379.71)", async () => {
    // This fixture mirrors the rendered example in design.md:140-147:
    //   # group: work
    //   10.0.1.5  jira.work       (single space here, not double)
    //   # group: work/prod
    //   10.0.2.10 db.prod.work
    //   # 10.0.2.11 db-replica.prod.work
    const result = await applyHostsFile(
      makeFile([
        {
          name: "work",
          entries: [
            { id: "1", ip: "10.0.1.5", hostname: "jira.work", aliases: [], enabled: true },
          ],
          groups: [
            {
              name: "prod",
              entries: [
                { id: "2", ip: "10.0.2.10", hostname: "db.prod.work", aliases: [], enabled: true },
                { id: "3", ip: "10.0.2.11", hostname: "db-replica.prod.work", aliases: [], enabled: false },
              ],
              groups: [],
            },
          ],
        },
      ])
    );

    expect(result.changed).toBe(true);
    const written = state.written ?? "";
    // Group headers use full path syntax.
    expect(written).toContain("# group: work");
    expect(written).toContain("# group: work/prod");
    // Enabled entries: bare lines.
    expect(written).toContain("10.0.1.5 jira.work");
    expect(written).toContain("10.0.2.10 db.prod.work");
    // Disabled entry: `# ` prefix preserves it visibly in /etc/hosts.
    expect(written).toContain("# 10.0.2.11 db-replica.prod.work");
    // BEGIN/END markers wrap the block.
    expect(written).toContain(BEGIN_MARKER);
    expect(written).toContain(END_MARKER);
  });
});
