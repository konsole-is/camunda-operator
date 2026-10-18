import type { ReactNode } from "react";
import type { ConditionLike } from "@/components/fleet/status";
import { StatusChip } from "@/components/fleet/status-chip";
import { buttonVariants } from "@/components/ui/button";
import type { LaunchLink } from "./launch";

/**
 * ClusterHeaderProps are the facts of the header. environment is the display
 * name of the Environment, or the label value, and undefined when the
 * namespace has no environment label. actions is the content of the
 * cluster.header.actions slot.
 */
export type ClusterHeaderProps = {
  name: string;
  namespace: string;
  ready?: ConditionLike;
  environment?: string;
  version?: string;
  preset?: string;
  storage?: "elasticsearch" | "rdbms";
  links: LaunchLink[];
  actions?: ReactNode;
};

const STORAGE_LABEL = {
  elasticsearch: "Elasticsearch",
  rdbms: "Relational database",
} as const;

/** ClusterHeader names the cluster, shows its Ready state and facts, and links to its web apps. */
export function ClusterHeader({
  name,
  namespace,
  ready,
  environment,
  version,
  preset,
  storage,
  links,
  actions,
}: ClusterHeaderProps) {
  const facts: [string, string][] = [
    ["Environment", environment ?? "No environment"],
    ["Namespace", namespace],
    ["Version", version ?? "-"],
    ["Preset", preset ?? "-"],
    ["Secondary storage", storage ? STORAGE_LABEL[storage] : "-"],
  ];
  return (
    <header className="flex flex-col gap-4 border-b border-divider pb-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex min-w-0 flex-wrap items-center gap-3">
          <h1 className="min-w-0 font-heading text-3xl font-bold tracking-[-0.02em] break-all">
            {name}
          </h1>
          <StatusChip condition={ready} />
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {links.map((link) => (
            <a
              key={link.label}
              href={link.href}
              target="_blank"
              rel="noreferrer"
              className={buttonVariants({ variant: "outline", size: "sm" })}
            >
              {link.label}
            </a>
          ))}
          {actions}
        </div>
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
