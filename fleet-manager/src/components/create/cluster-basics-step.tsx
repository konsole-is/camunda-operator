"use client";

import type { V1Namespace } from "@kubernetes/client-node";
import { type ReactNode, useEffect } from "react";
import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import type { Environment } from "@/generated/crds/environment";
import { NamespacePicker } from "@/components/fleet/namespace-picker";
import type { RenderedHost } from "@/lib/claims/hosts";
import {
  type ClusterDraft,
  pickerEnvironment,
  syncedNamespace,
} from "@/lib/clusters/draft";
import { environmentOf } from "@/lib/policy";
import type { EnvironmentCount } from "@/lib/usage";
import { ClaimNameField } from "./claim-name-field";
import { EnvironmentCards } from "./environment-cards";
import { PresetCards } from "./preset-cards";
import type { DraftPath } from "./use-create-flow";

/**
 * ClusterBasicsStep is step 1 of the cluster create flow: the environment
 * (only while an Environment exists), the name, the namespace, and the
 * preset. presets are the presets that the picked environment offers.
 * nameErrors are the errors of the name and of its hosts. A new environment
 * clears the namespace, and the preset when the new environment does not
 * offer it. Each part restores its value from draft when the step mounts
 * again.
 */
export function ClusterBasicsStep({
  draft,
  set,
  environments,
  counts,
  namespaces,
  presets,
  usage,
  nameErrors,
  hosts,
}: {
  draft: ClusterDraft;
  set: (path: DraftPath, value: unknown) => void;
  environments: readonly Environment[];
  counts: Readonly<Record<string, EnvironmentCount>>;
  namespaces: readonly V1Namespace[];
  presets: readonly CamundaClusterPreset[];
  usage: ReadonlyMap<string, string[]>;
  nameErrors: readonly string[];
  hosts: readonly RenderedHost[];
}) {
  const environment = environments.find(
    (e) => e.metadata.name === draft.environment,
  );
  const waiting = environments.length > 0 && !environment;
  const presetRef = draft.claim.spec.presetRef;

  // NamespacePicker reports no change on mount, so a new namespace that was
  // saved before the rules of its environment changed would keep old labels.
  const synced = syncedNamespace(draft.namespace, environment);
  useEffect(() => {
    if (synced !== draft.namespace) set(["namespace"], synced);
  });

  const pickEnvironment = (name: string) => {
    if (name === draft.environment) return;
    set(["environment"], name);
    set(["namespace"], undefined);
    const next = environments.find((e) => e.metadata.name === name);
    if (
      presetRef &&
      !(next?.spec.presets?.camundaCluster ?? []).includes(presetRef)
    ) {
      set(["claim", "spec", "presetRef"], undefined);
    }
  };

  return (
    <div className="flex max-w-3xl min-w-0 flex-col gap-8">
      {environments.length > 0 && (
        <EnvironmentCards
          environments={environments}
          counts={counts}
          value={draft.environment}
          onChange={pickEnvironment}
        />
      )}
      <ClaimNameField
        value={draft.claim.metadata.name}
        errors={nameErrors}
        hosts={hosts}
        hostsLabel="The cluster gets these hosts:"
        onChange={(name) => set(["claim", "metadata", "name"], name)}
      />
      {waiting ? (
        <Part title="Namespace">
          <ChooseEnvironmentFirst />
        </Part>
      ) : (
        <NamespacePicker
          key={draft.environment ?? ""}
          namespaces={namespaces.map((ns) => ({
            name: ns.metadata?.name ?? "",
            environment: environmentOf(ns, environments).environment?.metadata
              .name,
          }))}
          environment={environment && pickerEnvironment(environment)}
          defaultValue={draft.namespace}
          onChange={(choice, errors) =>
            set(["namespace"], errors.length ? undefined : choice)
          }
        />
      )}
      <Part title="Preset">
        {waiting ? (
          <ChooseEnvironmentFirst />
        ) : environment && presets.length === 0 ? (
          <p className="text-sm text-muted">
            {`The environment ${environment.spec.displayName || environment.metadata.name} allows no cluster preset. Ask your admin to add one.`}
          </p>
        ) : (
          <PresetCards
            presets={presets}
            usage={usage}
            value={presetRef}
            onChange={(name) => set(["claim", "spec", "presetRef"], name)}
          />
        )}
      </Part>
    </div>
  );
}

function Part({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex min-w-0 flex-col gap-3">
      <h2 className="font-heading text-base font-semibold">{title}</h2>
      {children}
    </section>
  );
}

function ChooseEnvironmentFirst() {
  return <p className="text-sm text-muted">Choose an environment first.</p>;
}
