import type { ReactNode } from "react";
import { AppShell } from "@/components/fleet/app-shell";
import { GitBanner } from "@/components/fleet/git-banner";

export default function FleetLayout({ children }: { children: ReactNode }) {
  return <AppShell banner={<GitBanner />}>{children}</AppShell>;
}
