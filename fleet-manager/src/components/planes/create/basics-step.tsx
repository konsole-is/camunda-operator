"use client";

import type { V1Namespace } from "@kubernetes/client-node";
import { useEffect, useId } from "react";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import { ClaimNameField } from "@/components/create/claim-name-field";
import {
  pickerEnvironment,
  syncedNamespace,
} from "@/components/create/cluster-draft";
import { EnvironmentCards } from "@/components/create/environment-cards";
import type { DraftPath } from "@/components/create/use-create-flow";
import { NamespacePicker } from "@/components/fleet/namespace-picker";
import type { SecretRefs } from "@/components/fleet/secret-ref-picker";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import { claimNameErrors } from "@/lib/claims/names";
import {
  NO_ENVIRONMENT,
  type PlaneContext,
  type PlaneDraft,
  platformConfigOf,
} from "@/lib/planes/draft";
import { environmentOf } from "@/lib/policy";
import { environmentCounts } from "@/lib/usage";
import { ExternalKeycloakFields, IdentityCards } from "./identity-cards";

/**
 * BasicsStep is step 1 of the plane create flow: the environment, the name,
 * the namespace, the platform config, and where users sign in. Without any
 * Environment it shows only that a plane needs one. A new environment
 * clears the namespace. secrets are the Secrets of the chosen namespace,
 * none for a new one.
 */
export function BasicsStep({
  draft,
  set,
  ctx,
  namespaces,
  clusters,
  secrets,
}: {
  draft: PlaneDraft;
  set: (path: DraftPath, value: unknown) => void;
  ctx: PlaneContext;
  namespaces: readonly V1Namespace[];
  clusters: readonly CamundaCluster[];
  secrets: SecretRefs;
}) {
  const id = useId();
  const environment = ctx.environments.find(
    (e) => e.metadata.name === draft.environment,
  );

  // NamespacePicker reports no change on mount, so a new namespace that was
  // saved before the rules of its environment changed would keep old labels.
  const synced = syncedNamespace(draft.namespace, environment);
  useEffect(() => {
    if (synced !== draft.namespace) set(["namespace"], synced);
  });

  if (ctx.environments.length === 0) {
    return <p className="text-sm text-muted">{NO_ENVIRONMENT}</p>;
  }

  const environmentName =
    environment && (environment.spec.displayName || environment.metadata.name);
  const configName = platformConfigOf(draft, ctx);
  const config = ctx.platformConfigs.find(
    (c) => c.metadata.name === configName,
  );
  const isDefault =
    configName !== undefined &&
    configName === environment?.spec.platformConfigRef;
  const configNames = ctx.platformConfigs
    .map((c) => c.metadata.name ?? "")
    .sort();

  const pickEnvironment = (name: string) => {
    if (name === draft.environment) return;
    set(["environment"], name);
    set(["namespace"], undefined);
  };

  return (
    <div className="flex max-w-3xl min-w-0 flex-col gap-8">
      <EnvironmentCards
        environments={ctx.environments}
        counts={environmentCounts(ctx.environments, namespaces, clusters)}
        value={draft.environment}
        onChange={pickEnvironment}
      />
      <ClaimNameField
        value={draft.name}
        errors={claimNameErrors(draft.name, ctx.takenNames)}
        hosts={[]}
        hostsLabel="The plane gets these hosts:"
        onChange={(name) => set(["name"], name)}
      />
      {environment ? (
        <NamespacePicker
          key={draft.environment ?? ""}
          namespaces={namespaces.map((ns) => ({
            name: ns.metadata?.name ?? "",
            environment: environmentOf(ns, ctx.environments).environment
              ?.metadata.name,
          }))}
          environment={pickerEnvironment(environment)}
          defaultValue={draft.namespace}
          onChange={(choice, errors) =>
            set(["namespace"], errors.length ? undefined : choice)
          }
        />
      ) : (
        <section className="flex min-w-0 flex-col gap-3">
          <h2 className="font-heading text-base font-semibold">Namespace</h2>
          <p className="text-sm text-muted">Choose an environment first.</p>
        </section>
      )}
      {environmentName && (
        <p className="border-l-2 border-accent pl-3 text-sm text-text-2">
          {draft.namespace
            ? `Policy from the ${environmentName} environment (namespace ${draft.namespace.name}).`
            : `Policy from the ${environmentName} environment.`}
        </p>
      )}
      <div className="flex max-w-md flex-col gap-1">
        <label
          htmlFor={`${id}-config`}
          className="font-heading text-base font-semibold"
        >
          Platform config
        </label>
        <NativeSelect
          id={`${id}-config`}
          value={configName ?? ""}
          aria-describedby={isDefault ? `${id}-config-note` : undefined}
          onChange={(event) =>
            set(["platformConfig"], event.target.value || undefined)
          }
        >
          {!config && (
            <NativeSelectOption value={configName ?? ""}>
              Pick a platform config
            </NativeSelectOption>
          )}
          {configNames.map((name) => (
            <NativeSelectOption key={name} value={name}>
              {name}
            </NativeSelectOption>
          ))}
        </NativeSelect>
        {isDefault && environmentName && (
          <p id={`${id}-config-note`} className="text-sm text-muted">
            {`Default from the ${environmentName} environment.`}
          </p>
        )}
      </div>
      <IdentityCards
        value={draft.mode}
        platformConfig={config}
        onChange={(mode) => set(["mode"], mode)}
      />
      {draft.mode === "externalKeycloak" && (
        <ExternalKeycloakFields
          value={draft.externalKeycloak}
          secrets={secrets}
          namespaceIsNew={draft.namespace?.kind === "new"}
          onChange={(value) => set(["externalKeycloak"], value)}
        />
      )}
    </div>
  );
}
