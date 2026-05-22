/**
 * Domain validators for hostie
 * 
 * Implements validation rules for hostnames, IPs, and other domain entities.
 */

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
