import Link from "next/link";
import { type ReactNode, useId } from "react";
import { CHIP_TONE, DOT_TONE } from "@/components/fleet/status";
import type { Environment } from "@/generated/crds/environment";
import { cn } from "@/lib/utils";
import {
  type ReleaseEnvironmentState,
  releaseEnvironments,
} from "./environments";

const TONE: Record<ReleaseEnvironmentState, "ok" | "muted"> = {
  Default: "ok",
  Allowed: "ok",
  "Not allowed": "muted",
};

/**
 * ReleaseEnvironments is the card that shows, for each environment, whether
 * it has release as its default, allows it, or does not allow it. Each
 * environment links to its page.
 */
export function ReleaseEnvironments({
  release,
  environments,
}: {
  release: string;
  environments: readonly Environment[];
}): ReactNode {
  const id = useId();
  const rows = releaseEnvironments(release, environments);
  return (
    <section
      aria-labelledby={id}
      className="flex flex-col gap-3 rounded-md border border-border bg-panel p-5"
    >
      <h2 id={id} className="font-heading text-base font-semibold">
        Environments
      </h2>
      {rows.length === 0 ? (
        <p className="text-sm text-muted">
          No environment exists yet.{" "}
          <Link
            href="/setup/environments"
            className="relative text-info underline-offset-2 hit-area hover:underline"
          >
            Go to the Environments page
          </Link>
        </p>
      ) : (
        <ul className="flex flex-col divide-y divide-divider">
          {rows.map((row) => (
            <li
              key={row.name}
              className="flex items-center justify-between gap-3 py-2"
            >
              <Link
                href={`/setup/environments/${encodeURIComponent(row.name)}`}
                className="relative min-w-0 text-sm font-medium break-words text-text hit-area hover:text-info"
              >
                {row.label}
              </Link>
              <span
                className={cn(
                  "inline-flex shrink-0 items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium",
                  CHIP_TONE[TONE[row.state]],
                )}
              >
                <span
                  aria-hidden
                  className={cn(
                    "size-1.5 rounded-full",
                    DOT_TONE[TONE[row.state]],
                  )}
                />
                {row.state}
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
