"use client";

import { type ReactNode, useId } from "react";
import type { Environment } from "@/generated/crds/environment";
import { TEXT_TONE } from "@/components/fleet/status";
import { environmentReady } from "@/lib/environments";
import type { EnvironmentCount } from "@/lib/usage";
import { byCodeUnit } from "@/lib/policy";
import { cn } from "@/lib/utils";

const HEX_COLOR = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i;

/**
 * EnvironmentCardsProps are the props of EnvironmentCards. Without multiple,
 * the cards pick one environment. With multiple, they pick any number, and
 * onChange gets the picked names in the order of the cards.
 */
export type EnvironmentCardsProps = {
  environments: readonly Environment[];
  counts: Readonly<Record<string, EnvironmentCount>>;
  extra?: (environment: Environment) => ReactNode;
} & (
  | { multiple?: false; value?: string; onChange?: (name: string) => void }
  | { multiple: true; value: string[]; onChange?: (names: string[]) => void }
);

/**
 * EnvironmentCards is the environment choice of a create flow: one card for
 * each environment, by name, with its display name, its color, its counts,
 * and extra under the counts. An environment that is not ready shows its
 * message, and in single mode it cannot be picked. counts can lack an
 * environment: it then shows zeros. In multiple mode each card names the
 * environment by its object name, with the display name next to it.
 */
export function EnvironmentCards(props: EnvironmentCardsProps) {
  const { environments, counts, extra } = props;
  const id = useId();
  const sorted = [...environments].sort((a, b) =>
    byCodeUnit(a.metadata.name ?? "", b.metadata.name ?? ""),
  );
  const isChecked = (name: string) =>
    props.multiple ? props.value.includes(name) : props.value === name;
  const pick = (name: string) => {
    if (!props.multiple) {
      props.onChange?.(name);
      return;
    }
    const picked = new Set(props.value);
    if (picked.has(name)) picked.delete(name);
    else picked.add(name);
    props.onChange?.(
      sorted.map((e) => e.metadata.name ?? "").filter((n) => picked.has(n)),
    );
  };
  return (
    <div className="flex min-w-0 flex-col gap-3">
      <h2 id={`${id}-label`} className="font-heading text-base font-semibold">
        {props.multiple ? "Environments" : "Environment"}
      </h2>
      <div
        role={props.multiple ? "group" : "radiogroup"}
        aria-labelledby={`${id}-label`}
        className="grid gap-3 sm:grid-cols-2"
      >
        {sorted.map((environment) => {
          const name = environment.metadata.name ?? "";
          const { ready, message } = environmentReady(environment);
          const count = counts[name] ?? { namespaces: 0, clusters: 0 };
          const color = environment.spec.color;
          const displayName = environment.spec.displayName;
          const checked = isChecked(name);
          // A plane serves any Environment that exists, also one that is
          // not ready, so multiple mode leaves it pickable.
          const pickable = ready || props.multiple;
          return (
            <label
              key={name}
              className={cn(
                "flex min-w-0 gap-3 rounded-md border bg-panel p-3",
                checked ? "border-accent" : "border-border",
                pickable ? "cursor-pointer" : "cursor-not-allowed",
              )}
            >
              <input
                type={props.multiple ? "checkbox" : "radio"}
                name={`${id}-environment`}
                value={name}
                checked={checked}
                disabled={!pickable}
                aria-disabled={!pickable || undefined}
                onChange={() => pick(name)}
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
                  {props.multiple ? name : displayName || name}
                  {props.multiple && displayName && displayName !== name && (
                    <span className="font-normal text-muted">
                      {displayName}
                    </span>
                  )}
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
