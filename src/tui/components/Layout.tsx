import React from "react";
import { Box, Text } from "ink";

/**
 * Layout component props
 */
interface LayoutProps {
  children: React.ReactNode;
}

/**
 * Sidebar component props
 */
interface SidebarProps {
  children: React.ReactNode;
}

/**
 * Main content area component props
 */
interface MainProps {
  children: React.ReactNode;
}

/**
 * Status bar component props
 */
interface StatusBarProps {
  children: React.ReactNode;
}

/**
 * Sidebar component - 30% width, displays group tree
 */
function Sidebar({ children }: SidebarProps) {
  return (
    <Box width="30%" flexDirection="column" borderStyle="single" borderRight>
      <Text>{children}</Text>
    </Box>
  );
}

/**
 * Main content area - 70% width, displays entry list
 */
function Main({ children }: MainProps) {
  return (
    <Box width="70%" flexDirection="column" paddingLeft={1}>
      <Text>{children}</Text>
    </Box>
  );
}

/**
 * Status bar - full width at bottom, shows mode and help hint
 */
function StatusBar({ children }: StatusBarProps) {
  return (
    <Box width="100%" borderStyle="single" borderTop paddingX={1}>
      <Text>{children}</Text>
    </Box>
  );
}

/**
 * Main layout component
 * 
 * Provides the three-panel TUI layout:
 * - Sidebar (30% width): group tree navigation
 * - Main (70% width): entry list
 * - Status bar (full width, bottom): mode and help hints
 * 
 * @example
 * ```tsx
 * <Layout>
 *   <Layout.Sidebar>Groups here</Layout.Sidebar>
 *   <Layout.Main>Entries here</Layout.Main>
 *   <Layout.StatusBar>Status info</Layout.StatusBar>
 * </Layout>
 * ```
 */
export function Layout({ children }: LayoutProps) {
  const childArray = React.Children.toArray(children);
  
  // Extract sidebar, main, and status bar from children
  const sidebar = childArray.find(
    (child) => React.isValidElement(child) && child.type === Sidebar
  );
  const main = childArray.find(
    (child) => React.isValidElement(child) && child.type === Main
  );
  const statusBar = childArray.find(
    (child) => React.isValidElement(child) && child.type === StatusBar
  );

  return (
    <Box flexDirection="column" height="100%">
      {/* Top section: sidebar and main side by side */}
      <Box flexGrow={1}>
        {sidebar}
        {main}
      </Box>
      
      {/* Bottom section: status bar */}
      {statusBar}
    </Box>
  );
}

// Attach subcomponents to Layout for compound component pattern
Layout.Sidebar = Sidebar;
Layout.Main = Main;
Layout.StatusBar = StatusBar;
