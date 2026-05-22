import { describe, test, expect } from "bun:test";
import React from "react";
import { render } from "ink-testing-library";
import { GroupCreatorModal } from "../GroupCreatorModal";

describe("GroupCreatorModal", () => {
  test("renders modal with name input field", () => {
    const onClose = () => {};
    const onSubmit = () => {};

    const { lastFrame } = render(
      <GroupCreatorModal onClose={onClose} onSubmit={onSubmit} />
    );

    expect(lastFrame()).toContain("Create Group");
    expect(lastFrame()).toContain("Name:");
  });

  test("displays parent group when provided", () => {
    const onClose = () => {};
    const onSubmit = () => {};

    const { lastFrame } = render(
      <GroupCreatorModal
        onClose={onClose}
        onSubmit={onSubmit}
        parentPath={["work", "prod"]}
      />
    );

    expect(lastFrame()).toContain("Parent: work/prod");
  });

  test("displays help text for controls", () => {
    const onClose = () => {};
    const onSubmit = () => {};

    const { lastFrame } = render(
      <GroupCreatorModal onClose={onClose} onSubmit={onSubmit} />
    );

    expect(lastFrame()).toContain("[Enter] Save");
    expect(lastFrame()).toContain("[Esc] Cancel");
  });
});
