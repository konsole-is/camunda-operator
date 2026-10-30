"use client";

import { useResources } from "@/lib/k8s/hooks";
import type { Resource } from "@/lib/k8s/kinds";
import { WizardStepper } from "@/components/fleet/wizard-stepper";
import {
  defaultAccessPatterns,
  hostConflicts,
  renderHosts,
  takenHosts,
} from "@/lib/claims/hosts";
import { claimNameErrors, namesIn } from "@/lib/claims/names";
import { ACCESS_SECTION, SECTION_LABEL } from "@/lib/labels";
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
  const environments = useResources(
    "Environment",
    EVERYWHERE,
    initial.environments,
  ).items;
  const namespaces = useResources(
    "Namespace",
    EVERYWHERE,
    initial.namespaces,
  ).items;
  const claims = useResources(
    "CamundaClusterClaim",
    EVERYWHERE,
    initial.claims,
  ).items;
  const clusters = useResources(
    "CamundaCluster",
    EVERYWHERE,
    initial.clusters,
  ).items;
  const allPresets = useResources(
    "CamundaClusterPreset",
    EVERYWHERE,
    initial.presets,
  ).items;
  const sections = useResources(
    "PluginSection",
    EVERYWHERE,
    initial.sections,
  ).items;
  const registrations = useResources(
    "PluginRegistration",
    EVERYWHERE,
    initial.registrations,
  ).items;
  const accessRequests = useResources(
    "PluginRequest",
    ACCESS_REQUESTS,
    initial.accessRequests,
  ).items;

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
            return basicsErrors(draft, {
              environments,
              takenNames,
              conflicts,
              presets: presets.map((p) => p.metadata.name ?? ""),
            });
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
