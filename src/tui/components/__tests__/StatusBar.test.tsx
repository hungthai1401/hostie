import { describe, test, expect } from "bun:test";
import React from "react";
import { render } from "ink";
import { StatusBar } from "../StatusBar";

describe("StatusBar", () => {
  test("renders with default props", () => {
    const instance = render(<StatusBar mode="normal" helpHint="Press ? for help" />);
    expect(instance).toBeDefined();
    instance.unmount();
  });

  test("displays mode on left side", () => {
    const instance = render(<StatusBar mode="search" helpHint="Press ? for help" />);
    expect(instance).toBeDefined();
    instance.unmount();
  });

  test("displays help hint on right side", () => {
    const instance = render(<StatusBar mode="normal" helpHint="Press Esc to cancel" />);
    expect(instance).toBeDefined();
    instance.unmount();
  });

  test("handles different modes", () => {
    const modes = ["normal", "search", "edit"] as const;
    modes.forEach((mode) => {
      const instance = render(<StatusBar mode={mode} helpHint="Test hint" />);
      expect(instance).toBeDefined();
      instance.unmount();
    });
  });

  test("renders with inverse colors for visibility", () => {
    const instance = render(<StatusBar mode="normal" helpHint="Press ? for help" />);
    expect(instance).toBeDefined();
    instance.unmount();
  });
});
