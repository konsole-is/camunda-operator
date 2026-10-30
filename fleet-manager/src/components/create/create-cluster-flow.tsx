"use client";

import type { V1Namespace } from "@kubernetes/client-node";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { useResources } from "@/lib/k8s/hooks";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import type { ListOptions } from "@/lib/k8s/read";
import { WizardStepper } from "@/components/fleet/wizard-stepper";
import {
  defaultAccessPatterns,
  hostConflicts,
  renderHosts,
  takenHosts,
} from "@/lib/claims/hosts";
import { claimNameErrors, namesIn } from "@/lib/claims/names";
import { ACCESS_SECTION, SECTION_LABEL } from "@/lib/labels";
import { environmentOf } from "@/lib/policy";
import { environmentCounts, presetUsage } from "@/lib/usage";
import {
  CLUSTER_STEPS,
  type ClusterDraft,
  basicsErrors,
  offeredPresets,
} from "./cluster-draft";
import { ClusterStepBasics } from "./cluster-step-basics";
import { type CreateStep, useCreateFlow } from "./use-create-flow";

/** CreateClusterData is the first snapshot of the create cluster page, from every namespace. accessRequests are the requests of the access section. */
export type CreateClusterData = {
  environments: Resource<"Environment">[];
  namespaces: Resource<"Namespace">[];
  claims: Resource<"CamundaClusterClaim">[];
  clusters: Resource<"CamundaCluster">[];
  presets: Resource<"CamundaClusterPreset">[];
  sections: Resource<"PluginSection">[];
  registrations: Resource<"PluginRegistration">[];
  accessRequests: Resource<"PluginRequest">[];
};

const EVERYWHERE = {};
const ACCESS_REQUESTS = { labelSelector: `${SECTION_LABEL}=${ACCESS_SECTION}` };

const INITIAL: ClusterDraft = { claim: { metadata: { name: "" }, spec: {} } };

/**
 * CreateClusterFlow is the create cluster wizard. It follows initial live
 * and checks step 1 against the newest data. Steps 2 to 5 hold no fields
 * yet, and the flow writes nothing.
 */
export function CreateClusterFlow({ initial }: { initial: CreateClusterData }) {
  const live = {
    environments: useLive("Environment", EVERYWHERE, initial.environments),
    namespaces: useLive("Namespace", EVERYWHERE, initial.namespaces),
    claims: useLive("CamundaClusterClaim", EVERYWHERE, initial.claims),
    clusters: useLive("CamundaCluster", EVERYWHERE, initial.clusters),
    presets: useLive("CamundaClusterPreset", EVERYWHERE, initial.presets),
    sections: useLive("PluginSection", EVERYWHERE, initial.sections),
    registrations: useLive(
      "PluginRegistration",
      EVERYWHERE,
      initial.registrations,
    ),
    accessRequests: useLive(
      "PluginRequest",
      ACCESS_REQUESTS,
      initial.accessRequests,
    ),
  };
  // A list that cannot be read would let a taken name or host, or a rule of
  // an Environment, pass unseen, so it blocks the step.
  const failures = Object.values(live).flatMap((l) =>
    l.failure ? [l.failure] : [],
  );
  const environments = models<Environment>(live.environments.items);
  const namespaces = models<V1Namespace>(live.namespaces.items);
  const claims = live.claims.items;
  const clusters = models<CamundaCluster>(live.clusters.items);
  const allPresets = models<CamundaClusterPreset>(live.presets.items);
  const sections = models<PluginSection>(live.sections.items);
  const registrations = models<PluginRegistration>(live.registrations.items);
  const accessRequests = models<PluginRequest>(live.accessRequests.items);
  const namespaceEnvironments = namespaces.map((ns) => ({
    name: ns.metadata?.name ?? "",
    environment: environmentOf(ns, environments).environment?.metadata.name,
  }));

  // Each value below reads the draft, so the step check gets it as an
  // argument and the screen gets it from the flow.
  const derive = (draft: ClusterDraft) => {
    const environment = environments.find(
      (e) => e.metadata.name === draft.environment,
    );
    const presets = offeredPresets(allPresets, environment);
    const name = draft.claim.metadata.name;
    const namespace = draft.namespace?.name ?? "";
    const takenNames =
      draft.namespace?.kind === "existing"
        ? namesIn(namespace, claims, clusters)
        : [];
    const patterns = defaultAccessPatterns(
      sections,
      registrations,
      environment,
      "CamundaClusterClaim",
    );
    const hosts =
      patterns && draft.namespace && claimNameErrors(name, []).length === 0
        ? renderHosts(patterns, name, namespace)
        : [];
    const conflicts = hostConflicts(hosts, takenHosts(accessRequests));
    return { presets, takenNames, hosts, conflicts };
  };

  const steps: CreateStep<ClusterDraft>[] = CLUSTER_STEPS.map((step) =>
    step.id === "basics"
      ? {
          ...step,
          check: (draft) => {
            const { presets, takenNames, conflicts } = derive(draft);
            return [
              ...failures,
              ...basicsErrors(draft, {
                environments,
                namespaces: namespaceEnvironments,
                takenNames,
                conflicts,
                presets: presets.map((p) => p.metadata.name ?? ""),
              }),
            ];
          },
        }
      : step,
  );
  const flow = useCreateFlow({ steps, initial: INITIAL });
  const { presets, takenNames, hosts, conflicts } = derive(flow.draft);

  return (
    <WizardStepper
      steps={steps.map((s) => ({
        id: s.id,
        label: s.label,
        errors: flow.errors[s.id],
      }))}
      current={flow.current}
      onStep={flow.go}
      onNext={flow.next}
      onBack={flow.back}
    >
      {flow.current === "basics" ? (
        <ClusterStepBasics
          draft={flow.draft}
          set={flow.set}
          environments={environments}
          counts={environmentCounts(environments, namespaces, clusters)}
          namespaces={namespaces}
          presets={presets}
          usage={presetUsage(clusters)}
          nameErrors={[
            ...claimNameErrors(flow.draft.claim.metadata.name, takenNames),
            ...conflicts,
          ]}
          hosts={hosts}
        />
      ) : (
        <p className="text-sm text-muted">
          This step comes in a later version of the fleet manager.
        </p>
      )}
    </WizardStepper>
  );
}

function useLive<K extends KindName>(
  kind: K,
  opts: ListOptions,
  initial: Resource<K>[],
): { items: Resource<K>[]; failure?: string } {
  const { items, error } = useResources(kind, opts, initial);
  return error === undefined
    ? { items }
    : {
        items,
        failure: `The fleet manager could not read the ${kind} objects: ${error}`,
      };
}

// A Resource is plain JSON, so it differs from its model type only in its
// timestamps, which neither the policy rules nor the step read.
function models<T>(items: readonly unknown[]): T[] {
  return items as T[];
}
