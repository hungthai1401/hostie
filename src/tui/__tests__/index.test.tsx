import { describe, test, expect } from "bun:test";
import { renderTUI } from "../index";
import React from "react";
import { render } from "ink";
import { Text } from "ink";
import { create } from "zustand";

describe("TUI", () => {
  test("renderTUI function exists and is callable", () => {
    expect(typeof renderTUI).toBe("function");
  });

  test("renderTUI returns an Ink instance with cleanup", () => {
    const instance = renderTUI();
    expect(instance).toBeDefined();
    expect(typeof instance.unmount).toBe("function");
    expect(typeof instance.waitUntilExit).toBe("function");
    instance.unmount();
  });

  test("React hooks work in TUI context", () => {
    // Test that React hooks can be used
    const TestComponent = () => {
      const [count, setCount] = React.useState(0);
      React.useEffect(() => {
        setCount(1);
      }, []);
      return <Text>Count: {count}</Text>;
    };

    const instance = render(<TestComponent />);
    expect(instance).toBeDefined();
    instance.unmount();
  });

  test("Zustand store can be created", () => {
    // Test that Zustand works
    type Store = {
      count: number;
      increment: () => void;
    };

    const useStore = create<Store>((set) => ({
      count: 0,
      increment: () => set((state) => ({ count: state.count + 1 })),
    }));

    const store = useStore.getState();
    expect(store.count).toBe(0);
    store.increment();
    expect(useStore.getState().count).toBe(1);
  });
});
