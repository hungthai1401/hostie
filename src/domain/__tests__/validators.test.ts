import { describe, test, expect } from "bun:test";
import {
  validateIPv4,
  validateIPv6,
  validateIP,
} from "../validators";

describe("validateIPv4", () => {
  test("accepts valid IPv4 addresses", () => {
    expect(validateIPv4("127.0.0.1")).toEqual({ valid: true });
    expect(validateIPv4("192.168.1.1")).toEqual({ valid: true });
    expect(validateIPv4("10.0.0.1")).toEqual({ valid: true });
    expect(validateIPv4("0.0.0.0")).toEqual({ valid: true });
    expect(validateIPv4("255.255.255.255")).toEqual({ valid: true });
  });

  test("rejects IPv4 with octets > 255", () => {
    const result = validateIPv4("256.1.1.1");
    expect(result.valid).toBe(false);
    expect(result.error).toContain("Octet");
  });

  test("rejects IPv4 with too few octets", () => {
    const result = validateIPv4("1.1.1");
    expect(result.valid).toBe(false);
    expect(result.error).toContain("four octets");
  });

  test("rejects IPv4 with too many octets", () => {
    const result = validateIPv4("1.1.1.1.1");
    expect(result.valid).toBe(false);
    expect(result.error).toContain("four octets");
  });

  test("rejects empty string", () => {
    const result = validateIPv4("");
    expect(result.valid).toBe(false);
    expect(result.error).toBeDefined();
  });

  test("rejects non-numeric octets", () => {
    const result = validateIPv4("192.168.a.1");
    expect(result.valid).toBe(false);
    expect(result.error).toContain("numeric");
  });

  test("rejects negative numbers", () => {
    const result = validateIPv4("192.168.-1.1");
    expect(result.valid).toBe(false);
    expect(result.error).toBeDefined();
  });
});

describe("validateIPv6", () => {
  test("accepts valid IPv6 addresses", () => {
    expect(validateIPv6("::1")).toEqual({ valid: true });
    expect(validateIPv6("2001:db8::1")).toEqual({ valid: true });
    expect(validateIPv6("fe80::1")).toEqual({ valid: true });
    expect(validateIPv6("2001:0db8:0000:0000:0000:0000:0000:0001")).toEqual({ valid: true });
    expect(validateIPv6("::")).toEqual({ valid: true });
    expect(validateIPv6("2001:db8:85a3::8a2e:370:7334")).toEqual({ valid: true });
  });

  test("rejects IPv6 with triple colons", () => {
    const result = validateIPv6(":::1");
    expect(result.valid).toBe(false);
    expect(result.error).toContain("consecutive colons");
  });

  test("rejects IPv6 with invalid hex characters", () => {
    const result = validateIPv6("2001:db8::xyz");
    expect(result.valid).toBe(false);
    expect(result.error).toContain("hexadecimal");
  });

  test("rejects empty string", () => {
    const result = validateIPv6("");
    expect(result.valid).toBe(false);
    expect(result.error).toBeDefined();
  });

  test("rejects too many groups", () => {
    const result = validateIPv6("1:2:3:4:5:6:7:8:9");
    expect(result.valid).toBe(false);
    expect(result.error).toContain("groups");
  });

  test("rejects group with more than 4 hex digits", () => {
    const result = validateIPv6("12345::1");
    expect(result.valid).toBe(false);
    expect(result.error).toContain("4 hexadecimal");
  });

  test("rejects multiple :: compressions", () => {
    const result = validateIPv6("2001::db8::1");
    expect(result.valid).toBe(false);
    expect(result.error).toContain("one double-colon");
  });
});

describe("validateIP", () => {
  test("accepts valid IPv4 addresses", () => {
    expect(validateIP("127.0.0.1")).toEqual({ valid: true });
    expect(validateIP("192.168.1.1")).toEqual({ valid: true });
  });

  test("accepts valid IPv6 addresses", () => {
    expect(validateIP("::1")).toEqual({ valid: true });
    expect(validateIP("2001:db8::1")).toEqual({ valid: true });
  });

  test("rejects invalid addresses", () => {
    const result = validateIP("256.1.1.1");
    expect(result.valid).toBe(false);
    expect(result.error).toBeDefined();
  });

  test("rejects empty string", () => {
    const result = validateIP("");
    expect(result.valid).toBe(false);
    expect(result.error).toBeDefined();
  });

  test("rejects non-IP strings", () => {
    const result = validateIP("not-an-ip");
    expect(result.valid).toBe(false);
    expect(result.error).toBeDefined();
  });
});
