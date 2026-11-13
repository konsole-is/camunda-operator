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
import type { Preview } from "@storybook/nextjs-vite";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import { QueryProvider } from "@/components/fleet/query-provider";

// The app has one theme. The root layout sets the class dark on <html>.
document.documentElement.classList.add("dark");

const preview: Preview = {
  tags: ["autodocs"],
  parameters: {
    nextjs: { appDirectory: true },
    layout: "padded",
    // The stories check runs axe and fails only on a serious or critical
    // violation. The accessibility panel still shows every violation.
    a11y: { test: "off" },
  },
  decorators: [
    (Story) => (
      <QueryProvider>
        <GitPushesProvider>
          <Story />
        </GitPushesProvider>
      </QueryProvider>
    ),
  ],
};

export default preview;
