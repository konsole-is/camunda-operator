import Link from "next/link";
import type { ReactNode } from "react";
import { Logo } from "./logo";
import { ThemeSwitch } from "./theme-switch";
import { TopNav } from "./top-nav";

/**
 * AppShell is the frame of every page: the logo, the top navigation, and the
 * theme switch, an optional banner (the Git waiting banner), an optional side
 * navigation, and the page. A banner needs its own page gutter, because
 * AppShell adds none. A page without a side navigation is at most 1280 px
 * wide, and the CSS variable --page-gutter in it holds the width of the
 * empty space on each side. pathname goes to TopNav, for a story. AppShell
 * needs a ThemeProvider above it.
 */
export function AppShell({
  pathname,
  banner,
  sideNav,
  children,
}: {
  pathname?: string;
  banner?: ReactNode;
  sideNav?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="min-h-dvh bg-bg text-text">
      <header className="flex flex-wrap items-center gap-4 border-b border-divider bg-panel px-4 py-3.5 md:px-8">
        <Link href="/fleet" className="rounded-md no-underline">
          <Logo />
        </Link>
        <TopNav pathname={pathname} />
        <div className="ml-auto">
          <ThemeSwitch />
        </div>
      </header>
      {banner}
      {sideNav ? (
        <div className="mx-auto flex max-w-[1500px] flex-col md:flex-row md:items-start">
          {sideNav}
          <main className="min-w-0 flex-1 px-4 py-6 md:px-8 md:py-8">
            {children}
          </main>
        </div>
      ) : (
        <main className="mx-auto max-w-[1280px] px-4 py-6 [--page-gutter:max(0px,(100vw_-_1280px)/2)] md:px-8 md:py-7">
          {children}
        </main>
      )}
    </div>
  );
}
