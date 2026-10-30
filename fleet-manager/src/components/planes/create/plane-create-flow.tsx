"use client";

import type { V1Namespace } from "@kubernetes/client-node";
import { type ReactNode, useState } from "react";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import type { Environment } from "@/generated/crds/environment";
import {
  type CreateStep,
  type DraftPath,
  useCreateFlow,
} from "@/components/create/use-create-flow";
import type { SecretRefs } from "@/components/fleet/secret-ref-picker";
import { WizardStepper } from "@/components/fleet/wizard-stepper";
import type { Plane, PlaneSources } from "@/components/planes/plane";
import { type ReadError, usePlanes } from "@/components/planes/planes-table";
import { namesIn } from "@/lib/claims/names";
import { environmentOf } from "@/lib/policy";
import { useResources } from "@/lib/k8s/hooks";
import type { Resource } from "@/lib/k8s/kinds";
import {
  EMPTY_PLANE_DRAFT,
  type PlaneContext,
  type PlaneDraft,
  basicsErrors,
  servedErrors,
} from "@/lib/planes/draft";
import type { ValidationError } from "@/lib/schema";
import { BasicsStep } from "./basics-step";
import { ServedStep } from "./served-step";

/** PlaneCreateData is the first snapshot of the new plane page, from every namespace. planes are the lists of planesOf. */
export type PlaneCreateData = {
  environments: Resource<"Environment">[];
  namespaces: Resource<"Namespace">[];
  clusters: Resource<"CamundaCluster">[];
  platformConfigs: Resource<"CamundaPlatformConfig">[];
  planes: PlaneSources;
};

/** PLANE_STEPS are the steps of the plane create flow that exist so far, in order. */
export const PLANE_STEPS: readonly { id: string; label: string }[] = [
  { id: "basics", label: "Basics" },
  { id: "served", label: "Environments served" },
];

/** PlaneModels are the lists that the steps of the plane create flow read. */
export type PlaneModels = {
  environments: Environment[];
  namespaces: V1Namespace[];
  clusters: CamundaCluster[];
  platformConfigs: CamundaPlatformConfig[];
  planes: Plane[];
  sources: PlaneSources;
};

/**
 * planeContextOf returns the context that the steps check draft against.
 * The taken names are those of the plane claims and the
 * CamundaManagementClusters in an existing namespace of draft.
 */
export function planeContextOf(
  models: PlaneModels,
  draft: PlaneDraft,
): PlaneContext {
  return {
    environments: models.environments,
    platformConfigs: models.platformConfigs,
    takenNames:
      draft.namespace?.kind === "existing"
        ? namesIn(
            draft.namespace.name,
            models.sources.claims,
            models.sources.clusters,
          )
        : [],
    namespaces: models.namespaces.map((ns) => ({
      name: ns.metadata?.name ?? "",
      environment: environmentOf(ns, models.environments).environment?.metadata
        .name,
    })),
  };
}

/**
 * planeSteps returns PLANE_STEPS with the check of each step.
 * selectorErrors are the errors of the cluster labels. failures are the
 * lists that cannot be read, and each blocks step 1.
 */
export function planeSteps(
  models: PlaneModels,
  selectorErrors: readonly ValidationError[],
  failures: readonly ReadError[] = [],
): CreateStep<PlaneDraft>[] {
  return PLANE_STEPS.map((step) => ({
    ...step,
    check: (draft: PlaneDraft) => {
      const ctx = planeContextOf(models, draft);
      // A list that cannot be read would let a taken name, or a rule of an
      // Environment, pass unseen.
      return step.id === "basics"
        ? [
            ...failures.map(
              (f) =>
                `The fleet manager could not read the ${f.kind} objects: ${f.message}`,
            ),
            ...basicsErrors(draft, ctx),
          ]
        : servedErrors(draft, ctx, selectorErrors);
    },
  }));
}

/**
 * PlaneStepBody renders the step current of the plane create flow. secrets
 * are the Secrets of the namespace of draft.
 */
