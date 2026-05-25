/**
 * Domain validator tests (tests/ tree).
 *
 * The bulk of validator coverage lives in src/domain/__tests__/validators.test.ts
 * (validateIPv4, validateIPv6, validateIP, validateNoDuplicates). This file
 * consolidates the remaining gap — validateHostname (RFC 952/1123) — and
 * re-verifies a few cross-cutting cases so `bun test tests/` exercises every
 * validator surface in isolation.
 */

import { describe, test, expect } from "bun:test";
import {
  validateHostname,
  validateIPv4,
  validateIPv6,
  validateNoDuplicates,
} from "../../src/domain/validators";
import type { Entry } from "../../src/domain/types";

describe("validateHostname", () => {
  test("accepts simple valid hostnames", () => {
    expect(validateHostname("example.com")).toEqual({ valid: true });
    expect(validateHostname("localhost")).toEqual({ valid: true });
    expect(validateHostname("sub.example.com")).toEqual({ valid: true });
    expect(validateHostname("a")).toEqual({ valid: true });
  });

  test("accepts hostnames with digits and hyphens", () => {
    expect(validateHostname("host-1.example.com")).toEqual({ valid: true });
    expect(validateHostname("123.example.com")).toEqual({ valid: true });
    expect(validateHostname("my-server-01")).toEqual({ valid: true });
  });

  test("accepts mixed-case hostnames", () => {
    expect(validateHostname("Example.COM")).toEqual({ valid: true });
  });

  test("accepts maximum-length label (63 chars)", () => {
    const label = "a".repeat(63);
    expect(validateHostname(label)).toEqual({ valid: true });
  });

  test("rejects empty hostname", () => {
    const result = validateHostname("");
    expect(result.valid).toBe(false);
    expect(result.error).toContain("empty");
  });

  test("rejects hostnames longer than 255 characters", () => {
    const longHost = "a".repeat(64) + "." + "b".repeat(64) + "." + "c".repeat(64) + "." + "d".repeat(64);
    // 64+1+64+1+64+1+64 = 259 chars
    const result = validateHostname(longHost);
    expect(result.valid).toBe(false);
    expect(result.error).toContain("255");
  });

  test("rejects labels longer than 63 characters", () => {
    const result = validateHostname("a".repeat(64) + ".com");
    expect(result.valid).toBe(false);
    expect(result.error).toContain("63");
  });

  test("rejects consecutive dots", () => {
    const result = validateHostname("foo..bar");
    expect(result.valid).toBe(false);
    expect(result.error).toBeDefined();
  });

  test("rejects leading dot", () => {
    const result = validateHostname(".example.com");
    expect(result.valid).toBe(false);
    expect(result.error).toBeDefined();
  });

  test("rejects trailing dot", () => {
    const result = validateHostname("example.com.");
    expect(result.valid).toBe(false);
    expect(result.error).toBeDefined();
  });

  test("rejects labels starting with a hyphen", () => {
    const result = validateHostname("-example.com");
    expect(result.valid).toBe(false);
    expect(result.error).toBeDefined();
  });

  test("rejects labels ending with a hyphen", () => {
    const result = validateHostname("example-.com");
    expect(result.valid).toBe(false);
    expect(result.error).toBeDefined();
  });

  test("rejects labels with illegal characters", () => {
    const result = validateHostname("exa_mple.com");
    expect(result.valid).toBe(false);
    expect(result.error).toBeDefined();

    const r2 = validateHostname("exa mple.com");
    expect(r2.valid).toBe(false);

    const r3 = validateHostname("example.c@m");
    expect(r3.valid).toBe(false);
  });
});

describe("validator surface smoke tests", () => {
  test("validateIPv4 rejects octet > 255", () => {
    const result = validateIPv4("256.0.0.1");
    expect(result.valid).toBe(false);
  });

  test("validateIPv6 accepts loopback", () => {
    expect(validateIPv6("::1")).toEqual({ valid: true });
  });

  test("validateNoDuplicates flags duplicate hostnames", () => {
    const entries: Entry[] = [
      { id: "1", ip: "127.0.0.1", hostname: "dup.example.com", aliases: [], enabled: true },
      { id: "2", ip: "127.0.0.2", hostname: "dup.example.com", aliases: [], enabled: true },
    ];
    const result = validateNoDuplicates(entries);
    expect(result.valid).toBe(false);
    expect(result.error).toContain("dup.example.com");
  });
});
