import React, { useState, useEffect } from "react";
import { Box, Text, useInput } from "ink";
import type { Entry } from "../../domain/types";
import { validateIP, validateHostname } from "../../domain/validators";
import { generateId } from "../../domain/id";
import { useAppStore } from "../store";
import { writeHostsFile } from "../../core/file-io";

/**
 * EntryEditorModal component props
 */
export interface EntryEditorModalProps {
  /** Mode: add new entry or edit existing */
  mode: "add" | "edit";
  /** Entry to edit (required in edit mode) */
  entry?: Entry;
  /** Callback when form is submitted */
  onSave: (entry: Omit<Entry, "id">) => void;
  /** Callback when form is cancelled */
  onCancel: () => void;
}

/**
 * Field names for form navigation
 */
type FieldName = "ip" | "hostname" | "aliases" | "enabled" | "comment" | "submit";

/**
 * Form state
 */
interface FormState {
  ip: string;
  hostname: string;
  aliases: string;
  enabled: boolean;
  comment: string;
}

/**
 * Validation errors
 */
interface ValidationErrors {
  ip?: string;
  hostname?: string;
}

/**
 * EntryEditorModal component - modal for adding/editing entries
 * 
 * Provides a form with fields for IP, hostname, aliases, enabled checkbox, and comment.
 * Validates input on blur and shows errors inline.
 * Saves to store and ~/.hosts on submit.
 * 
 * @example
 * ```tsx
 * <EntryEditorModal
 *   mode="add"
 *   onSave={(entry) => console.log("Saved:", entry)}
 *   onCancel={() => console.log("Cancelled")}
 * />
 * ```
 */
