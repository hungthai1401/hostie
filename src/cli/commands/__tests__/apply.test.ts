/**
 * Tests for apply command
 */

import { describe, test, expect, spyOn } from "bun:test";
import { applyCommand } from "../apply";
import * as fileIo from "../../../core/file-io";
import * as apply from "../../../core/apply";
import type { HostsFile } from "../../../domain/types";

describe("applyCommand", () => {
  test("reads ~/.hosts and calls applyHostsFile when not dry-run", async () => {
    const mockHostsFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "test",
          entries: [
            {
              id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
              ip: "127.0.0.1",
              hostname: "test.local",
              aliases: [],
              enabled: true,
            },
          ],
          groups: [],
        },
      ],
    };

    const readSpy = spyOn(fileIo, "readHostsFile").mockResolvedValue(mockHostsFile);
    const applySpy = spyOn(apply, "applyHostsFile").mockResolvedValue({
      changed: true,
      message: "/etc/hosts updated successfully",
    });

    const exitCode = await applyCommand({ dryRun: false });

    expect(readSpy).toHaveBeenCalledWith("~/.hosts");
    expect(applySpy).toHaveBeenCalledWith(mockHostsFile);
    expect(exitCode).toBe(0);

    readSpy.mockRestore();
    applySpy.mockRestore();
  });

  test("shows preview without writing when dry-run is true", async () => {
    const mockHostsFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "test",
          entries: [
            {
              id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
              ip: "127.0.0.1",
              hostname: "test.local",
              aliases: [],
              enabled: true,
            },
          ],
          groups: [],
        },
      ],
    };

    const readSpy = spyOn(fileIo, "readHostsFile").mockResolvedValue(mockHostsFile);
    const applySpy = spyOn(apply, "applyHostsFile");

    const exitCode = await applyCommand({ dryRun: true });

    expect(readSpy).toHaveBeenCalledWith("~/.hosts");
    expect(applySpy).not.toHaveBeenCalled();
    expect(exitCode).toBe(0);

    readSpy.mockRestore();
    applySpy.mockRestore();
  });

  test("returns exit code 2 on I/O error", async () => {
    const readSpy = spyOn(fileIo, "readHostsFile").mockRejectedValue(
      new Error("ENOENT: file not found")
    );

    const exitCode = await applyCommand({ dryRun: false });

    expect(exitCode).toBe(2);

    readSpy.mockRestore();
  });

  test("returns exit code 3 on permission error", async () => {
    const mockHostsFile: HostsFile = {
      version: 1,
      groups: [],
    };

    const permError: any = new Error("EACCES: permission denied");
    permError.code = "EACCES";

    const readSpy = spyOn(fileIo, "readHostsFile").mockResolvedValue(mockHostsFile);
    const applySpy = spyOn(apply, "applyHostsFile").mockRejectedValue(permError);

    const exitCode = await applyCommand({ dryRun: false });

    expect(exitCode).toBe(3);

    readSpy.mockRestore();
    applySpy.mockRestore();
  });

  test("shows no-change message when content is unchanged", async () => {
    const mockHostsFile: HostsFile = {
      version: 1,
      groups: [],
    };

    const readSpy = spyOn(fileIo, "readHostsFile").mockResolvedValue(mockHostsFile);
    const applySpy = spyOn(apply, "applyHostsFile").mockResolvedValue({
      changed: false,
      message: "/etc/hosts is already up to date (no changes needed)",
    });

    const exitCode = await applyCommand({ dryRun: false });

    expect(exitCode).toBe(0);

    readSpy.mockRestore();
    applySpy.mockRestore();
  });
});
