import React from "react";
import { render } from "ink-testing-library";
import { EntryEditorModal } from "../EntryEditorModal";
import type { Entry } from "../../../domain/types";

describe("EntryEditorModal", () => {
  const mockEntry: Entry = {
    id: "test-id-123",
    ip: "192.168.1.10",
    hostname: "test.local",
    aliases: ["alias1", "alias2"],
    enabled: true,
    comment: "Test comment",
  };

  describe("Add mode", () => {
    it("renders empty form for adding new entry", () => {
      const onSave = jest.fn();
      const onCancel = jest.fn();

      const { lastFrame } = render(
        <EntryEditorModal mode="add" onSave={onSave} onCancel={onCancel} />
      );

      expect(lastFrame()).toContain("Add Entry");
      expect(lastFrame()).toContain("IP Address:");
      expect(lastFrame()).toContain("Hostname:");
      expect(lastFrame()).toContain("Aliases:");
      expect(lastFrame()).toContain("Enabled:");
      expect(lastFrame()).toContain("Comment:");
    });
  });

  describe("Edit mode", () => {
    it("renders form pre-filled with entry data", () => {
      const onSave = jest.fn();
      const onCancel = jest.fn();

      const { lastFrame } = render(
        <EntryEditorModal
          mode="edit"
          entry={mockEntry}
          onSave={onSave}
          onCancel={onCancel}
        />
      );

      expect(lastFrame()).toContain("Edit Entry");
      expect(lastFrame()).toContain("192.168.1.10");
      expect(lastFrame()).toContain("test.local");
      expect(lastFrame()).toContain("alias1, alias2");
      expect(lastFrame()).toContain("Test comment");
    });
  });

  describe("Rendering", () => {
    it("shows enabled checkbox as checked when enabled is true", () => {
      const onSave = jest.fn();
      const onCancel = jest.fn();

      const { lastFrame } = render(
        <EntryEditorModal
          mode="edit"
          entry={mockEntry}
          onSave={onSave}
          onCancel={onCancel}
        />
      );

      expect(lastFrame()).toContain("[✓]");
    });

    it("shows enabled checkbox as unchecked when enabled is false", () => {
      const onSave = jest.fn();
      const onCancel = jest.fn();

      const disabledEntry = { ...mockEntry, enabled: false };

      const { lastFrame } = render(
        <EntryEditorModal
          mode="edit"
          entry={disabledEntry}
          onSave={onSave}
          onCancel={onCancel}
        />
      );

      expect(lastFrame()).toContain("[ ]");
    });

    it("displays help text for aliases field", () => {
      const onSave = jest.fn();
      const onCancel = jest.fn();

      const { lastFrame } = render(
        <EntryEditorModal mode="add" onSave={onSave} onCancel={onCancel} />
      );

      expect(lastFrame()).toContain("comma-separated");
    });

    it("shows save and cancel instructions", () => {
      const onSave = jest.fn();
      const onCancel = jest.fn();

      const { lastFrame } = render(
        <EntryEditorModal mode="add" onSave={onSave} onCancel={onCancel} />
      );

      expect(lastFrame()).toContain("Save");
      expect(lastFrame()).toContain("Cancel");
    });
  });
});
