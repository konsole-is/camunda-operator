"use client";

import { Settings } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";
import { sectionOf } from "./nav";

const ITEM =
  "inline-flex min-h-11 items-center gap-2 rounded-md px-3 text-sm no-underline";

/**
 * TopNav holds the two parts of the app, Fleet and Setup, and marks the part
 * of the current path with aria-current="page". pathname replaces the route
 * path, for the gallery.
 */
export function TopNav({ pathname }: { pathname?: string }) {
  const route = usePathname();
  const section = sectionOf(pathname ?? route);
  const state = (active: boolean) =>
    active ? "bg-panel-2 text-text" : "text-muted hover:text-text";
  return (
    <nav aria-label="Main" className="flex flex-wrap items-center gap-1">
      <Link
        href="/fleet"
        aria-current={section === "fleet" ? "page" : undefined}
        className={cn(ITEM, state(section === "fleet"))}
      >
        Fleet
      </Link>
      <span aria-hidden className="mx-2 h-5 w-px bg-border" />
      <Link
        href="/setup"
        aria-current={section === "setup" ? "page" : undefined}
        className={cn(ITEM, state(section === "setup"))}
      >
        <Settings aria-hidden className="size-4" />
        Setup
      </Link>
    </nav>
  );
}
