import { describe, test, expect } from "bun:test";
import React from "react";
import { render } from "ink-testing-library";
import { EntryList } from "../EntryList";
import type { Entry } from "../../../domain/types";

describe("EntryList", () => {
  const mockEntries: Entry[] = [
    {
      id: "1",
      ip: "10.0.0.1",
      hostname: "api.work",
      aliases: ["api", "api-server"],
      enabled: true,
    },
    {
      id: "2",
      ip: "192.168.1.1",
      hostname: "home.local",
      aliases: [],
      enabled: true,
    },
    {
      id: "3",
      ip: "10.0.0.5",
      hostname: "disabled.host",
      aliases: ["old"],
      enabled: false,
    },
  ];

  test("renders entries with columns", () => {
    const { lastFrame } = render(
      <EntryList
        entries={mockEntries}
        selectedId={null}
        onSelect={() => {}}
      />
    );

    const output = lastFrame();
    expect(output).toContain("api.work");
    expect(output).toContain("10.0.0.1");
    expect(output).toContain("home.local");
    expect(output).toContain("192.168.1.1");
  });

  test("displays aliases", () => {
    const { lastFrame } = render(
      <EntryList
        entries={mockEntries}
        selectedId={null}
        onSelect={() => {}}
      />
    );

    const output = lastFrame();
    expect(output).toContain("api, api-server");
  });

  test("shows enabled checkbox indicator", () => {
    const { lastFrame } = render(
      <EntryList
        entries={mockEntries}
        selectedId={null}
        onSelect={() => {}}
      />
    );

    const output = lastFrame();
    // Enabled entries should show checked indicator
    expect(output).toMatch(/[✓✔]/);
  });

  test("shows disabled checkbox indicator", () => {
    const { lastFrame } = render(
      <EntryList
        entries={mockEntries}
        selectedId={null}
        onSelect={() => {}}
      />
    );

    const output = lastFrame();
    // Disabled entry should show unchecked indicator
    expect(output).toMatch(/[✗✘]/);
  });

  test("highlights selected entry", () => {
    const { lastFrame } = render(
      <EntryList
        entries={mockEntries}
        selectedId="2"
        onSelect={() => {}}
      />
    );

    const output = lastFrame();
    expect(output).toContain("home.local");
  });

  test("displays empty state when no entries", () => {
    const { lastFrame } = render(
      <EntryList
        entries={[]}
        selectedId={null}
        onSelect={() => {}}
      />
    );

    const output = lastFrame();
    expect(output).toContain("No entries");
  });

  test("handles entries with no aliases", () => {
    const { lastFrame } = render(
      <EntryList
        entries={mockEntries}
        selectedId={null}
        onSelect={() => {}}
      />
    );

    const output = lastFrame();
    expect(output).toContain("home.local");
  });

  test("displays column headers", () => {
    const { lastFrame } = render(
      <EntryList
        entries={mockEntries}
        selectedId={null}
        onSelect={() => {}}
      />
    );

    const output = lastFrame();
    expect(output).toContain("Hostname");
    expect(output).toContain("IP");
    expect(output).toContain("Aliases");
  });
});
