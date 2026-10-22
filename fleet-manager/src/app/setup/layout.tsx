import type { ReactNode } from "react";
import { AppShell } from "@/components/fleet/app-shell";
import { GitBanner } from "@/components/fleet/git-banner";
import { SetupBreadcrumb } from "@/components/fleet/setup-breadcrumb";
import { SetupNav } from "@/components/fleet/setup-nav";

export default function SetupLayout({ children }: { children: ReactNode }) {
  return (
    <AppShell banner={<GitBanner />} sideNav={<SetupNav />}>
      <SetupBreadcrumb />
      {children}
    </AppShell>
  );
}
