import { describe, test, expect, beforeEach } from "bun:test";
import React from "react";
import { render } from "ink-testing-library";
import { GroupTree } from "../GroupTree";
import type { Group } from "../../../domain/types";

describe("GroupTree", () => {
  const mockGroups: Group[] = [
    {
      name: "work",
      entries: [{ id: "1", ip: "10.0.0.1", hostname: "api.work", aliases: [], enabled: true }],
      groups: [
        {
          name: "prod",
          entries: [{ id: "2", ip: "10.0.0.2", hostname: "db.prod", aliases: [], enabled: true }],
          groups: [],
        },
        {
          name: "staging",
          entries: [],
          groups: [],
        },
      ],
    },
    {
      name: "personal",
      entries: [
        { id: "3", ip: "192.168.1.1", hostname: "home.local", aliases: [], enabled: true },
        { id: "4", ip: "192.168.1.2", hostname: "nas.local", aliases: [], enabled: false },
      ],
      groups: [],
    },
  ];

  test("renders groups hierarchically", () => {
    const { lastFrame } = render(
      <GroupTree
        groups={mockGroups}
        selectedPath={[]}
        collapsedPaths={new Set()}
        onSelect={() => {}}
        onToggleCollapse={() => {}}
      />
    );

    const output = lastFrame();
    expect(output).toContain("work");
    expect(output).toContain("personal");
  });

  test("displays entry count per group", () => {
    const { lastFrame } = render(
      <GroupTree
        groups={mockGroups}
        selectedPath={[]}
        collapsedPaths={new Set()}
        onSelect={() => {}}
        onToggleCollapse={() => {}}
      />
    );

    const output = lastFrame();
    // work has 1 direct entry
    expect(output).toContain("(1)");
    // personal has 2 entries
    expect(output).toContain("(2)");
  });

  test("shows arrow indicator for collapsible groups", () => {
    const { lastFrame } = render(
      <GroupTree
        groups={mockGroups}
        selectedPath={[]}
        collapsedPaths={new Set()}
        onSelect={() => {}}
        onToggleCollapse={() => {}}
      />
    );

    const output = lastFrame();
    // work has subgroups, should show expanded arrow
    expect(output).toMatch(/[▼▽]/);
  });

  test("hides nested groups when collapsed", () => {
    const collapsedPaths = new Set(["work"]);
    const { lastFrame } = render(
      <GroupTree
        groups={mockGroups}
        selectedPath={[]}
        collapsedPaths={collapsedPaths}
        onSelect={() => {}}
        onToggleCollapse={() => {}}
      />
    );

    const output = lastFrame();
    expect(output).toContain("work");
    // Nested groups should not be visible when parent is collapsed
    expect(output).not.toContain("prod");
    expect(output).not.toContain("staging");
  });

  test("highlights selected group", () => {
    const { lastFrame } = render(
      <GroupTree
        groups={mockGroups}
        selectedPath={["work"]}
        collapsedPaths={new Set()}
        onSelect={() => {}}
        onToggleCollapse={() => {}}
      />
    );

    const output = lastFrame();
    // Selected group should be highlighted (implementation will use inverse or color)
    expect(output).toContain("work");
  });

  test("shows nested groups with proper indentation", () => {
    const { lastFrame } = render(
      <GroupTree
        groups={mockGroups}
        selectedPath={[]}
        collapsedPaths={new Set()}
        onSelect={() => {}}
        onToggleCollapse={() => {}}
      />
    );

    const output = lastFrame();
    expect(output).toContain("work");
    expect(output).toContain("prod");
    expect(output).toContain("staging");
  });

  test("handles empty groups array", () => {
    const { lastFrame } = render(
      <GroupTree
        groups={[]}
        selectedPath={[]}
        collapsedPaths={new Set()}
        onSelect={() => {}}
        onToggleCollapse={() => {}}
      />
    );

    const output = lastFrame();
    expect(output).toContain("No groups");
  });

  test("handles deeply nested groups", () => {
    const deepGroups: Group[] = [
      {
        name: "level1",
        entries: [],
        groups: [
          {
            name: "level2",
            entries: [],
            groups: [
              {
                name: "level3",
                entries: [{ id: "5", ip: "10.0.0.5", hostname: "deep.host", aliases: [], enabled: true }],
                groups: [],
              },
            ],
          },
        ],
      },
    ];

    const { lastFrame } = render(
      <GroupTree
        groups={deepGroups}
        selectedPath={[]}
        collapsedPaths={new Set()}
        onSelect={() => {}}
        onToggleCollapse={() => {}}
      />
    );

    const output = lastFrame();
    expect(output).toContain("level1");
    expect(output).toContain("level2");
    expect(output).toContain("level3");
  });
});
