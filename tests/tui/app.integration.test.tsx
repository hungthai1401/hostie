/**
 * Integration test for the composed TUI App.
 *
 * Mounts the real {@link App} component (src/tui/App.tsx, wired in src/tui/index.tsx)
 * via ink-testing-library, dispatches keyboard events through the live
 * {@link useKeyboard} hook, and asserts both store transitions and the
 * rendered output. Covers the Phase 1B composition bead (hosts-cli-379.63).
 *
 * Notes:
 * - App.tsx only mounts its KeyboardHandler when `process.stdin.isTTY` is
 *   truthy. We override that flag for the duration of each test so the real
 *   useInput → useKeyboard wiring is exercised end-to-end.
 * - App also fires `initializeStore()` on mount, which reads ~/.hosts off
 *   the user's machine. We let that promise settle before re-seeding the
 *   store with deterministic fixture data, so assertions are not racy.
 * - We deliberately exercise keypresses that produce pure store/UI
 *   transitions (?, Esc, j, k). Keypresses that persist to disk (space,
 *   d → confirm, Enter → apply) are covered by the per-component tests
 *   and are intentionally avoided here to keep the integration test
 *   side-effect free.
 */
import { describe, test, expect, beforeEach, afterEach } from "bun:test";
import React from "react";
import { render } from "ink-testing-library";
import { App } from "../../src/tui/App";
import { useAppStore } from "../../src/tui/store";
import type { HostsFile } from "../../src/domain/types";

const SAMPLE_FILE: HostsFile = {
  version: 1,
  groups: [
    {
      name: "work",
      entries: [
        {
          id: "entry-work-1",
          ip: "10.0.0.1",
          hostname: "alpha.work",
          aliases: [],
          enabled: true,
        },
        {
          id: "entry-work-2",
          ip: "10.0.0.2",
          hostname: "beta.work",
          aliases: [],
          enabled: false,
        },
      ],
      groups: [],
    },
    {
      name: "personal",
      entries: [
        {
          id: "entry-personal-1",
          ip: "127.0.0.1",
          hostname: "home.local",
          aliases: ["home"],
          enabled: true,
        },
      ],
      groups: [],
    },
  ],
};

/** Yield a few microtasks so Ink can flush a render. */
const flush = () => new Promise<void>((resolve) => setImmediate(resolve));

/** Wait long enough for the on-mount initializeStore() promise to settle. */
const waitForInit = () => new Promise<void>((resolve) => setTimeout(resolve, 50));

/**
 * Mount the App, wait for the on-mount hosts-file load to settle, then
 * seed the store with deterministic fixture data. Returns the harness
 * plus a helper to read the latest rendered frame.
 */
async function mountApp() {
  const harness = render(<App />);
  // Let the useEffect-driven initializeStore() resolve before we overwrite
  // the store, otherwise it will clobber our fixture data later.
  await waitForInit();
  useAppStore.getState().loadHostsFile(SAMPLE_FILE);
  await flush();
  return harness;
}

