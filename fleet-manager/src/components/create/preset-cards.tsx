"use client";

import { useId } from "react";
import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import { cn } from "@/lib/utils";

/**
 * presetFacts returns the main facts of preset that it sets: Brokers,
 * Partitions, CPU, and Memory (the requests of a broker).
 */
export function presetFacts(
  preset: CamundaClusterPreset,
): { label: string; value: string }[] {
  const zeebe = preset.spec.cluster.zeebe;
  const requests = zeebe?.resources?.requests ?? {};
  const facts: [string, unknown][] = [
    ["Brokers", zeebe?.replicas],
    ["Partitions", zeebe?.partitions],
    ["CPU", requests.cpu],
    ["Memory", requests.memory],
  ];
  return facts
    .filter(([, value]) => value !== undefined && value !== "")
    .map(([label, value]) => ({ label, value: String(value) }));
}

/**
 * PresetCards is the preset choice of the cluster create flow: one card for
 * each preset in the given order, with its facts and the number of clusters
 * that use it (usage, by preset name).
 */
export function PresetCards({
  presets,
  usage,
  value,
  onChange,
}: {
  presets: readonly CamundaClusterPreset[];
  usage: ReadonlyMap<string, string[]>;
  value?: string;
  onChange?: (name: string) => void;
}) {
  const id = useId();
  return (
    <div role="radiogroup" aria-label="Preset" className="min-w-0">
      <div className="grid gap-3 sm:grid-cols-2">
        {presets.map((preset) => {
          const name = preset.metadata.name ?? "";
          const used = usage.get(name)?.length ?? 0;
          const checked = value === name;
          const facts = presetFacts(preset);
          return (
            <label
              key={name}
              className={cn(
                "flex min-w-0 cursor-pointer gap-3 rounded-md border bg-panel p-3",
                checked ? "border-accent" : "border-border",
              )}
            >
              <input
                type="radio"
                name={`${id}-preset`}
                value={name}
                checked={checked}
                onChange={() => onChange?.(name)}
                className="mt-1 size-4 shrink-0 accent-[var(--accent)]"
              />
              <span className="flex min-w-0 flex-1 flex-col gap-2">
                <span className="text-sm font-medium text-text">{name}</span>
                {facts.length > 0 && (
                  <dl className="grid grid-cols-2 gap-x-4 gap-y-0.5 text-sm">
                    {facts.map((fact) => (
                      <div key={fact.label} className="flex gap-2">
                        <dt className="text-muted">{fact.label}</dt>
                        <dd className="text-text tabular-nums">{fact.value}</dd>
                      </div>
                    ))}
                  </dl>
                )}
                <span className="text-xs text-muted">
                  {used === 0
                    ? "Not used yet"
                    : `Used by ${used} cluster${used === 1 ? "" : "s"}`}
                </span>
              </span>
            </label>
          );
        })}
      </div>
    </div>
  );
}