export function PlaneStepBody({
  current,
  draft,
  set,
  models,
  secrets,
  onSelectorErrors,
}: {
  current: string;
  draft: PlaneDraft;
  set: (path: DraftPath, value: unknown) => void;
  models: PlaneModels;
  secrets: SecretRefs;
  onSelectorErrors: (errors: ValidationError[]) => void;
}) {
  const ctx = planeContextOf(models, draft);
  return current === "served" ? (
    <ServedStep
      draft={draft}
      set={set}
      ctx={ctx}
      namespaces={models.namespaces}
      clusters={models.clusters}
      planes={models.planes}
      onSelectorErrors={onSelectorErrors}
    />
  ) : (
    <BasicsStep
      draft={draft}
      set={set}
      ctx={ctx}
      namespaces={models.namespaces}
      clusters={models.clusters}
      secrets={secrets}
    />
  );
}

/**
 * PlaneCreateFlow is the plane create wizard. It follows each list of
 * initial live and checks each step against the newest data. The error of
 * each failed watch shows above the steps, and the lists then keep their
 * last items. It writes nothing.
 */
export function PlaneCreateFlow({ initial }: { initial: PlaneCreateData }) {
  const environments = useResources("Environment", {}, initial.environments);
  const namespaces = useResources("Namespace", {}, initial.namespaces);
  const clusters = useResources("CamundaCluster", {}, initial.clusters);
  const platformConfigs = useResources(
    "CamundaPlatformConfig",
    {},
    initial.platformConfigs,
  );
  const planes = usePlanes(initial.planes);
  const managementClusters = useResources(
    "CamundaManagementCluster",
    {},
    initial.planes.clusters,
  );
  const claims = useResources(
    "CamundaManagementClusterClaim",
    {},
    initial.planes.claims,
  );
  const readErrors: ReadError[] = [
    ...(
      [
        ["Environment", environments],
        ["CamundaCluster", clusters],
        ["CamundaPlatformConfig", platformConfigs],
      ] as const
    ).flatMap(([kind, result]) =>
      result.error ? [{ kind, message: result.error }] : [],
    ),
    ...planes.errors,
  ];
  const models: PlaneModels = {
    environments: modelsOf<Environment>(environments.items),
    namespaces: modelsOf<V1Namespace>(namespaces.items),
    clusters: modelsOf<CamundaCluster>(clusters.items),
    platformConfigs: modelsOf<CamundaPlatformConfig>(platformConfigs.items),
    planes: planes.planes,
    sources: {
      clusters: managementClusters.items,
      claims: claims.items,
      namespaces: namespaces.items,
    },
  };
  const [selectorErrors, setSelectorErrors] = useState<ValidationError[]>([]);
  const steps = planeSteps(models, selectorErrors, readErrors);
  const flow = useCreateFlow({ steps, initial: EMPTY_PLANE_DRAFT });
  const namespace = flow.draft.namespace;

  const body = (secrets: SecretRefs) => (
    <PlaneStepBody
      current={flow.current}
      draft={flow.draft}
      set={flow.set}
      models={models}
      secrets={secrets}
      onSelectorErrors={setSelectorErrors}
    />
  );

  return (
    <div className="flex min-w-0 flex-col gap-4">
      {readErrors.map((e) => (
        <p key={e.kind} role="alert" className="text-sm text-bad">
          Could not read {e.kind}: {e.message}
        </p>
      ))}
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
        {namespace?.kind === "existing" ? (
          <NamespaceSecrets key={namespace.name} namespace={namespace.name}>
            {body}
          </NamespaceSecrets>
        ) : (
          body([])
        )}
      </WizardStepper>
    </div>
  );
}

// NamespaceSecrets follows the Secrets of one namespace, so the flow watches
// Secrets only once a namespace is chosen, and only that one.
function NamespaceSecrets({
  namespace,
  children,
}: {
  namespace: string;
  children: (secrets: SecretRefs) => ReactNode;
}) {
  const { items } = useResources("Secret", { namespace }, []);
  return children(
    items.map((s) => ({ name: s.metadata?.name ?? "", keys: s.keys })),
  );
}

// A Resource is plain JSON, so it differs from its model type only in its
// timestamps, which the steps do not read.
function modelsOf<T>(items: readonly unknown[]): T[] {
  return items as T[];
}
