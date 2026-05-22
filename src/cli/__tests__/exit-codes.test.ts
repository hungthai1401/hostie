/**
 * Tests for exit codes module
 */

import { describe, it, expect } from "bun:test";
import { ExitCode } from "../exit-codes";

describe("ExitCode enum", () => {
  it("should define SUCCESS as 0", () => {
    expect(ExitCode.SUCCESS).toBe(0);
  });

  it("should define VALIDATION as 1", () => {
    expect(ExitCode.VALIDATION).toBe(1);
  });

  it("should define IO_ERROR as 2", () => {
    expect(ExitCode.IO_ERROR).toBe(2);
  });

  it("should define PERMISSION as 3", () => {
    expect(ExitCode.PERMISSION).toBe(3);
  });
});
