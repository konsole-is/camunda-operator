// Keep these font imports equal to the imports of .storybook/preview.tsx.
import "@fontsource/ibm-plex-sans/400.css";
import "@fontsource/ibm-plex-sans/500.css";
import "@fontsource/ibm-plex-sans/600.css";
import "@fontsource/jetbrains-mono/400.css";
import "@fontsource/jetbrains-mono/500.css";
import "@fontsource/sora/500.css";
import "@fontsource/sora/600.css";
import "@fontsource/sora/700.css";
import type { Metadata } from "next";
import { cookies } from "next/headers";
import type { ReactNode } from "react";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import { QueryProvider } from "@/components/fleet/query-provider";
import { ThemeProvider } from "@/components/fleet/theme-switch";
import { THEME_COOKIE, themeClass, themeOf } from "@/lib/theme";
import "./globals.css";

export const metadata: Metadata = { title: "Fleet manager" };

export default async function RootLayout({
  children,
}: {
  children: ReactNode;
}) {
  const theme = themeOf((await cookies()).get(THEME_COOKIE)?.value);
  return (
    <html lang="en" className={themeClass(theme)}>
      <body>
        <ThemeProvider initial={theme}>
          <QueryProvider>
            <GitPushesProvider>{children}</GitPushesProvider>
          </QueryProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
