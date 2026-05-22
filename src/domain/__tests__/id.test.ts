import { describe, test, expect } from "bun:test";
import { generateId } from "../id";

describe("generateId", () => {
  test("generates a 26-character ULID", () => {
    const id = generateId();
    expect(id).toHaveLength(26);
  });

  test("generates unique IDs", () => {
    const id1 = generateId();
    const id2 = generateId();
    expect(id1).not.toBe(id2);
  });

  test("generates lexicographically sortable IDs", () => {
    const id1 = generateId();
    // Small delay to ensure different timestamp
    const start = Date.now();
    while (Date.now() - start < 2) {
      // Busy wait for 2ms
    }
    const id2 = generateId();
    
    // IDs generated later should sort after earlier ones
    expect(id1 < id2).toBe(true);
  });

  test("generates unique IDs within same millisecond", () => {
    // Generate multiple IDs rapidly to test monotonic behavior
    const ids = new Set<string>();
    for (let i = 0; i < 100; i++) {
      ids.add(generateId());
    }
    
    // All IDs should be unique
    expect(ids.size).toBe(100);
  });

  test("generates IDs with valid ULID characters", () => {
    const id = generateId();
    // ULID uses Crockford's Base32: 0-9 and A-Z excluding I, L, O, U
    const validPattern = /^[0123456789ABCDEFGHJKMNPQRSTVWXYZ]{26}$/;
    expect(validPattern.test(id)).toBe(true);
  });
});
