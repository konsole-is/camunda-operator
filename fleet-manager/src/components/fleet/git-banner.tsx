"use client";
import { useEffect, useState, type ReactNode } from "react";
import { useGitPushes } from "./git-pushes";
import { GitWaitingLine } from "./git-waiting-line";

const NOW_INTERVAL_MS = 30_000;

/**
 * GitBannerView says that pushed changes wait for GitOps, with children
 * below. It renders nothing when count is 0. It brings its own page
 * gutter, so the slot banner of AppShell adds no space when it is empty.
 */
export function GitBannerView({
  children,
  count,
}: {
  children: ReactNode;
  count: number;
}) {
  if (count === 0) return null;
  return (
    <div className="px-4 pt-4 md:px-8">
      <div
        role="status"
        className="flex flex-col gap-1.5 rounded-lg border border-border bg-panel px-3.5 py-2.5 text-text-2"
      >
        <div className="flex items-center gap-2.5">
          <span
            aria-hidden
            className="size-2 rounded-full bg-info shadow-[0_0_8px_var(--info)]"
          />
          Waiting for GitOps to apply
        </div>
        {children}
      </div>
    </div>
  );
}

/**
 * GitBanner shows each pending push of useGitPushes. GitPushesProvider
 * removes a push when GitOps applied it. It needs a GitPushesProvider above
 * it.
 */
export function GitBanner() {
  const { pushes } = useGitPushes();
  const [now, setNow] = useState(() => new Date().toISOString());
  useEffect(() => {
    const timer = setInterval(
      () => setNow(new Date().toISOString()),
      NOW_INTERVAL_MS,
    );
    return () => clearInterval(timer);
  }, []);
  return (
    <GitBannerView count={pushes.length}>
      {pushes.map((push) => (
        <GitWaitingLine key={push.key} pushed={push} now={now} />
      ))}
    </GitBannerView>
  );
}
