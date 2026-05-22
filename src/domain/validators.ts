/**
 * Domain validators for hostie
 * 
 * Implements validation rules for hostnames, IPs, and other domain entities.
 */

import type { Entry } from "./types";

/**
 * Validation result type
 */
export interface ValidationResult {
  valid: boolean;
  error?: string;
}

/**
 * Validates a hostname according to RFC 952 and RFC 1123.
 * 
 * Rules:
 * - Total length: 1-255 characters
 * - Each label (segment between dots): 1-63 characters
 * - Characters: a-z, A-Z, 0-9, hyphen (-), period (.)
 * - First character of each label: letter or digit (RFC 1123 relaxation)
 * - Last character of each label: letter or digit (not hyphen or period)
 * - No consecutive periods
 * - Case insensitive
 * 
 * @param hostname - The hostname to validate
 * @returns ValidationResult with valid flag and optional error message
 */
export function validateHostname(hostname: string): ValidationResult {
  // Check for empty string
  if (!hostname || hostname.length === 0) {
    return {
      valid: false,
      error: "Hostname cannot be empty",
    };
  }

  // Check total length (1-255 characters)
  if (hostname.length > 255) {
    return {
      valid: false,
      error: "Hostname exceeds maximum length of 255 characters",
    };
  }

  // Check for consecutive periods
  if (hostname.includes("..")) {
    return {
      valid: false,
      error: "Hostname cannot contain consecutive periods",
    };
  }

  // Check for leading or trailing period
  if (hostname.startsWith(".")) {
    return {
      valid: false,
      error: "Hostname cannot start with a period",
    };
  }

  if (hostname.endsWith(".")) {
    return {
      valid: false,
      error: "Hostname cannot end with a period",
    };
  }

  // Split into labels and validate each
  const labels = hostname.split(".");

  for (const label of labels) {
    // Check label length (1-63 characters)
    if (label.length === 0) {
      return {
        valid: false,
        error: "Hostname labels cannot be empty",
      };
    }

    if (label.length > 63) {
      return {
        valid: false,
        error: `Hostname label "${label}" exceeds maximum length of 63 characters`,
      };
    }

    // Check first character: must be letter or digit (RFC 1123)
    const firstChar = label[0];
    if (!/[a-zA-Z0-9]/.test(firstChar)) {
      return {
        valid: false,
        error: `Hostname label "${label}" must start with a letter or digit`,
      };
    }

    // Check last character: must be letter or digit (not hyphen)
    const lastChar = label[label.length - 1];
    if (!/[a-zA-Z0-9]/.test(lastChar)) {
      return {
        valid: false,
        error: `Hostname label "${label}" must end with a letter or digit`,
      };
    }

    // Check all characters: must be letter, digit, or hyphen
    for (let i = 0; i < label.length; i++) {
      const char = label[i];
      if (!/[a-zA-Z0-9-]/.test(char)) {
        return {
          valid: false,
          error: `Hostname label "${label}" contains invalid character "${char}"`,
        };
      }
    }
  }

  // All checks passed
  return {
    valid: true,
  };
}

/**
 * Validates an IPv4 address in dotted-decimal notation.
 * 
 * Rules:
 * - Must have exactly 4 octets separated by periods
 * - Each octet must be a number between 0 and 255
 * - No leading zeros (except for "0" itself)
 * 
 * @param ip - The IPv4 address to validate
 * @returns ValidationResult with valid flag and optional error message
 */
export function validateIPv4(ip: string): ValidationResult {
  // Check for empty string
  if (!ip || ip.length === 0) {
    return {
      valid: false,
      error: "IPv4 address cannot be empty",
    };
  }

  // Split into octets
  const octets = ip.split(".");

  // Must have exactly 4 octets
  if (octets.length !== 4) {
    return {
      valid: false,
      error: "IPv4 address must have exactly four octets",
    };
  }

  // Validate each octet
  for (let i = 0; i < octets.length; i++) {
    const octet = octets[i];

    // Check if octet is numeric
    if (!/^\d+$/.test(octet)) {
      return {
        valid: false,
        error: `Octet "${octet}" must be numeric`,
      };
    }

    // Parse as integer
    const value = parseInt(octet, 10);

    // Check range (0-255)
    if (value < 0 || value > 255) {
      return {
        valid: false,
        error: `Octet "${octet}" must be between 0 and 255`,
      };
    }

    // Check for leading zeros (except "0" itself)
    if (octet.length > 1 && octet[0] === "0") {
      return {
        valid: false,
        error: `Octet "${octet}" cannot have leading zeros`,
      };
    }
  }

  // All checks passed
  return {
    valid: true,
  };
}

