"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { setupBreadcrumb } from "./nav";

/** SetupBreadcrumb shows "Setup / <list> / <name>" on a Setup detail page and nothing elsewhere. */
export function SetupBreadcrumb({ pathname }: { pathname?: string }) {
  const route = usePathname();
  const crumb = setupBreadcrumb(pathname ?? route);
  if (!crumb) return null;
  return (
    <nav aria-label="Breadcrumb" className="mb-4 text-sm">
      <ol className="flex flex-wrap items-center gap-x-2 text-muted">
        <li>
          <Link href="/setup" className="text-muted hover:text-text">
            Setup
          </Link>
        </li>
        <li aria-hidden>{" / "}</li>
        <li>
          <Link href={crumb.item.href} className="text-muted hover:text-text">
            {crumb.item.label}
          </Link>
        </li>
        <li aria-hidden>{" / "}</li>
        <li aria-current="page" className="min-w-0 break-all text-text">
          {crumb.name}
        </li>
      </ol>
    </nav>
  );
}
