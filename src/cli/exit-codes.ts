/**
 * Standard exit codes for CLI commands
 * 
 * These codes allow scripts to detect and handle different error types:
 * - 0: Success
 * - 1: Validation error (invalid input, duplicate hostname, etc.)
 * - 2: I/O error (file not found, disk full, corrupted YAML, etc.)
 * - 3: Permission error (cannot write /etc/hosts, need sudo)
 */

export enum ExitCode {
  SUCCESS = 0,
  VALIDATION = 1,
  IO_ERROR = 2,
  PERMISSION = 3,
}