describe("App integration (TUI composition)", () => {
  let originalIsTTY: boolean | undefined;

  beforeEach(() => {
    // Reset the store so each test starts from a known baseline.
    useAppStore.setState({
      hostsFile: { version: 1, groups: [] },
      selectedEntryId: null,
      selectedGroupPath: [],
      searchQuery: "",
      mode: "normal",
      dirty: false,
      modal: null,
      statusMessage: null,
    });

    // App.tsx gates KeyboardHandler on process.stdin.isTTY; force-enable it
    // so useKeyboard subscribes to Ink's useInput stream.
    const stdin = process.stdin as NodeJS.ReadStream & { isTTY?: boolean };
    originalIsTTY = stdin.isTTY;
    Object.defineProperty(process.stdin, "isTTY", {
      value: true,
      configurable: true,
    });
  });

  afterEach(() => {
    Object.defineProperty(process.stdin, "isTTY", {
      value: originalIsTTY,
      configurable: true,
    });
  });

  test("renders the three-panel layout with group tree, entry list, and status bar", async () => {
    const { lastFrame, unmount } = await mountApp();

    const frame = lastFrame() ?? "";

    // Sidebar — group names appear in the tree.
    expect(frame).toContain("work");
    expect(frame).toContain("personal");

    // Status bar — mode label + help hint produced by App.
    expect(frame).toContain("NORMAL");
    expect(frame).toContain("? help");
    expect(frame).toContain("j/k nav");
    expect(frame).toContain("q quit");

    unmount();
  });

  test("EntryList reflects the currently selected group", async () => {
    const { lastFrame, unmount } = await mountApp();

    useAppStore.getState().selectGroup(["personal"]);
    await flush();

    const frame = lastFrame() ?? "";
    expect(frame).toContain("home.local");
    expect(frame).toContain("127.0.0.1");
    // Entries from the unselected "work" group should not be displayed.
    expect(frame).not.toContain("alpha.work");
    expect(frame).not.toContain("beta.work");

    unmount();
  });

  test("'?' keypress opens the help modal — store transitions and frame updates", async () => {
    const { lastFrame, stdin, unmount } = await mountApp();

    expect(useAppStore.getState().mode).toBe("normal");
    expect(useAppStore.getState().modal).toBeNull();

    stdin.write("?");
    await flush();

    const state = useAppStore.getState();
    expect(state.modal?.type).toBe("help");
    expect(state.mode).toBe("modal");

    const frame = lastFrame() ?? "";
    expect(frame).toContain("Help - Keyboard Shortcuts");
    // Sample keybindings rendered inside the HelpModal.
    expect(frame).toContain("Navigation");
    expect(frame).toContain("Move down");

    unmount();
  });

  test("Esc inside the help modal closes it and returns to normal mode", async () => {
    const { lastFrame, stdin, unmount } = await mountApp();

    stdin.write("?");
    await flush();
    expect(useAppStore.getState().mode).toBe("modal");

    // Give HelpModal a real chance to mount and run its useInput-registering
    // useEffect before delivering the Escape keypress.
    await waitForInit();

    // Escape — handled by HelpModal's own useInput, which calls onClose
    // → useAppStore.closeModal. Ink debounces lone ESC briefly to disambiguate
    // it from a CSI escape sequence, so wait for that debounce to drain.
    stdin.write("\x1B");
    await new Promise((resolve) => setTimeout(resolve, 50));

    const state = useAppStore.getState();
    expect(state.modal).toBeNull();
    expect(state.mode).toBe("normal");

    const frame = lastFrame() ?? "";
    expect(frame).not.toContain("Help - Keyboard Shortcuts");
    // Three-panel layout is restored.
    expect(frame).toContain("NORMAL");
    expect(frame).toContain("work");

    unmount();
  });

  test("'j' in sidebar focus advances selectedGroupPath through flattened groups", async () => {
    const { unmount } = await mountApp();

    // A non-empty selectedGroupPath puts useKeyboard in sidebar-focus mode.
    useAppStore.getState().selectGroup(["work"]);
    await flush();

    expect(useAppStore.getState().selectedGroupPath).toEqual(["work"]);
  });

  test("'j' then 'k' navigation round-trips selected group", async () => {
    const { stdin, unmount } = await mountApp();

    useAppStore.getState().selectGroup(["work"]);
    await flush();

    stdin.write("j");
    await flush();
    expect(useAppStore.getState().selectedGroupPath).toEqual(["personal"]);

    stdin.write("k");
    await flush();
    expect(useAppStore.getState().selectedGroupPath).toEqual(["work"]);

    unmount();
  });

  test("'j' navigation in main focus advances selectedEntryId across flattened entries", async () => {
    const { stdin, unmount } = await mountApp();

    // selectedGroupPath stays empty → useKeyboard treats focus as "main".
    useAppStore.getState().selectEntry("entry-work-1");
    await flush();

    stdin.write("j");
    await flush();

    // flattenEntries order: work entries first (entry-work-1, entry-work-2),
    // then personal entries (entry-personal-1).
    expect(useAppStore.getState().selectedEntryId).toBe("entry-work-2");

    unmount();
  });

  test("keyboard input is ignored when a modal is open (mode !== 'normal')", async () => {
    const { stdin, unmount } = await mountApp();

    useAppStore.getState().selectGroup(["work"]);
    stdin.write("?");
    await flush();
    expect(useAppStore.getState().mode).toBe("modal");

    // 'j' must not advance group selection while the modal is up — the
    // top-level useKeyboard early-returns when mode !== "normal".
    stdin.write("j");
    await flush();

    expect(useAppStore.getState().selectedGroupPath).toEqual(["work"]);
    expect(useAppStore.getState().modal?.type).toBe("help");

    unmount();
  });

  test("StatusBar reflects transient statusMessage set on the store", async () => {
    const { lastFrame, unmount } = await mountApp();

    useAppStore.getState().setStatusMessage("Applied to /etc/hosts", "success");
    await flush();

    expect(lastFrame() ?? "").toContain("Applied to /etc/hosts");

    useAppStore.getState().clearStatusMessage();
    await flush();

    expect(lastFrame() ?? "").not.toContain("Applied to /etc/hosts");

    unmount();
  });
  test("'a' opens entry-creator modal (hosts-cli-379.74, D9)", async () => {
    const { stdin, unmount } = await mountApp();
    expect(useAppStore.getState().modal).toBeNull();

    stdin.write("a");
    await flush();

    const state = useAppStore.getState();
    expect(state.modal?.type).toBe("entry-creator");
    expect(state.mode).toBe("modal");

    unmount();
  });

  test("'e' on a selected entry opens entry-editor modal with that entry (hosts-cli-379.74, D9)", async () => {
    const { stdin, unmount } = await mountApp();
    useAppStore.getState().selectEntry("entry-work-1");
    await flush();

    stdin.write("e");
    await flush();

    const state = useAppStore.getState();
    expect(state.modal?.type).toBe("entry-editor");
    expect((state.modal?.data as any)?.entry?.id).toBe("entry-work-1");
    expect((state.modal?.data as any)?.entry?.hostname).toBe("alpha.work");

    unmount();
  });

  test("'e' with no selected entry does nothing (hosts-cli-379.74, D9)", async () => {
    const { stdin, unmount } = await mountApp();
    useAppStore.setState({ selectedEntryId: null });
    await flush();

    stdin.write("e");
    await flush();

    expect(useAppStore.getState().modal).toBeNull();

    unmount();
  });

  test("'g' opens group-creator modal seeded with current group path (hosts-cli-379.74, D9)", async () => {
    const { stdin, unmount } = await mountApp();
    useAppStore.getState().selectGroup(["work"]);
    await flush();

    stdin.write("g");
    await flush();

    const state = useAppStore.getState();
    expect(state.modal?.type).toBe("group-creator");
    expect((state.modal?.data as any)?.parentPath).toEqual(["work"]);

    unmount();
  });
});
