"use client";

import type { V1Namespace } from "@kubernetes/client-node";
import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import type { Environment } from "@/generated/crds/environment";
import { WizardStepper } from "@/components/fleet/wizard-stepper";
import type { RenderedHost } from "@/lib/claims/hosts";
import { claimNameErrors } from "@/lib/claims/names";
import { environmentOf } from "@/lib/policy";
import { environmentCounts } from "@/lib/usage";
import {
  CLUSTER_STEPS,
  type ClusterDraft,
  basicsErrors,
  offeredPresets,
} from "./cluster-draft";
import { ClusterStepBasics } from "./cluster-step-basics";
import { useCreateFlow } from "./use-create-flow";

/**
 * BasicsStory shows step 1 of the cluster create flow in the gallery. A
 * story passes no event handler, so this wrapper owns the draft and takes
 * only data. usage is by preset name.
 */
export function BasicsStory({
  environments,
  namespaces,
  presets,
  usage = {},
  initial,
  hosts = [],
  conflicts = [],
}: {
  environments: Environment[];
  namespaces: V1Namespace[];
  presets: CamundaClusterPreset[];
  usage?: Record<string, string[]>;
  initial: ClusterDraft;
  hosts?: RenderedHost[];
  conflicts?: string[];
}) {
  const offered = (draft: ClusterDraft) =>
    offeredPresets(
      presets,
      environments.find((e) => e.metadata.name === draft.environment),
    );
  const flow = useCreateFlow<ClusterDraft>({
    steps: CLUSTER_STEPS.map((s) =>
      s.id === "basics"
        ? {
            ...s,
            check: (draft: ClusterDraft) =>
              basicsErrors(draft, {
                environments,
                namespaces: namespaces.map((ns) => ({
                  name: ns.metadata?.name ?? "",
                  environment: environmentOf(ns, environments).environment
                    ?.metadata.name,
                })),
                takenNames: [],
                conflicts,
                presets: offered(draft).map((p) => p.metadata.name ?? ""),
              }),
          }
        : s,
    ),
    initial,
  });
  return (
    <WizardStepper
      steps={CLUSTER_STEPS.map((s) => ({ ...s, errors: flow.errors[s.id] }))}
      current={flow.current}
    >
      <ClusterStepBasics
        draft={flow.draft}
        set={flow.set}
        environments={environments}
        counts={environmentCounts(environments, namespaces, [])}
        namespaces={namespaces}
        presets={offered(flow.draft)}
        usage={new Map(Object.entries(usage))}
        nameErrors={[
          ...claimNameErrors(flow.draft.claim.metadata.name, []),
          ...conflicts,
        ]}
        hosts={hosts}
      />
    </WizardStepper>
  );
}
