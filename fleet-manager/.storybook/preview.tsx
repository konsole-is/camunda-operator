// Keep these font imports equal to the imports of src/app/layout.tsx.
import "@fontsource/ibm-plex-sans/400.css";
import "@fontsource/ibm-plex-sans/500.css";
import "@fontsource/ibm-plex-sans/600.css";
import "@fontsource/jetbrains-mono/400.css";
import "@fontsource/jetbrains-mono/500.css";
import "@fontsource/sora/500.css";
import "@fontsource/sora/600.css";
import "@fontsource/sora/700.css";
import "../src/app/globals.css";
import {
  DocsContainer,
  type DocsContainerProps,
} from "@storybook/addon-docs/blocks";
import type { Preview } from "@storybook/nextjs-vite";
import { type PropsWithChildren, useEffect, useState } from "react";
import { GLOBALS_UPDATED } from "storybook/internal/core-events";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import { QueryProvider } from "@/components/fleet/query-provider";
import { ThemeProvider } from "@/components/fleet/theme-switch";
import { applyTheme } from "@/lib/theme";
import { type StorybookTheme, storybookTheme, themes } from "./theme";

// The Docs page draws its own frame with a Storybook theme, outside the
// decorators of its stories.
function ThemedDocsContainer({
  context,
  children,
}: PropsWithChildren<DocsContainerProps>) {
  const [theme, setTheme] = useState<StorybookTheme>(() => {
    try {
      return storybookTheme(
        context.getStoryContext(context.storyById()).globals.theme,
      );
    } catch {
      // A Docs page without a stories file has no primary story.
      return "dark";
    }
  });
  useEffect(() => {
    const update = ({ globals }: { globals: Record<string, unknown> }) =>
      setTheme(storybookTheme(globals.theme));
    context.channel.on(GLOBALS_UPDATED, update);
    return () => context.channel.off(GLOBALS_UPDATED, update);
  }, [context.channel]);
  applyTheme(theme, document.documentElement);
  return (
    <DocsContainer context={context} theme={themes[theme]}>
      {children}
    </DocsContainer>
  );
}

const preview: Preview = {
  tags: ["autodocs"],
  globalTypes: {
    theme: {
      description: "The theme of the fleet manager",
      toolbar: {
        title: "Theme",
        icon: "contrast",
        items: [
          { value: "dark", title: "Dark", icon: "moon" },
          { value: "light", title: "Light", icon: "sun" },
        ],
        dynamicTitle: true,
      },
    },
  },
  initialGlobals: { theme: "dark" },
  parameters: {
    nextjs: { appDirectory: true },
    layout: "padded",
    docs: { container: ThemedDocsContainer },
    backgrounds: { disable: true },
    // The stories check runs axe and fails only on a serious or critical
    // violation. The accessibility panel still shows every violation.
    a11y: { test: "off" },
  },
  decorators: [
    (Story, { globals }) => {
      const theme = storybookTheme(globals.theme);
      // The class goes on before the story renders, so it never paints in
      // the other theme.
      applyTheme(theme, document.documentElement);
      return (
        <ThemeProvider key={theme} initial={theme}>
          <QueryProvider>
            <GitPushesProvider>
              <Story />
            </GitPushesProvider>
          </QueryProvider>
        </ThemeProvider>
      );
    },
  ],
};

export default preview;