/**
 * Validates an IPv6 address in standard or compressed notation.
 * 
 * Rules:
 * - Up to 8 groups of 1-4 hexadecimal digits separated by colons
 * - Can use :: to compress consecutive zero groups (only once)
 * - Case insensitive
 * 
 * @param ip - The IPv6 address to validate
 * @returns ValidationResult with valid flag and optional error message
 */
export function validateIPv6(ip: string): ValidationResult {
  // Check for empty string
  if (!ip || ip.length === 0) {
    return {
      valid: false,
      error: "IPv6 address cannot be empty",
    };
  }

  // Check for triple or more consecutive colons
  if (ip.includes(":::")) {
    return {
      valid: false,
      error: "IPv6 address cannot have three or more consecutive colons",
    };
  }

  // Check for multiple :: compressions
  const doubleColonCount = (ip.match(/::/g) || []).length;
  if (doubleColonCount > 1) {
    return {
      valid: false,
      error: "IPv6 address can only have one double-colon compression",
    };
  }

  // Handle :: compression
  let groups: string[];
  if (ip.includes("::")) {
    // Split on :: and handle each side
    const parts = ip.split("::");
    const leftGroups = parts[0] ? parts[0].split(":") : [];
    const rightGroups = parts[1] ? parts[1].split(":") : [];
    
    // Calculate how many zero groups are compressed
    const totalGroups = leftGroups.length + rightGroups.length;
    if (totalGroups > 8) {
      return {
        valid: false,
        error: "IPv6 address cannot have more than 8 groups",
      };
    }
    
    // Combine for validation (we just need to validate the non-zero groups)
    groups = [...leftGroups, ...rightGroups].filter(g => g.length > 0);
  } else {
    // No compression, must have exactly 8 groups
    groups = ip.split(":");
    if (groups.length !== 8) {
      return {
        valid: false,
        error: "IPv6 address without compression must have exactly 8 groups",
      };
    }
  }

  // Validate each group
  for (const group of groups) {
    // Each group must be 1-4 hexadecimal digits
    if (group.length === 0 || group.length > 4) {
      return {
        valid: false,
        error: `IPv6 group "${group}" must be 1-4 hexadecimal digits`,
      };
    }

    // Check if all characters are valid hex
    if (!/^[0-9a-fA-F]+$/.test(group)) {
      return {
        valid: false,
        error: `IPv6 group "${group}" must contain only hexadecimal digits`,
      };
    }
  }

  // All checks passed
  return {
    valid: true,
  };
}

/**
 * Validates an IP address (either IPv4 or IPv6).
 * 
 * @param ip - The IP address to validate
 * @returns ValidationResult with valid flag and optional error message
 */
export function validateIP(ip: string): ValidationResult {
  // Try IPv4 first
  const ipv4Result = validateIPv4(ip);
  if (ipv4Result.valid) {
    return ipv4Result;
  }

  // Try IPv6
  const ipv6Result = validateIPv6(ip);
  if (ipv6Result.valid) {
    return ipv6Result;
  }

  // Neither worked, return a generic error
  return {
    valid: false,
    error: "Invalid IP address (neither valid IPv4 nor IPv6)",
  };
}

/**
 * Validates that no two enabled entries have duplicate hostnames or aliases.
 * 
 * Rules:
 * - Only checks enabled entries (disabled entries can have duplicates)
 * - Case-insensitive comparison
 * - Checks both hostnames and aliases
 * - A hostname cannot conflict with another hostname or alias
 * 
 * @param entries - Array of entries to validate
 * @returns ValidationResult with valid flag and optional error message
 */
export function validateNoDuplicates(entries: Entry[]): ValidationResult {
  // Filter to only enabled entries
  const enabledEntries = entries.filter(entry => entry.enabled);

  // If 0 or 1 enabled entries, no duplicates possible
  if (enabledEntries.length <= 1) {
    return {
      valid: true,
    };
  }

  // Collect all hostnames and aliases (case-insensitive)
  const seenNames = new Map<string, string>(); // lowercase -> original

  for (const entry of enabledEntries) {
    // Check hostname
    const hostnameLower = entry.hostname.toLowerCase();
    if (seenNames.has(hostnameLower)) {
      return {
        valid: false,
        error: `Duplicate hostname "${hostnameLower}" found in enabled entries`,
      };
    }
    seenNames.set(hostnameLower, entry.hostname);

    // Check aliases
    for (const alias of entry.aliases) {
      const aliasLower = alias.toLowerCase();
      if (seenNames.has(aliasLower)) {
        return {
          valid: false,
          error: `Duplicate hostname/alias "${aliasLower}" found in enabled entries`,
        };
      }
      seenNames.set(aliasLower, alias);
    }
  }

  // All checks passed
  return {
    valid: true,
  };
}
