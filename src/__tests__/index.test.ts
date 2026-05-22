/**
 * Tests for main entry point
 */

import { describe, test, expect, mock } from "bun:test";
import { parseCLI } from "../cli/index.js";

describe("main entry point", () => {
  test("module exports main function", async () => {
    const indexModule = await import("../index.js");
    expect(indexModule.main).toBeDefined();
    expect(typeof indexModule.main).toBe("function");
  });

  test("parseCLI returns tui command for empty args", () => {
    const result = parseCLI([]);
    expect(result.command).toBe("tui");
  });

  test("parseCLI returns add command with args", () => {
    const result = parseCLI(["add", "127.0.0.1", "example.local"]);
    expect(result.command).toBe("add");
    expect(result.args?.ip).toBe("127.0.0.1");
    expect(result.args?.hostname).toBe("example.local");
  });

  test("parseCLI handles --help flag", () => {
    const result = parseCLI(["--help"]);
    expect(result.command).toBe("help");
    expect(result.showHelp).toBe(true);
  });

  test("parseCLI handles --version flag", () => {
    const result = parseCLI(["--version"]);
    expect(result.command).toBe("version");
  });

  test("parseCLI handles version command", () => {
    const result = parseCLI(["version"]);
    expect(result.command).toBe("version");
  });

  test("parseCLI handles list command", () => {
    const result = parseCLI(["list"]);
    expect(result.command).toBe("list");
  });

  test("parseCLI handles apply command", () => {
    const result = parseCLI(["apply"]);
    expect(result.command).toBe("apply");
  });

  test("parseCLI handles group subcommands", () => {
    const result = parseCLI(["group", "list"]);
    expect(result.command).toBe("group");
    expect(result.subcommand).toBe("list");
  });
});
