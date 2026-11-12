"use client";

import { type ReactNode, useId } from "react";
import { LockedField } from "@/components/fleet/locked-field";
import { Button } from "@/components/ui/button";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import { offeredPresets } from "@/lib/clusters/draft";
import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import type { Environment } from "@/generated/crds/environment";
import { lockLabel, pathFrom, type Step } from "@/lib/fields";
import { claimAfter, fieldLock } from "@/lib/session";
import type { ClusterSlotContext } from "../slots";
import { useClusterEdit } from "./cluster-edit";
import { downgradeDone } from "./downgrade";
import { type ReleaseOption, releaseChoices } from "./release-options";

const RELEASE: Step[] = [{ field: "spec" }, { field: "releaseRef" }];
const PRESET: Step[] = [{ field: "spec" }, { field: "presetRef" }];
const PLATFORM: Step[] = [{ field: "spec" }, { field: "platformConfigRef" }];
const ALLOW: Step[] = [{ field: "spec" }, { field: "allowVersionDowngrade" }];

/**
 * ReleasePanel edits the release, the preset, and the platform config of
 * the claim. The release select offers releaseChoices. A downgrade sets
 * allowVersionDowngrade to its target too, and another release sets it
 * back to the value of the claim. The preset select offers the presets
 * that the Environment allows and the current one. The empty platform
 * config option removes spec.platformConfigRef. Once the cluster runs the
 * version that allowVersionDowngrade names and nothing is pending, Clear
 * removes the field and opens the review. A field error of a select shows
 * under it. It renders nothing without a session.
 */
export function ReleasePanel({ context }: { context: ClusterSlotContext }) {
  const edit = useClusterEdit();
  const { session } = edit;
  if (!session) return null;
  const live = session.claim.spec;
  const after = claimAfter(session).spec ?? {};
  const errorsAt = (steps: Step[]) => {
    const path = pathFrom("", steps);
    return edit.errors.filter((e) => e.path === path).map((e) => e.message);
  };
  const lockOf = (steps: Step[]) => fieldLock(session, steps);
  // A ref that the claim leaves empty shows the value of the cluster.
  const choose = (steps: Step[], value: string, fallback?: string) => {
    if (!valueOf(live, steps) && (value === "" || value === fallback))
      return edit.reset(steps);
    return value === "" ? edit.reset(steps) : edit.set(steps, value);
  };

  const { options, note } = releaseChoices({
    cluster: context.cluster,
    claim: session.claim,
    releases: context.releases,
    environment: context.environment,
    backups: context.backups,
  });
  const release = after.releaseRef || context.cluster.spec.releaseRef || "";
  const picked = options.find((o) => o.name === release);
  const pickRelease = (name: string) => {
    choose(RELEASE, name, context.cluster.spec.releaseRef);
    const target = options.find((o) => o.name === name)?.downgrade?.to;
    if (target) edit.set(ALLOW, target);
    else if (live?.allowVersionDowngrade === undefined) edit.reset(ALLOW);
    else edit.set(ALLOW, live.allowVersionDowngrade);
  };

  const presets = presetNames(context, live?.presetRef);
  const preset = after.presetRef ?? "";
  const platformDefault = context.environment?.spec.platformConfigRef;
  const platform = after.platformConfigRef ?? "";
  const clear =
    edit.pending === 0 && downgradeDone(context.cluster, session.claim);

  return (
    <div className="flex flex-col gap-5">
      <Choice
        label="Release"
        value={release}
        lock={lockOf(RELEASE)}
        errors={errorsAt(RELEASE)}
        onChange={pickRelease}
        hint={
          <>
            {picked?.downgrade && (
              <p className="text-xs text-warn">
                {picked.downgrade.from} to {picked.downgrade.to} is a downgrade.
                The review asks you to confirm it.
              </p>
            )}
            {note && <p className="text-xs text-muted">{note}</p>}
          </>
        }
      >
        {options.map((o) => (
          <NativeSelectOption
            key={o.name}
            value={o.name}
            disabled={o.disabled !== undefined && !o.isCurrent}
          >
            {releaseLabel(o)}
          </NativeSelectOption>
        ))}
      </Choice>
      {clear && (
        <div className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-md border border-border bg-panel-2 px-3 py-2 text-sm text-text-2">
          <p className="min-w-0 flex-1">
            The cluster runs {context.cluster.status?.management?.version}, so
            allowVersionDowngrade can go.
          </p>
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              edit.reset(ALLOW);
              edit.review();
            }}
          >
            Clear
          </Button>
        </div>
      )}
      <Choice
        label="Preset"
        value={preset}
        lock={lockOf(PRESET)}
        errors={errorsAt(PRESET)}
        onChange={(name) => choose(PRESET, name)}
      >
        {presets.map((name) => (
          <NativeSelectOption key={name} value={name}>
            {name}
          </NativeSelectOption>
        ))}
      </Choice>
      <Choice
        label="Platform config"
        value={platform}
        lock={lockOf(PLATFORM)}
        errors={errorsAt(PLATFORM)}
        onChange={(name) => choose(PLATFORM, name)}
      >
        <NativeSelectOption value="">
          {platformDefault ? "Default of the environment" : "Not set"}
        </NativeSelectOption>
        {context.platformConfigs.map((p) => (
          <NativeSelectOption key={p.metadata?.name} value={p.metadata?.name}>
            {p.metadata?.name}
          </NativeSelectOption>
        ))}
      </Choice>
    </div>
  );
}

function releaseLabel(o: ReleaseOption): string {
  const label = `${o.version} (${o.name})`;
  if (o.disabled) return `${label}: ${o.disabled}`;
  const marks = [o.isDefault && "Default", o.downgrade && "downgrade"].filter(
    Boolean,
  );
  return marks.length > 0 ? `${label}, ${marks.join(", ")}` : label;
}

// presetNames returns the names of the presets that the Environment
// allows, then the current preset when the Environment no longer allows it.
function presetNames(
  context: ClusterSlotContext,
  current: string | undefined,
): string[] {
  // The cache objects differ from the generated types only in their
  // timestamps, which offeredPresets does not read.
  const names = offeredPresets(
    context.presets as unknown as CamundaClusterPreset[],
    context.environment as unknown as Environment | undefined,
  ).map((p) => p.metadata.name ?? "");
  return current && !names.includes(current) ? [...names, current] : names;
}

function valueOf(spec: object | undefined, steps: Step[]): unknown {
  const field = steps[1];
  return spec && "field" in field
    ? (spec as Record<string, unknown>)[field.field]
    : undefined;
}

function Choice({
  label,
  value,
  lock,
  errors,
  hint,
  onChange,
  children,
}: {
  label: string;
  value: string;
  lock: ReturnType<typeof fieldLock>;
  errors: string[];
  hint?: ReactNode;
  onChange: (value: string) => void;
  children: ReactNode;
}) {
  const id = useId();
  if (lock)
    return (
      <LockedField
        label={label}
        value={value || "Not set"}
        reason={lockLabel(lock)}
      />
    );
  return (
    <div className="flex max-w-md flex-col gap-1.5">
      <label htmlFor={id} className="text-sm font-medium text-text">
        {label}
      </label>
      <NativeSelect
        id={id}
        value={value}
        aria-invalid={errors.length > 0 ? true : undefined}
        aria-describedby={errors.length > 0 ? `${id}-error` : undefined}
        onChange={(event) => onChange(event.target.value)}
      >
        {children}
      </NativeSelect>
      {hint}
      {errors.length > 0 && (
        <p id={`${id}-error`} className="text-xs text-bad">
          {errors.join(" ")}
        </p>
      )}
    </div>
  );
}
