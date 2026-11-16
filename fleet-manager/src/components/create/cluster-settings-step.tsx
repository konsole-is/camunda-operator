"use client";

import { useId } from "react";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { Environment } from "@/generated/crds/environment";
import { settingsOf } from "@/components/cluster/editable-settings";
import {
  type ReleaseOption,
  releaseLabel,
} from "@/components/cluster/edit/release-options";
import { SettingsEditor } from "@/components/cluster/settings-editor";
import { settingsGroupsOf } from "@/components/cluster/settings";
import type { SecretRefs } from "@/components/fleet/secret-ref-picker";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import { type ClusterDraft, releaseOf } from "@/lib/clusters/draft";
import type { ClusterField, Step } from "@/lib/fields";
import type { KindName } from "@/lib/k8s/kinds";
import type { Merged } from "@/lib/merge";

// The selects of this step and the preset of step 1 set these.
const PICKED = new Set([
  "spec.presetRef",
  "spec.releaseRef",
  "spec.platformConfigRef",
]);

/**
 * ClusterSettingsStep is step 3 of the cluster create flow: the release,
 * the platform config, then the settings editors over fields, without the
 * fields at presetRef, releaseRef, and platformConfigRef. fields are
 * clusterFields of the draft cluster. A field that a step 2 option fills
 * shows locked with its section. unread are the kinds that could not be
 * read. It holds no draft: pick sets releaseRef or platformConfigRef, an
 * empty value unsets it, and set, reset, and invalid take steps from the
 * root of the claim.
 */
export function ClusterSettingsStep({
  draft,
  environment,
  releases,
  platformConfigs,
  fields,
  merged,
  secrets,
  pick,
  set,
  reset,
  invalid,
  unread = [],
}: {
  draft: ClusterDraft;
  environment: Environment | undefined;
  releases: readonly ReleaseOption[];
  platformConfigs: readonly string[];
  fields: readonly ClusterField[];
  merged: Merged<CamundaClusterSpec>;
  secrets: SecretRefs;
  pick: (
    field: "releaseRef" | "platformConfigRef",
    name: string | undefined,
  ) => void;
  set: (steps: Step[], value: unknown) => void;
  reset: (steps: Step[]) => void;
  invalid: (steps: Step[], message: string | undefined) => void;
  unread?: readonly KindName[];
}) {
  const id = useId();
  const release = releaseOf(draft, environment) ?? "";
  const platformDefault = environment?.spec.platformConfigRef;
  const platform = draft.claim.spec.platformConfigRef ?? platformDefault ?? "";
  const isDefault = platform !== "" && platform === platformDefault;
  const environmentName =
    environment?.spec.displayName || environment?.metadata.name;
  const claim = draft.claim.spec.cluster ?? {};

  return (
    <div className="flex min-w-0 flex-col gap-6">
      <div className="flex flex-col gap-4">
        <div className="flex max-w-md flex-col gap-1">
          <label
            htmlFor={`${id}-release`}
            className="font-heading text-base font-semibold"
          >
            Release
          </label>
          <NativeSelect
            id={`${id}-release`}
            value={release}
            onChange={(event) =>
              pick("releaseRef", event.target.value || undefined)
            }
          >
            {!releases.some((o) => o.name === release) && (
              <NativeSelectOption value={release}>
                {unread.includes("CamundaRelease")
                  ? "Could not read the releases"
                  : "Pick a release"}
              </NativeSelectOption>
            )}
            {releases.map((o) => (
              <NativeSelectOption key={o.name} value={o.name}>
                {releaseLabel(o, o.isDefault)}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>
        <div className="flex max-w-md flex-col gap-1">
          <label
            htmlFor={`${id}-platform`}
            className="font-heading text-base font-semibold"
          >
            Platform config
          </label>
          <NativeSelect
            id={`${id}-platform`}
            value={platform}
            aria-describedby={isDefault ? `${id}-platform-note` : undefined}
            onChange={(event) =>
              pick("platformConfigRef", event.target.value || undefined)
            }
          >
            {!platformConfigs.includes(platform) && (
              <NativeSelectOption value={platform}>
                {unread.includes("CamundaPlatformConfig")
                  ? "Could not read the platform configs"
                  : "Pick a platform config"}
              </NativeSelectOption>
            )}
            {platformConfigs.map((name) => (
              <NativeSelectOption key={name} value={name}>
                {name}
              </NativeSelectOption>
            ))}
          </NativeSelect>
          {isDefault && environmentName && (
            <p id={`${id}-platform-note`} className="text-sm text-muted">
              {`Default from the ${environmentName} environment.`}
            </p>
          )}
        </div>
      </div>
      {settingsGroupsOf(
        fields.filter((f) => !PICKED.has(f.claimPath ?? "")),
        merged.spec,
      ).map((group) => (
        <SettingsEditor
          key={group.component}
          group={group}
          // A new cluster has no volume yet, so no size is current.
          settings={settingsOf({ group, spec: merged.spec, live: {}, claim })}
          secrets={secrets}
          set={set}
          reset={reset}
          invalid={invalid}
        />
      ))}
    </div>
  );
}
