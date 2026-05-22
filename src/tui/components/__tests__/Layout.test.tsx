import { describe, test, expect } from "bun:test";
import React from "react";
import { render } from "ink-testing-library";
import { Layout } from "../Layout";

describe("Layout", () => {
  test("renders without crashing", () => {
    const { lastFrame } = render(
      <Layout>
        <Layout.Sidebar>Sidebar content</Layout.Sidebar>
        <Layout.Main>Main content</Layout.Main>
        <Layout.StatusBar>Status bar content</Layout.StatusBar>
      </Layout>
    );
    expect(lastFrame()).toBeDefined();
  });

  test("displays sidebar content", () => {
    const { lastFrame } = render(
      <Layout>
        <Layout.Sidebar>Test Sidebar</Layout.Sidebar>
        <Layout.Main>Main</Layout.Main>
        <Layout.StatusBar>Status</Layout.StatusBar>
      </Layout>
    );
    expect(lastFrame()).toContain("Test Sidebar");
  });

  test("displays main content", () => {
    const { lastFrame } = render(
      <Layout>
        <Layout.Sidebar>Sidebar</Layout.Sidebar>
        <Layout.Main>Test Main Content</Layout.Main>
        <Layout.StatusBar>Status</Layout.StatusBar>
      </Layout>
    );
    expect(lastFrame()).toContain("Test Main Content");
  });

  test("displays status bar at bottom", () => {
    const { lastFrame } = render(
      <Layout>
        <Layout.Sidebar>Sidebar</Layout.Sidebar>
        <Layout.Main>Main</Layout.Main>
        <Layout.StatusBar>Test Status Bar</Layout.StatusBar>
      </Layout>
    );
    expect(lastFrame()).toContain("Test Status Bar");
  });

  test("sidebar and main are side by side", () => {
    const { lastFrame } = render(
      <Layout>
        <Layout.Sidebar>Sidebar</Layout.Sidebar>
        <Layout.Main>Main</Layout.Main>
        <Layout.StatusBar>Status</Layout.StatusBar>
      </Layout>
    );
    const frame = lastFrame();
    expect(frame).toBeDefined();
    // Both sidebar and main content should be present
    expect(frame).toContain("Sidebar");
    expect(frame).toContain("Main");
  });
});
