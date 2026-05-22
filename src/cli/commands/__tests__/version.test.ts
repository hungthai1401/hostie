/**
 * Tests for version command
 */

import { describe, test, expect } from "bun:test";
import { versionCommand } from "../version";

describe("versionCommand", () => {
  test("outputs version in correct format", async () => {
    // Capture stdout
    const originalLog = console.log;
    let output = "";
    console.log = (msg: string) => {
      output += msg;
    };

    // Act
    const exitCode = await versionCommand();

    // Restore console.log
    console.log = originalLog;

    // Assert
    expect(exitCode).toBe(0);
    expect(output).toMatch(/^hostie v\d+\.\d+\.\d+$/);
  });

  test("shows correct version from package.json", async () => {
    // Capture stdout
    const originalLog = console.log;
    let output = "";
    console.log = (msg: string) => {
      output += msg;
    };

    // Act
    const exitCode = await versionCommand();

    // Restore console.log
    console.log = originalLog;

    // Assert
    expect(exitCode).toBe(0);
    expect(output).toBe("hostie v0.1.0");
  });
});
