/**
 * Direct unit tests for validateHostname (RFC 952 / RFC 1123).
 *
 * Covers (per bead hosts-cli-379.67 and docs/hostie/design.md):
 *   - Empty string rejected
 *   - Valid single-label and multi-label hostnames
 *   - Label length boundaries (1–63 chars per label)
 *   - Total length boundary (implementation limit: 255 chars)
 *   - Leading/trailing hyphen on a label rejected
 *   - All-numeric labels accepted (RFC 1123 relaxation)
 *   - Underscores rejected
 *   - Leading/trailing/consecutive periods rejected
 *   - IDN: ASCII Punycode (`xn--...`) accepted; raw Unicode rejected
 *   - Case-insensitive (mixed case accepted)
 *
 * IDN/Punycode decision: design.md §"Validation" pins the hostname grammar
 * to `^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.…)*$`, i.e. ASCII-only.
 * Punycode (`xn--…`) is pure ASCII and therefore accepted; raw Unicode
 * (e.g. `café.com`) is rejected by the grammar. Callers must IDN-encode
 * Unicode hostnames to punycode before validation.
 */

import { describe, test, expect } from "bun:test";
import { validateHostname } from "../../src/domain/validators";

describe("validateHostname — empty / boundary", () => {
  test("rejects empty string", () => {
    const r = validateHostname("");
    expect(r.valid).toBe(false);
    expect(r.error).toMatch(/empty/i);
  });

  test("accepts a single-letter hostname", () => {
    expect(validateHostname("a").valid).toBe(true);
  });

  test("accepts a single-digit hostname", () => {
    expect(validateHostname("7").valid).toBe(true);
  });
});

describe("validateHostname — valid hostnames", () => {
  test("accepts a simple two-label hostname", () => {
    expect(validateHostname("example.com").valid).toBe(true);
  });

  test("accepts a multi-label hostname", () => {
    expect(validateHostname("foo.bar.baz.example.com").valid).toBe(true);
  });

  test("accepts hostnames containing internal hyphens", () => {
    expect(validateHostname("db-replica.prod.work").valid).toBe(true);
    expect(validateHostname("a-b-c.d-e-f").valid).toBe(true);
  });

  test("accepts mixed-case hostnames (case-insensitive)", () => {
    expect(validateHostname("Example.COM").valid).toBe(true);
    expect(validateHostname("MyHost.Local").valid).toBe(true);
  });

  test("accepts labels starting with a digit (RFC 1123)", () => {
    expect(validateHostname("3com.example").valid).toBe(true);
    expect(validateHostname("1.2.example").valid).toBe(true);
  });

  test("accepts all-numeric labels including TLD position (RFC 1123)", () => {
    // RFC 1123 §2.1 explicitly permits labels (and hence the right-most
    // label) to begin with a digit. The hostname grammar accepts these.
    expect(validateHostname("123").valid).toBe(true);
    expect(validateHostname("host.123").valid).toBe(true);
    expect(validateHostname("1.2.3.4").valid).toBe(true);
  });
});

describe("validateHostname — label length (max 63)", () => {
  test("accepts a label that is exactly 63 characters", () => {
    const label63 = "a".repeat(63);
    expect(validateHostname(label63).valid).toBe(true);
    expect(validateHostname(`${label63}.com`).valid).toBe(true);
  });

  test("rejects a label longer than 63 characters", () => {
    const label64 = "a".repeat(64);
    const r = validateHostname(label64);
    expect(r.valid).toBe(false);
    expect(r.error).toMatch(/63 characters/);
  });

  test("rejects only the offending label in a multi-label hostname", () => {
    const label64 = "b".repeat(64);
    const r = validateHostname(`ok.${label64}.com`);
    expect(r.valid).toBe(false);
    expect(r.error).toMatch(/63 characters/);
  });
});

