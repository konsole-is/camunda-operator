import "@fontsource/ibm-plex-sans/400.css";
import "@fontsource/ibm-plex-sans/500.css";
import "@fontsource/ibm-plex-sans/600.css";
import "@fontsource/jetbrains-mono/400.css";
import "@fontsource/jetbrains-mono/500.css";
import "@fontsource/sora/500.css";
import "@fontsource/sora/600.css";
import "@fontsource/sora/700.css";
import type { Metadata } from "next";
import type { ReactNode } from "react";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import { QueryProvider } from "@/components/fleet/query-provider";
import "./globals.css";

export const metadata: Metadata = { title: "Fleet manager" };

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en" className="dark">
      <body>
        <QueryProvider>
          <GitPushesProvider>{children}</GitPushesProvider>
        </QueryProvider>
      </body>
    </html>
  );
}
