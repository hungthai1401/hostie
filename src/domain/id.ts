/**
 * ULID generation for unique entry identifiers
 * 
 * Uses monotonic factory to ensure uniqueness even when multiple IDs
 * are generated within the same millisecond.
 */

import { monotonicFactory } from "ulid";

/**
 * Monotonic ULID generator
 * Ensures lexicographic sortability and uniqueness within same millisecond
 */
const ulid = monotonicFactory();

/**
 * Generates a new ULID for an entry
 * 
 * @returns A 26-character ULID string
 */
export function generateId(): string {
  return ulid();
}