describe("validateHostname — total length", () => {
  test("accepts a hostname of exactly 253 characters (design.md target)", () => {
    // 11 labels of 22 chars + 10 dots = 252; add one more char = 253.
    // Build precisely: 4 labels of 63 + 3 dots = 255; trim to 253.
    // Use: "a"*61 + "." + "a"*63 + "." + "a"*63 + "." + "a"*63
    //  = 61 + 1 + 63 + 1 + 63 + 1 + 63 = 253
    const h = `${"a".repeat(61)}.${"a".repeat(63)}.${"a".repeat(63)}.${"a".repeat(63)}`;
    expect(h.length).toBe(253);
    expect(validateHostname(h).valid).toBe(true);
  });

  test("accepts a hostname of exactly 255 characters (implementation limit)", () => {
    // 4 labels of 63 + 3 dots = 255
    const h = `${"a".repeat(63)}.${"a".repeat(63)}.${"a".repeat(63)}.${"a".repeat(63)}`;
    expect(h.length).toBe(255);
    expect(validateHostname(h).valid).toBe(true);
  });

  test("rejects a hostname longer than 255 characters", () => {
    // 256 chars total: 4 labels of 63 + 3 dots + 1 extra char on last label
    // Use 4*63 + 3 dots = 255, then add to a label: 63 + . + 63 + . + 63 + . + 64 → label too long.
    // Build 256-total with all labels <=63: not possible (4*63 + 3 = 255 is the cap with 4 labels of 63).
    // Use 5 labels: 51+1+51+1+51+1+51+1+51+1 = 5*51+4 = 259 ; ok over 255.
    const h = `${"a".repeat(51)}.${"a".repeat(51)}.${"a".repeat(51)}.${"a".repeat(51)}.${"a".repeat(52)}`;
    expect(h.length).toBeGreaterThan(255);
    const r = validateHostname(h);
    expect(r.valid).toBe(false);
    expect(r.error).toMatch(/255 characters/);
  });
});

describe("validateHostname — hyphen placement", () => {
  test("rejects a label starting with a hyphen", () => {
    const r = validateHostname("-foo.com");
    expect(r.valid).toBe(false);
    expect(r.error).toMatch(/start with a letter or digit/);
  });

  test("rejects a label ending with a hyphen", () => {
    const r = validateHostname("foo-.com");
    expect(r.valid).toBe(false);
    expect(r.error).toMatch(/end with a letter or digit/);
  });

  test("rejects a non-leading label starting with a hyphen", () => {
    const r = validateHostname("ok.-bad.com");
    expect(r.valid).toBe(false);
    expect(r.error).toMatch(/start with a letter or digit/);
  });

  test("rejects a non-trailing label ending with a hyphen", () => {
    const r = validateHostname("ok.bad-.com");
    expect(r.valid).toBe(false);
    expect(r.error).toMatch(/end with a letter or digit/);
  });

  test("rejects a hostname that is a single hyphen", () => {
    expect(validateHostname("-").valid).toBe(false);
  });
});

describe("validateHostname — disallowed characters", () => {
  test("rejects underscores in labels", () => {
    const r = validateHostname("foo_bar.com");
    expect(r.valid).toBe(false);
    expect(r.error).toMatch(/invalid character "_"/);
  });

  test("rejects spaces", () => {
    const r = validateHostname("foo bar.com");
    expect(r.valid).toBe(false);
    expect(r.error).toMatch(/invalid character/);
  });

  test("rejects common punctuation (!, @, /, :, *, ?)", () => {
    for (const bad of ["foo!.com", "foo@host.com", "foo/bar", "foo:bar.com", "foo*.com", "foo?.com"]) {
      const r = validateHostname(bad);
      expect(r.valid).toBe(false);
    }
  });

  test("rejects raw Unicode hostnames (IDN must be punycode-encoded)", () => {
    // Design.md grammar is ASCII-only. Unicode like `café.com` must be
    // IDN-encoded to punycode (`xn--caf-dma.com`) by the caller first.
    const r = validateHostname("café.com");
    expect(r.valid).toBe(false);
    // Rejection may surface as "invalid character" or "must end with a
    // letter or digit" depending on which non-ASCII byte trips the check
    // first; what matters is that the hostname is rejected with an error.
    expect(r.error).toBeDefined();
  });

  test("rejects emoji hostnames", () => {
    expect(validateHostname("🎉.com").valid).toBe(false);
  });
});

describe("validateHostname — period placement", () => {
  test("rejects a hostname starting with a period", () => {
    const r = validateHostname(".example.com");
    expect(r.valid).toBe(false);
    expect(r.error).toMatch(/start with a period/);
  });

  test("rejects a hostname ending with a period", () => {
    const r = validateHostname("example.com.");
    expect(r.valid).toBe(false);
    expect(r.error).toMatch(/end with a period/);
  });

  test("rejects consecutive periods", () => {
    const r = validateHostname("foo..bar");
    expect(r.valid).toBe(false);
    expect(r.error).toMatch(/consecutive periods/);
  });
});

describe("validateHostname — IDN / punycode", () => {
  test("accepts ASCII punycode A-labels (xn--…)", () => {
    // `xn--caf-dma` is the IDNA punycode encoding of `café`.
    expect(validateHostname("xn--caf-dma.com").valid).toBe(true);
  });

  test("accepts punycode in a deeper position", () => {
    // `xn--p1ai` = `.рф` punycode TLD
    expect(validateHostname("example.xn--p1ai").valid).toBe(true);
  });

  test("accepts a fully-punycoded multi-label hostname", () => {
    expect(validateHostname("xn--80akhbyknj4f.xn--p1ai").valid).toBe(true);
  });
});
