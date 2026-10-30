"use client";

import { type ReactNode, useId } from "react";
import type { FleetEnvironmentConditionType } from "@/generated/conditions";
import type { Environment } from "@/generated/crds/environment";
import { TEXT_TONE } from "@/components/fleet/status";
import type { EnvironmentCount } from "@/lib/usage";
import { byCodeUnit } from "@/lib/policy";
import { cn } from "@/lib/utils";

const READY: FleetEnvironmentConditionType = "Ready";
const HEX_COLOR = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i;

/**
 * environmentReady says whether a claim can pick environment. Only
 * Ready=False makes it not ready, with the message of the condition. A new
 * Environment has no status yet and is ready.
 */
export function environmentReady(environment: Environment): {
  ready: boolean;
  message?: string;
} {
  const condition = environment.status?.conditions?.find(
    (c) => c.type === READY,
  );
  return condition?.status === "False"
    ? { ready: false, message: condition.message }
    : { ready: true };
}

/**
 * EnvironmentCards is the environment choice of a create flow: one card for
 * each environment, by name, with its display name, its color, its counts,
 * and extra under the counts. An environment that is not ready cannot be
 * picked and shows its message. counts can lack an environment: it then
 * shows zeros.
 */
export function EnvironmentCards({
  environments,
  counts,
  value,
  onChange,
  extra,
}: {
  environments: readonly Environment[];
  counts: Readonly<Record<string, EnvironmentCount>>;
  value?: string;
  onChange?: (name: string) => void;
  extra?: (environment: Environment) => ReactNode;
}) {
  const id = useId();
  const sorted = [...environments].sort((a, b) =>
    byCodeUnit(a.metadata.name ?? "", b.metadata.name ?? ""),
  );
  return (
    <div className="flex min-w-0 flex-col gap-3">
      <h2 id={`${id}-label`} className="font-heading text-base font-semibold">
        Environment
      </h2>
      <div
        role="radiogroup"
        aria-labelledby={`${id}-label`}
        className="grid gap-3 sm:grid-cols-2"
      >
        {sorted.map((environment) => {
          const name = environment.metadata.name ?? "";
          const { ready, message } = environmentReady(environment);
          const count = counts[name] ?? { namespaces: 0, clusters: 0 };
          const color = environment.spec.color;
          const checked = value === name;
          return (
            <label
              key={name}
              className={cn(
                "flex min-w-0 gap-3 rounded-md border bg-panel p-3",
                checked ? "border-accent" : "border-border",
                ready ? "cursor-pointer" : "cursor-not-allowed",
              )}
            >
              <input
                type="radio"
                name={`${id}-environment`}
                value={name}
                checked={checked}
                disabled={!ready}
                aria-disabled={!ready || undefined}
                onChange={() => onChange?.(name)}
                className="mt-1 size-4 shrink-0 accent-[var(--accent)]"
              />
              <span className="flex min-w-0 flex-col gap-1">
                <span
                  className={cn(
                    "flex items-center gap-2 text-sm font-medium",
                    ready ? "text-text" : "text-muted",
                  )}
                >
                  {color && HEX_COLOR.test(color) && (
                    <span
                      data-swatch
                      aria-hidden
                      className="size-3 shrink-0 rounded-full ring-1 ring-border"
                      style={{ backgroundColor: color }}
                    />
                  )}
                  {environment.spec.displayName || name}
                </span>
                <span className="text-sm text-muted">
                  {`${plural(count.namespaces, "namespace")} · ${plural(count.clusters, "cluster")}`}
                </span>
                {!ready && (
                  <span className={cn("text-sm", TEXT_TONE.bad)}>
                    {message ? `Not ready: ${message}` : "Not ready"}
                  </span>
                )}
                {extra?.(environment)}
              </span>
            </label>
          );
        })}
      </div>
    </div>
  );
}

function plural(n: number, noun: string): string {
  return `${n} ${noun}${n === 1 ? "" : "s"}`;
}
