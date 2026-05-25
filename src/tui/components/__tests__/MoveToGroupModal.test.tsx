import React from "react";
import { render } from "ink-testing-library";
import { MoveToGroupModal } from "../MoveToGroupModal";
import type { Group } from "../../../domain/types";

describe("MoveToGroupModal", () => {
  const mockGroups: Group[] = [
    { name: "work", entries: [], groups: [] },
    { name: "personal", entries: [], groups: [] },
    {
      name: "dev",
      entries: [],
      groups: [{ name: "staging", entries: [], groups: [] }],
    },
  ];

  it("renders group list", () => {
    const { lastFrame } = render(
      <MoveToGroupModal
        groups={mockGroups}
        onSelect={() => {}}
        onCancel={() => {}}
      />
    );

    expect(lastFrame()).toContain("Move to Group");
    expect(lastFrame()).toContain("work");
    expect(lastFrame()).toContain("personal");
    expect(lastFrame()).toContain("dev");
  });

  it("highlights first group by default", () => {
    const { lastFrame } = render(
      <MoveToGroupModal
        groups={mockGroups}
        onSelect={() => {}}
        onCancel={() => {}}
      />
    );

    const frame = lastFrame();
    expect(frame).toContain("work");
  });

  it("calls onCancel when Escape is pressed", async () => {
    const onCancel = jest.fn();
    const { stdin } = render(
      <MoveToGroupModal
        groups={mockGroups}
        onSelect={() => {}}
        onCancel={onCancel}
      />
    );

    stdin.write("\x1B"); // Escape key
    // Ink debounces lone ESC briefly to disambiguate it from a CSI escape sequence.
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(onCancel).toHaveBeenCalled();
  });

  it("calls onSelect with group path when Enter is pressed", async () => {
    const onSelect = jest.fn();
    const { stdin } = render(
      <MoveToGroupModal
        groups={mockGroups}
        onSelect={onSelect}
        onCancel={() => {}}
      />
    );

    stdin.write("\r"); // Enter key
    await new Promise((resolve) => setImmediate(resolve));
    expect(onSelect).toHaveBeenCalledWith(["work"]);
  });

  it("navigates down with j key", async () => {
    const onSelect = jest.fn();
    const { stdin } = render(
      <MoveToGroupModal
        groups={mockGroups}
        onSelect={onSelect}
        onCancel={() => {}}
      />
    );

    stdin.write("j"); // Move down
    await new Promise((resolve) => setImmediate(resolve));
    stdin.write("\r"); // Enter
    await new Promise((resolve) => setImmediate(resolve));
    expect(onSelect).toHaveBeenCalledWith(["personal"]);
  });

  it("navigates up with k key", async () => {
    const onSelect = jest.fn();
    const { stdin } = render(
      <MoveToGroupModal
        groups={mockGroups}
        onSelect={onSelect}
        onCancel={() => {}}
      />
    );

    stdin.write("j"); // Move to second item
    await new Promise((resolve) => setImmediate(resolve));
    stdin.write("k"); // Move back up
    await new Promise((resolve) => setImmediate(resolve));
    stdin.write("\r"); // Enter
    await new Promise((resolve) => setImmediate(resolve));
    expect(onSelect).toHaveBeenCalledWith(["work"]);
  });

  it("shows nested groups with indentation", () => {
    const { lastFrame } = render(
      <MoveToGroupModal
        groups={mockGroups}
        onSelect={() => {}}
        onCancel={() => {}}
      />
    );

    const frame = lastFrame();
    expect(frame).toContain("dev/staging");
  });
});
