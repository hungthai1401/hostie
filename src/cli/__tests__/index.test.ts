import { describe, test, expect } from "bun:test";
import { parseCLI } from "../index";

describe("parseCLI", () => {
  test("shows help with --help flag", () => {
    const result = parseCLI(["--help"]);
    expect(result.command).toBe("help");
    expect(result.showHelp).toBe(true);
  });

  test("shows version with --version flag", () => {
    const result = parseCLI(["--version"]);
    expect(result.command).toBe("version");
  });

  test("defaults to tui command when no args", () => {
    const result = parseCLI([]);
    expect(result.command).toBe("tui");
  });

  test("parses add command with required args", () => {
    const result = parseCLI(["add", "192.168.1.1", "example.local"]);
    expect(result.command).toBe("add");
    expect(result.args).toEqual({
      ip: "192.168.1.1",
      hostname: "example.local",
    });
  });

  test("parses add command with optional flags", () => {
    const result = parseCLI([
      "add",
      "10.0.0.1",
      "db.local",
      "--group=work/prod",
      "--alias=database",
      "--alias=db",
      "--disabled",
      "--comment=Production database",
    ]);
    expect(result.command).toBe("add");
    expect(result.args).toEqual({
      ip: "10.0.0.1",
      hostname: "db.local",
      group: "work/prod",
      aliases: ["database", "db"],
      disabled: true,
      comment: "Production database",
    });
  });

  test("parses rm command", () => {
    const result = parseCLI(["rm", "example.local"]);
    expect(result.command).toBe("rm");
    expect(result.args).toEqual({
      target: "example.local",
    });
  });

  test("parses enable command", () => {
    const result = parseCLI(["enable", "example.local"]);
    expect(result.command).toBe("enable");
    expect(result.args).toEqual({
      hostname: "example.local",
    });
  });

  test("parses disable command", () => {
    const result = parseCLI(["disable", "example.local"]);
    expect(result.command).toBe("disable");
    expect(result.args).toEqual({
      hostname: "example.local",
    });
  });

  test("parses list command with no args", () => {
    const result = parseCLI(["list"]);
    expect(result.command).toBe("list");
    expect(result.args).toEqual({});
  });

  test("parses list command with group filter", () => {
    const result = parseCLI(["list", "--group=work/prod"]);
    expect(result.command).toBe("list");
    expect(result.args).toEqual({
      group: "work/prod",
    });
  });

  test("parses list command with json flag", () => {
    const result = parseCLI(["list", "--json"]);
    expect(result.command).toBe("list");
    expect(result.args).toEqual({
      json: true,
    });
  });

  test("parses apply command", () => {
    const result = parseCLI(["apply"]);
    expect(result.command).toBe("apply");
    expect(result.args).toEqual({});
  });

  test("parses apply command with dry-run flag", () => {
    const result = parseCLI(["apply", "--dry-run"]);
    expect(result.command).toBe("apply");
    expect(result.args).toEqual({
      dryRun: true,
    });
  });

  test("parses group add command", () => {
    const result = parseCLI(["group", "add", "work/staging"]);
    expect(result.command).toBe("group");
    expect(result.subcommand).toBe("add");
    expect(result.args).toEqual({
      path: "work/staging",
    });
  });

  test("parses group rm command", () => {
    const result = parseCLI(["group", "rm", "work/old"]);
    expect(result.command).toBe("group");
    expect(result.subcommand).toBe("rm");
    expect(result.args).toEqual({
      path: "work/old",
    });
  });

  test("parses group rm command with force flag", () => {
    const result = parseCLI(["group", "rm", "work/old", "--force"]);
    expect(result.command).toBe("group");
    expect(result.subcommand).toBe("rm");
    expect(result.args).toEqual({
      path: "work/old",
      force: true,
    });
  });

  test("parses group list command", () => {
    const result = parseCLI(["group", "list"]);
    expect(result.command).toBe("group");
    expect(result.subcommand).toBe("list");
  });

  test("parses group mv command", () => {
    const result = parseCLI(["group", "mv", "work/old", "work/new"]);
    expect(result.command).toBe("group");
    expect(result.subcommand).toBe("mv");
    expect(result.args).toEqual({
      path: "work/old",
      dest: "work/new",
    });
  });

  test("parses completion command for bash", () => {
    const result = parseCLI(["completion", "bash"]);
    expect(result.command).toBe("completion");
    expect(result.args).toEqual({
      shell: "bash",
    });
  });

  test("parses completion command for zsh", () => {
    const result = parseCLI(["completion", "zsh"]);
    expect(result.command).toBe("completion");
    expect(result.args).toEqual({
      shell: "zsh",
    });
  });

  test("parses completion command for fish", () => {
    const result = parseCLI(["completion", "fish"]);
    expect(result.command).toBe("completion");
    expect(result.args).toEqual({
      shell: "fish",
    });
  });

  test("handles unknown command", () => {
    const result = parseCLI(["unknown"]);
    expect(result.command).toBe("unknown");
    expect(result.error).toBeDefined();
  });
});
