/**
 * Tests for completion command
 */

import { describe, test, expect } from "bun:test";
import { completionCommand } from "../completion";

describe("completionCommand", () => {
  test("generates bash completion script", async () => {
    // Capture stdout
    const output: string[] = [];
    const originalLog = console.log;
    console.log = (...args: any[]) => output.push(args.join(" "));

    const exitCode = await completionCommand("bash");

    console.log = originalLog;

    expect(exitCode).toBe(0);
    expect(output.length).toBeGreaterThan(0);
    const script = output.join("\n");
    expect(script).toContain("_hostie_completion");
    expect(script).toContain("bash");
  });

  test("generates zsh completion script", async () => {
    const output: string[] = [];
    const originalLog = console.log;
    console.log = (...args: any[]) => output.push(args.join(" "));

    const exitCode = await completionCommand("zsh");

    console.log = originalLog;

    expect(exitCode).toBe(0);
    expect(output.length).toBeGreaterThan(0);
    const script = output.join("\n");
    expect(script).toContain("_hostie");
    expect(script).toContain("zsh");
  });

  test("generates fish completion script", async () => {
    const output: string[] = [];
    const originalLog = console.log;
    console.log = (...args: any[]) => output.push(args.join(" "));

    const exitCode = await completionCommand("fish");

    console.log = originalLog;

    expect(exitCode).toBe(0);
    expect(output.length).toBeGreaterThan(0);
    const script = output.join("\n");
    expect(script).toContain("hostie");
    expect(script).toContain("complete");
  });

  test("returns error for unsupported shell", async () => {
    const errors: string[] = [];
    const originalError = console.error;
    console.error = (...args: any[]) => errors.push(args.join(" "));

    const exitCode = await completionCommand("powershell" as any);

    console.error = originalError;

    expect(exitCode).toBe(1);
    expect(errors.length).toBeGreaterThan(0);
    expect(errors[0]).toContain("Unsupported shell");
  });

  test("bash script includes dynamic hostname completion", async () => {
    const output: string[] = [];
    const originalLog = console.log;
    console.log = (...args: any[]) => output.push(args.join(" "));

    await completionCommand("bash");

    console.log = originalLog;

    const script = output.join("\n");
    // Should reference hostie list for dynamic completion
    expect(script).toContain("hostie list");
  });

  test("zsh script includes dynamic hostname completion", async () => {
    const output: string[] = [];
    const originalLog = console.log;
    console.log = (...args: any[]) => output.push(args.join(" "));

    await completionCommand("zsh");

    console.log = originalLog;

    const script = output.join("\n");
    expect(script).toContain("hostie list");
  });

  test("fish script includes dynamic hostname completion", async () => {
    const output: string[] = [];
    const originalLog = console.log;
    console.log = (...args: any[]) => output.push(args.join(" "));

    await completionCommand("fish");

    console.log = originalLog;

    const script = output.join("\n");
    expect(script).toContain("hostie list");
  });
});
