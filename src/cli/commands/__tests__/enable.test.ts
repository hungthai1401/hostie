/**
 * Tests for enable command
 */

import { describe, test, expect, spyOn } from "bun:test";
import { enableCommand } from "../enable";
import * as fileIo from "../../../core/file-io";
import type { HostsFile } from "../../../domain/types";

describe("enableCommand", () => {
  test("enables a disabled entry successfully", async () => {
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
              enabled: false,
            },
          ],
          groups: [],
        },
      ],
    };

    const readSpy = spyOn(fileIo, "readHostsFile").mockResolvedValue(mockHostsFile);
    const writeSpy = spyOn(fileIo, "writeHostsFile").mockResolvedValue(undefined);

    const exitCode = await enableCommand("test.local");

    expect(readSpy).toHaveBeenCalledWith("~/.hosts");
    expect(writeSpy).toHaveBeenCalledTimes(1);
    
    // Verify the written data has the entry enabled
    const writtenData = writeSpy.mock.calls[0][1] as HostsFile;
    expect(writtenData.groups[0].entries[0].enabled).toBe(true);
    expect(exitCode).toBe(0);

    readSpy.mockRestore();
    writeSpy.mockRestore();
  });

  test("is idempotent - enabling already-enabled entry succeeds", async () => {
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
    const writeSpy = spyOn(fileIo, "writeHostsFile").mockResolvedValue(undefined);

    const exitCode = await enableCommand("test.local");

    expect(readSpy).toHaveBeenCalledWith("~/.hosts");
    expect(writeSpy).toHaveBeenCalledTimes(1);
    
    // Verify the entry remains enabled
    const writtenData = writeSpy.mock.calls[0][1] as HostsFile;
    expect(writtenData.groups[0].entries[0].enabled).toBe(true);
    expect(exitCode).toBe(0);

    readSpy.mockRestore();
    writeSpy.mockRestore();
  });

  test("returns exit code 1 when hostname not found", async () => {
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
              enabled: false,
            },
          ],
          groups: [],
        },
      ],
    };

    const readSpy = spyOn(fileIo, "readHostsFile").mockResolvedValue(mockHostsFile);
    const writeSpy = spyOn(fileIo, "writeHostsFile");

    const exitCode = await enableCommand("nonexistent.local");

    expect(readSpy).toHaveBeenCalledWith("~/.hosts");
    expect(writeSpy).not.toHaveBeenCalled();
    expect(exitCode).toBe(1);

    readSpy.mockRestore();
    writeSpy.mockRestore();
  });

  test("returns exit code 2 on I/O error during read", async () => {
    const readSpy = spyOn(fileIo, "readHostsFile").mockRejectedValue(
      new Error("ENOENT: file not found")
    );

    const exitCode = await enableCommand("test.local");

    expect(exitCode).toBe(2);

    readSpy.mockRestore();
  });

  test("returns exit code 2 on I/O error during write", async () => {
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
              enabled: false,
            },
          ],
          groups: [],
        },
      ],
    };

    const readSpy = spyOn(fileIo, "readHostsFile").mockResolvedValue(mockHostsFile);
    const writeSpy = spyOn(fileIo, "writeHostsFile").mockRejectedValue(
      new Error("EACCES: permission denied")
    );

    const exitCode = await enableCommand("test.local");

    expect(exitCode).toBe(2);

    readSpy.mockRestore();
    writeSpy.mockRestore();
  });

  test("searches nested groups recursively", async () => {
    const mockHostsFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "parent",
          entries: [
            {
              id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
              ip: "127.0.0.1",
              hostname: "parent.local",
              aliases: [],
              enabled: true,
            },
          ],
          groups: [
            {
              name: "child",
              entries: [
                {
                  id: "01ARZ3NDEKTSV4RRFFQ69G5FAW",
                  ip: "192.168.1.1",
                  hostname: "child.local",
                  aliases: [],
                  enabled: false,
                },
              ],
              groups: [],
            },
          ],
        },
      ],
    };

    const readSpy = spyOn(fileIo, "readHostsFile").mockResolvedValue(mockHostsFile);
    const writeSpy = spyOn(fileIo, "writeHostsFile").mockResolvedValue(undefined);

    const exitCode = await enableCommand("child.local");

    const writtenData = writeSpy.mock.calls[0][1] as HostsFile;
    expect(writtenData.groups[0].entries[0].enabled).toBe(true);
    expect(writtenData.groups[0].groups[0].entries[0].enabled).toBe(true);
    expect(exitCode).toBe(0);

    readSpy.mockRestore();
    writeSpy.mockRestore();
  });

  test("preserves other entries unchanged", async () => {
    const mockHostsFile: HostsFile = {
      version: 1,
      groups: [
        {
          name: "test",
          entries: [
            {
              id: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
              ip: "127.0.0.1",
              hostname: "first.local",
              aliases: [],
              enabled: true,
            },
            {
              id: "01ARZ3NDEKTSV4RRFFQ69G5FAW",
              ip: "127.0.0.2",
              hostname: "second.local",
              aliases: [],
              enabled: false,
            },
            {
              id: "01ARZ3NDEKTSV4RRFFQ69G5FAX",
              ip: "127.0.0.3",
              hostname: "third.local",
              aliases: [],
              enabled: false,
            },
          ],
          groups: [],
        },
      ],
    };

    const readSpy = spyOn(fileIo, "readHostsFile").mockResolvedValue(mockHostsFile);
    const writeSpy = spyOn(fileIo, "writeHostsFile").mockResolvedValue(undefined);

    const exitCode = await enableCommand("second.local");

    const writtenData = writeSpy.mock.calls[0][1] as HostsFile;
    expect(writtenData.groups[0].entries).toHaveLength(3);
    expect(writtenData.groups[0].entries[0].enabled).toBe(true);
    expect(writtenData.groups[0].entries[1].enabled).toBe(true);
    expect(writtenData.groups[0].entries[2].enabled).toBe(false);
    expect(exitCode).toBe(0);

    readSpy.mockRestore();
    writeSpy.mockRestore();
  });
});
