import type { ReactNode } from "react";
import { AppShell } from "@/components/fleet/app-shell";
import { SetupBreadcrumb } from "@/components/fleet/setup-breadcrumb";
import { SetupNav } from "@/components/fleet/setup-nav";

export default function SetupLayout({ children }: { children: ReactNode }) {
  return (
    <AppShell sideNav={<SetupNav />}>
      <SetupBreadcrumb />
      {children}
    </AppShell>
  );
}
