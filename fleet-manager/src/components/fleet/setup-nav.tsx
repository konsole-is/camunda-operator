"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";
import { SETUP_ITEMS, setupItemOf } from "./nav";

/**
 * SetupNav is the side navigation of the Setup pages. It marks the Setup page
 * of the current path, on its list page and on its detail pages. pathname
 * replaces the route path, for the gallery.
 */
export function SetupNav({ pathname }: { pathname?: string }) {
  const route = usePathname();
  const current = setupItemOf(pathname ?? route);
  return (
    <aside className="px-4 pt-3 md:sticky md:top-0 md:w-[196px] md:shrink-0 md:px-0 md:py-7 md:pl-6">
      <p className="hidden px-3 pb-2 text-xs font-medium tracking-[0.06em] text-muted uppercase md:block">
        Setup
      </p>
      <nav aria-label="Setup">
        <ul className="flex flex-wrap gap-1 md:flex-col md:gap-0.5">
          {SETUP_ITEMS.map((item) => {
            const active = item.slug === current?.slug;
            return (
              <li key={item.slug}>
                <Link
                  href={item.href}
                  aria-current={active ? "page" : undefined}
                  className={cn(
                    "flex min-h-9 items-center rounded-md border-b-2 border-transparent px-3 whitespace-nowrap text-muted no-underline hover:bg-panel hover:text-text md:border-b-0 md:border-l-2",
                    active && "border-accent bg-panel-2 text-text",
                  )}
                >
                  {item.label}
                </Link>
              </li>
            );
          })}
        </ul>
      </nav>
    </aside>
  );
}
