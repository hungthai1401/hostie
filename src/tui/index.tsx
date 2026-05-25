import React from "react";
import { render, type Instance } from "ink";
import { App } from "./App";

/**
 * Renders the Hostie TUI application.
 *
 * Mounts the {@link App} component, which:
 * - loads the user's hosts file via {@link initializeStore} on mount,
 * - composes the three-panel Layout (group tree, entry list, status bar),
 * - registers keyboard handling (when stdin supports raw mode),
 * - hosts modals (add / edit / delete / move-to-group / help / etc).
 *
 * @returns Ink {@link Instance} exposing `unmount()` and `waitUntilExit()`.
 */
export function renderTUI(): Instance {
  return render(<App />);
}

export { App };
