import type { ReactNode } from "react";
import type { ConditionLike } from "@/components/fleet/status";
import { StatusChip } from "@/components/fleet/status-chip";
import type { IdentityMode, Plane } from "./plane";

/**
 * PlaneHeader names the plane and shows its Ready state, its identity mode,
 * its namespace, its platform config, and what it serves. actions holds the
 * buttons of a managed plane.
 */
export function PlaneHeader({
  name,
  namespace,
  ready,
  mode,
  platformConfig,
  serves,
  actions,
}: {
  name: string;
  namespace: string;
  ready?: ConditionLike;
  mode?: IdentityMode;
  platformConfig?: string;
  serves: Plane["serves"];
  actions?: ReactNode;
}) {
  const facts: [string, string][] = [
    ["Identity provider", mode ?? "-"],
    ["Namespace", namespace],
    ["Platform config", platformConfig || "-"],
    [
      "Serves",
      "environments" in serves
        ? serves.environments.join(", ")
        : serves.noClusterSelector
          ? "No cluster, because it has no cluster selector"
          : serves.selector,
    ],
  ];
  return (
    <header className="flex flex-col gap-4 border-b border-divider pb-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex min-w-0 flex-wrap items-center gap-3">
          <h1 className="min-w-0 font-heading text-3xl font-bold tracking-[-0.02em] break-all">
            {name}
          </h1>
          <StatusChip condition={ready} readyReason />
        </div>
        {actions && (
          <div className="flex flex-wrap items-center gap-2">{actions}</div>
        )}
      </div>
      <dl className="grid grid-cols-2 gap-x-8 gap-y-3 text-sm sm:flex sm:flex-wrap">
        {facts.map(([term, value]) => (
          <div key={term} className="flex min-w-0 flex-col gap-0.5">
            <dt className="text-xs text-muted">{term}</dt>
            <dd className="truncate text-text-2">{value}</dd>
          </div>
        ))}
      </dl>
    </header>
  );
}
