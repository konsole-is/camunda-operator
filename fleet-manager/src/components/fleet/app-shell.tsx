import Link from "next/link";
import type { ReactNode } from "react";
import { Logo } from "./logo";
import { TopNav } from "./top-nav";

/**
 * AppShell is the frame of every page: the logo and the top navigation, an
 * optional banner (the Git waiting banner), an optional side navigation, and
 * the page. pathname goes to TopNav, for the gallery.
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
      </header>
      {banner ? <div className="px-4 pt-4 md:px-8">{banner}</div> : null}
      {sideNav ? (
        <div className="mx-auto flex max-w-[1500px] flex-col md:flex-row md:items-start">
          {sideNav}
          <main className="min-w-0 flex-1 px-4 py-6 md:px-8 md:py-8">
            {children}
          </main>
        </div>
      ) : (
        <main className="mx-auto max-w-[1280px] px-4 py-6 md:px-8 md:py-7">
          {children}
        </main>
      )}
    </div>
  );
}