export function EntryEditorModal({ mode, entry, onSave, onCancel }: EntryEditorModalProps) {
  // Form state
  const [formState, setFormState] = useState<FormState>({
    ip: entry?.ip || "",
    hostname: entry?.hostname || "",
    aliases: entry?.aliases.join(", ") || "",
    enabled: entry?.enabled ?? true,
    comment: entry?.comment || "",
  });

  // Current focused field
  const [focusedField, setFocusedField] = useState<FieldName>("ip");

  // Validation errors
  const [errors, setErrors] = useState<ValidationErrors>({});

  // Store actions
  const { addEntry, updateEntry, hostsFile, markDirty } = useAppStore();

  /**
   * Validate a field
   */
  const validateField = (field: FieldName, value: string): string | undefined => {
    if (field === "ip") {
      const result = validateIP(value);
      return result.valid ? undefined : result.error;
    }
    if (field === "hostname") {
      const result = validateHostname(value);
      return result.valid ? undefined : result.error;
    }
    return undefined;
  };

  /**
   * Handle field change
   */
  const handleFieldChange = (field: keyof FormState, value: string | boolean) => {
    setFormState((prev) => ({ ...prev, [field]: value }));
    
    // Clear error for this field when user types
    if (typeof value === "string" && (field === "ip" || field === "hostname")) {
      setErrors((prev) => ({ ...prev, [field]: undefined }));
    }
  };

  /**
   * Handle field blur (validation)
   */
  const handleFieldBlur = (field: FieldName) => {
    if (field === "ip" || field === "hostname") {
      const error = validateField(field, formState[field]);
      if (error) {
        setErrors((prev) => ({ ...prev, [field]: error }));
      }
    }
  };

  /**
   * Navigate to next field
   */
  const nextField = () => {
    const fields: FieldName[] = ["ip", "hostname", "aliases", "enabled", "comment", "submit"];
    const currentIndex = fields.indexOf(focusedField);
    const nextIndex = (currentIndex + 1) % fields.length;
    
    // Validate current field before moving
    handleFieldBlur(focusedField);
    
    setFocusedField(fields[nextIndex]);
  };

  /**
   * Navigate to previous field
   */
  const prevField = () => {
    const fields: FieldName[] = ["ip", "hostname", "aliases", "enabled", "comment", "submit"];
    const currentIndex = fields.indexOf(focusedField);
    const prevIndex = currentIndex === 0 ? fields.length - 1 : currentIndex - 1;
    setFocusedField(fields[prevIndex]);
  };

  /**
   * Parse aliases string into array
   */
  const parseAliases = (aliasesStr: string): string[] => {
    if (!aliasesStr.trim()) return [];
    return aliasesStr
      .split(",")
      .map((alias) => alias.trim())
      .filter((alias) => alias.length > 0);
  };

  /**
   * Handle form submission
   */
  const handleSubmit = async () => {
    // Validate all fields
    const ipError = validateField("ip", formState.ip);
    const hostnameError = validateField("hostname", formState.hostname);

    if (ipError || hostnameError) {
      setErrors({
        ip: ipError,
        hostname: hostnameError,
      });
      return;
    }

    // Parse aliases
    const aliases = parseAliases(formState.aliases);

    // Create entry data
    const entryData: Omit<Entry, "id"> = {
      ip: formState.ip,
      hostname: formState.hostname,
      aliases,
      enabled: formState.enabled,
      comment: formState.comment || undefined,
    };

    // Save to store
    if (mode === "add") {
      const newEntry: Entry = {
        ...entryData,
        id: generateId(),
      };
      addEntry(newEntry);
    } else if (mode === "edit" && entry) {
      const updatedEntry: Entry = {
        ...entryData,
        id: entry.id,
      };
      updateEntry(entry.id, updatedEntry);
    }

    // Mark as dirty
    markDirty();

    // Save to ~/.hosts
    try {
      await writeHostsFile("~/.hosts", hostsFile);
    } catch (error) {
      // Error handling - in a real app, we'd show this to the user
      console.error("Failed to write hosts file:", error);
    }

    // Call onSave callback
    onSave(entryData);
  };

  /**
   * Handle keyboard input
   */
  useInput((input, key) => {
    // Esc to cancel
    if (key.escape) {
      onCancel();
      return;
    }

    // Tab to navigate forward
    if (key.tab) {
      nextField();
      return;
    }

    // Shift+Tab to navigate backward
    if (key.shift && key.tab) {
      prevField();
      return;
    }

    // Enter to submit
    if (key.return) {
      handleSubmit();
      return;
    }

    // Space to toggle enabled checkbox
    if (input === " " && focusedField === "enabled") {
      handleFieldChange("enabled", !formState.enabled);
      return;
    }

    // Backspace
    if (key.backspace || key.delete) {
      if (focusedField === "ip") {
        handleFieldChange("ip", formState.ip.slice(0, -1));
      } else if (focusedField === "hostname") {
        handleFieldChange("hostname", formState.hostname.slice(0, -1));
      } else if (focusedField === "aliases") {
        handleFieldChange("aliases", formState.aliases.slice(0, -1));
      } else if (focusedField === "comment") {
        handleFieldChange("comment", formState.comment.slice(0, -1));
      }
      return;
    }

    // Ctrl+U to clear line
    if (key.ctrl && input === "u") {
      if (focusedField === "ip") {
        handleFieldChange("ip", "");
      } else if (focusedField === "hostname") {
        handleFieldChange("hostname", "");
      } else if (focusedField === "aliases") {
        handleFieldChange("aliases", "");
      } else if (focusedField === "comment") {
        handleFieldChange("comment", "");
      }
      return;
    }

    // Regular character input
    if (input && !key.ctrl && !key.meta) {
      if (focusedField === "ip") {
        handleFieldChange("ip", formState.ip + input);
      } else if (focusedField === "hostname") {
        handleFieldChange("hostname", formState.hostname + input);
      } else if (focusedField === "aliases") {
        handleFieldChange("aliases", formState.aliases + input);
      } else if (focusedField === "comment") {
        handleFieldChange("comment", formState.comment + input);
      }
    }
  });

  const title = mode === "add" ? "Add Entry" : "Edit Entry";

  return (
    <Box
      flexDirection="column"
      borderStyle="double"
      borderColor="cyan"
      padding={1}
      width={60}
    >
      {/* Title */}
      <Box marginBottom={1}>
        <Text bold color="cyan">
          {title}
        </Text>
      </Box>

      {/* IP Address field */}
      <Box flexDirection="column" marginBottom={1}>
        <Box>
          <Text bold={focusedField === "ip"} color={focusedField === "ip" ? "cyan" : undefined}>
            IP Address:
          </Text>
        </Box>
        <Box>
          <Text>{formState.ip || " "}</Text>
          {focusedField === "ip" && <Text color="cyan">_</Text>}
        </Box>
        {errors.ip && (
          <Box>
            <Text color="red">{errors.ip}</Text>
          </Box>
        )}
      </Box>

      {/* Hostname field */}
      <Box flexDirection="column" marginBottom={1}>
        <Box>
          <Text bold={focusedField === "hostname"} color={focusedField === "hostname" ? "cyan" : undefined}>
            Hostname:
          </Text>
        </Box>
        <Box>
          <Text>{formState.hostname || " "}</Text>
          {focusedField === "hostname" && <Text color="cyan">_</Text>}
        </Box>
        {errors.hostname && (
          <Box>
            <Text color="red">{errors.hostname}</Text>
          </Box>
        )}
      </Box>

      {/* Aliases field */}
      <Box flexDirection="column" marginBottom={1}>
        <Box>
          <Text bold={focusedField === "aliases"} color={focusedField === "aliases" ? "cyan" : undefined}>
            Aliases:
          </Text>
          <Text dimColor> (comma-separated)</Text>
        </Box>
        <Box>
          <Text>{formState.aliases || " "}</Text>
          {focusedField === "aliases" && <Text color="cyan">_</Text>}
        </Box>
      </Box>

      {/* Enabled checkbox */}
      <Box marginBottom={1}>
        <Text bold={focusedField === "enabled"} color={focusedField === "enabled" ? "cyan" : undefined}>
          Enabled:
        </Text>
        <Text> </Text>
        <Text color={formState.enabled ? "green" : "red"}>
          {formState.enabled ? "[✓]" : "[ ]"}
        </Text>
      </Box>

      {/* Comment field */}
      <Box flexDirection="column" marginBottom={1}>
        <Box>
          <Text bold={focusedField === "comment"} color={focusedField === "comment" ? "cyan" : undefined}>
            Comment:
          </Text>
        </Box>
        <Box>
          <Text>{formState.comment || " "}</Text>
          {focusedField === "comment" && <Text color="cyan">_</Text>}
        </Box>
      </Box>

      {/* Submit button */}
      <Box marginTop={1}>
        <Text bold={focusedField === "submit"} color={focusedField === "submit" ? "cyan" : "gray"}>
          [Enter] Save
        </Text>
        <Text> </Text>
        <Text dimColor>[Esc] Cancel</Text>
      </Box>
    </Box>
  );
}
