import type { ReactNode } from "react";
import { AppShell } from "@/components/fleet/app-shell";

export default function FleetLayout({ children }: { children: ReactNode }) {
  return <AppShell>{children}</AppShell>;
}
