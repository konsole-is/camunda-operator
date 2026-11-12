"use client";

import type { V1Namespace } from "@kubernetes/client-node";
import { useCallback, useEffect, useRef, useState } from "react";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaManagementClusterClaim } from "@/generated/crds/camundamanagementclusterclaim";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { DatabaseConfig } from "@/generated/crds/databaseconfig";
import type { DatabaseServerPreset } from "@/generated/crds/databaseserverpreset";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import {
  type CreateStep,
  type DraftPath,
  useCreateFlow,
} from "@/components/create/use-create-flow";
import {
  type SecretRefs,
  secretRefsOf,
} from "@/components/fleet/secret-ref-picker";
import { Toast, useWriteToast } from "@/components/fleet/toast";
import { WizardStepper } from "@/components/fleet/wizard-stepper";
import type { Plane, PlaneSources } from "@/components/planes/plane";
import { type ReadError, usePlanes } from "@/components/planes/planes-table";
import { namesIn } from "@/lib/claims/names";
import { ACCESS_SECTION, SECTION_LABEL } from "@/lib/labels";
import { type BuiltinObjects, environmentOf } from "@/lib/policy";
import { useResources } from "@/lib/k8s/hooks";
import type { Resource } from "@/lib/k8s/kinds";
import {
  EMPTY_PLANE_DRAFT,
  type PlaneContext,
  type PlaneDraft,
  basicsErrors,
  componentsErrors,
  planeClaimOf,
  planeCreateChange,
  servedErrors,
} from "@/lib/planes/draft";
import {
  planeSectionViews,
  shownSectionErrors,
  storageErrors,
} from "@/lib/planes/storage";
import type { ValidationError } from "@/lib/schema";
import type {
  Change,
  Preview,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";
import type { ImagesTable } from "@/startup/bundled-data";
import { ComponentsStep } from "./components-step";
import { PlaneBasicsStep } from "./plane-basics-step";
import { PlaneProgress } from "./plane-progress";
import { PlaneReviewStep } from "./plane-review-step";
import { ServedStep } from "./served-step";
import { StorageAccessStep } from "./storage-access-step";

/**
 * PlaneCreateData is the first snapshot of the new plane page, from every
 * namespace. planes are the lists of planesOf. accessRequests are the
 * requests of the access section. images is the images table of the fleet
 * manager.
 */
export type PlaneCreateData = {
  environments: Resource<"Environment">[];
  namespaces: Resource<"Namespace">[];
  clusters: Resource<"CamundaCluster">[];
  platformConfigs: Resource<"CamundaPlatformConfig">[];
  releases: Resource<"CamundaRelease">[];
  sections: Resource<"PluginSection">[];
  registrations: Resource<"PluginRegistration">[];
  accessRequests: Resource<"PluginRequest">[];
  authConfigs: Resource<"ManagementAuthConfig">[];
  databaseServerPresets: Resource<"DatabaseServerPreset">[];
  databaseConfigs: Resource<"DatabaseConfig">[];
  planes: PlaneSources;
  images: ImagesTable;
};

const ACCESS_REQUESTS = {
  labelSelector: `${SECTION_LABEL}=${ACCESS_SECTION}`,
};

// STORAGE_KINDS are the kinds that no step before the storage step reads. A
// failed read of one of them blocks the storage step.
const STORAGE_KINDS: readonly string[] = [
  "PluginSection",
  "PluginRegistration",
  "PluginRequest",
  "ManagementAuthConfig",
  "DatabaseServerPreset",
  "DatabaseConfig",
];

/** PLANE_STEPS are the steps of the plane create flow that exist so far, in order. */
export const PLANE_STEPS: readonly { id: string; label: string }[] = [
  { id: "basics", label: "Basics" },
  { id: "served", label: "Environments served" },
  { id: "components", label: "Components and versions" },
  { id: "storage", label: "Storage and access" },
  { id: "review", label: "Review" },
];

/**
 * PlaneReview is what the review step needs to write: the write mode, the
 * dry run and the create of the server actions, and onWritten for the
 * result. blocked keeps Create disabled.
 */
export type PlaneReview = {
  mode: WriteMode;
  preview: (change: Change) => Promise<Preview>;
  create: (change: Change) => Promise<WriteResult>;
  onWritten: (result: WriteResult) => void;
  blocked?: boolean;
};

/** PlaneModels are the lists that the steps of the plane create flow read. */
export type PlaneModels = {
  environments: Environment[];
  namespaces: V1Namespace[];
  clusters: CamundaCluster[];
  platformConfigs: CamundaPlatformConfig[];
  releases: CamundaRelease[];
  sections: PluginSection[];
  registrations: PluginRegistration[];
  accessRequests: PluginRequest[];
  authConfigs: string[];
  objects: BuiltinObjects;
  planes: Plane[];
  sources: PlaneSources;
  images: ImagesTable;
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
    releases: models.releases,
    images: models.images,
    sections: models.sections,
    registrations: models.registrations,
    accessRequests: models.accessRequests,
    authConfigs: models.authConfigs,
    planeClaims: modelsOf(models.sources.claims),
  };
}

/**
 * planeSteps returns PLANE_STEPS with the check of each step.
 * selectorErrors are the errors of the cluster labels, and sectionErrors
 * those of the sections of the storage step, by section name. failures are
 * the lists that cannot be read. secrets are the Secrets of the existing
 * namespace of the draft.
 */
export function planeSteps(
  models: PlaneModels,
  selectorErrors: readonly ValidationError[],
  failures: readonly ReadError[] = [],
  sectionErrors: Readonly<Record<string, readonly ValidationError[]>> = {},
  secrets?: SecretRefs,
): CreateStep<PlaneDraft>[] {
  const message = (f: ReadError) =>
    `The fleet manager could not read the ${f.kind} objects: ${f.message}`;
  const storageFailures = failures.filter((f) =>
    STORAGE_KINDS.includes(f.kind),
  );
  const basicsFailures = failures.filter(
    (f) => !STORAGE_KINDS.includes(f.kind),
  );
  const secretsOf = (draft: PlaneDraft) =>
    secrets && (draft.namespace?.kind === "existing" ? secrets : []);
  return PLANE_STEPS.map((step) => ({
    ...step,
    check: (draft: PlaneDraft) => {
      const ctx = planeContextOf(models, draft);
      switch (step.id) {
        case "basics":
          // A list that cannot be read would let a taken name, or a rule of
          // an Environment, pass unseen.
          return [
            ...basicsFailures.map(message),
            ...basicsErrors(draft, ctx, secretsOf(draft)),
          ];
        case "served":
          return servedErrors(draft, ctx, selectorErrors);
        case "components":
          return componentsErrors(draft, ctx, secretsOf(draft));
        case "storage": {
          const views = planeSectionViews(draft, ctx, models.objects);
          return [
            ...storageFailures.map(message),
            ...storageErrors(
              draft,
              ctx,
              shownSectionErrors(sectionErrors, views),
              views,
              models.objects,
            ),
          ];
        }
        default:
          return [];
      }
    },
  }));
}

/**
 * PlaneStepBody renders the step current of the plane create flow. secrets
 * are the Secrets of the namespace of draft. The review step needs review.
 */
export function PlaneStepBody({
  current,
  draft,
  set,
  models,
  secrets,
  onSelectorErrors,
  onSectionErrors,
  review,
}: {
  current: string;
  draft: PlaneDraft;
  set: (path: DraftPath, value: unknown) => void;
  models: PlaneModels;
  secrets: SecretRefs;
  onSelectorErrors: (errors: ValidationError[]) => void;
  onSectionErrors: (section: string, errors: ValidationError[]) => void;
  review?: PlaneReview;
}) {
  const ctx = planeContextOf(models, draft);
  if (current === "review" && review) {
    return (
      <PlaneReviewStep
        change={planeCreateChange(draft, ctx)}
        claim={planeClaimOf(draft, ctx)}
        draft={draft}
        ctx={ctx}
        {...review}
      />
    );
  }
  if (current === "storage") {
    return (
      <StorageAccessStep
        draft={draft}
        set={set}
        ctx={ctx}
        objects={models.objects}
        onSectionErrors={onSectionErrors}
      />
    );
  }
  if (current === "components") {
    return (
      <ComponentsStep draft={draft} set={set} ctx={ctx} secrets={secrets} />
    );
  }
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
    <PlaneBasicsStep
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
 * last items. When a refused write created the new namespace, create must
 * name it in createdNamespace.
 */
export function PlaneCreateFlow({
  initial,
  mode,
  preview,
  create,
}: {
  initial: PlaneCreateData;
  mode: WriteMode;
  preview: (change: Change) => Promise<Preview>;
  create: (change: Change) => Promise<WriteResult>;
}) {
  const environments = useResources("Environment", {}, initial.environments);
  const namespaces = useResources("Namespace", {}, initial.namespaces);
  const clusters = useResources("CamundaCluster", {}, initial.clusters);
  const platformConfigs = useResources(
    "CamundaPlatformConfig",
    {},
    initial.platformConfigs,
  );
  const releases = useResources("CamundaRelease", {}, initial.releases);
  const sections = useResources("PluginSection", {}, initial.sections);
  const registrations = useResources(
    "PluginRegistration",
    {},
    initial.registrations,
  );
  const accessRequests = useResources(
    "PluginRequest",
    ACCESS_REQUESTS,
    initial.accessRequests,
  );
  const authConfigs = useResources(
    "ManagementAuthConfig",
    {},
    initial.authConfigs,
  );
  const presets = useResources(
    "DatabaseServerPreset",
    {},
    initial.databaseServerPresets,
  );
  const databaseConfigs = useResources(
    "DatabaseConfig",
    {},
    initial.databaseConfigs,
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
        ["CamundaCluster", clusters],
        ["CamundaPlatformConfig", platformConfigs],
        ["CamundaRelease", releases],
        ["PluginSection", sections],
        ["PluginRegistration", registrations],
        ["PluginRequest", accessRequests],
        ["ManagementAuthConfig", authConfigs],
        ["DatabaseServerPreset", presets],
        ["DatabaseConfig", databaseConfigs],
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
    releases: modelsOf<CamundaRelease>(releases.items),
    sections: modelsOf<PluginSection>(sections.items),
    registrations: modelsOf<PluginRegistration>(registrations.items),
    accessRequests: modelsOf<PluginRequest>(accessRequests.items),
    authConfigs: authConfigs.items.map((c) => c.metadata?.name ?? ""),
    objects: {
      DatabaseServerPreset: modelsOf<DatabaseServerPreset>(presets.items),
      DatabaseConfig: modelsOf<DatabaseConfig>(databaseConfigs.items),
    },
    planes: planes.planes,
    sources: {
      clusters: managementClusters.items,
      claims: claims.items,
      namespaces: namespaces.items,
      environments: environments.items,
    },
    images: initial.images,
  };
  const [selectorErrors, setSelectorErrors] = useState<ValidationError[]>([]);
  // The errors that each section of the storage step last reported, by
  // section name.
  const [sectionErrors, setSectionErrors] = useState<
    Record<string, ValidationError[]>
  >({});
  const onSectionErrors = useCallback(
    (section: string, errors: ValidationError[]) =>
      setSectionErrors((current) => ({ ...current, [section]: errors })),
    [],
  );
  const [secrets, setSecrets] = useState<SecretRefs>([]);
  const steps = planeSteps(
    models,
    selectorErrors,
    readErrors,
    sectionErrors,
    secrets,
  );
  const flow = useCreateFlow({ steps, initial: EMPTY_PLANE_DRAFT });
  const namespace = flow.draft.namespace;
  const existing = namespace?.kind === "existing" ? namespace.name : undefined;
  const { toast, show, dismiss } = useWriteToast();
  const [written, setWritten] = useState<CamundaManagementClusterClaim>();
  const [keptNamespace, setKeptNamespace] = useState<string>();
  // Back and Next remount the review step, so the lock of a running write
  // lives here.
  const [writing, setWriting] = useState(false);
  // onWritten runs in the render of the click, after the admin could have
  // picked another namespace.
  const latest = useRef(flow.draft);
  useEffect(() => {
    latest.current = flow.draft;
  });
  const createOnce = async (change: Change): Promise<WriteResult> => {
    setWriting(true);
    try {
      return await create(change);
    } finally {
      setWriting(false);
    }
  };
  const onWritten = (result: WriteResult) => {
    const name = flow.draft.name;
    show(result, `Created the management plane ${name}.`);
    if (result.ok) {
      setWritten(
        planeClaimOf(
          flow.draft,
          planeContextOf(models, flow.draft),
        ) as CamundaManagementClusterClaim,
      );
      return;
    }
    const now = latest.current.namespace;
    if (
      result.createdNamespace !== undefined &&
      now?.kind === "new" &&
      now.name === result.createdNamespace
    ) {
      flow.set(["namespace"], {
        kind: "existing",
        name: result.createdNamespace,
      });
      setKeptNamespace(result.createdNamespace);
    }
  };
  const earlier = steps
    .filter((s) => s.id !== "review")
    .some((s) => (flow.errors[s.id] ?? []).length > 0);

  const toastView = toast && (
    <div className="fixed right-4 bottom-4 z-50">
      <Toast result={toast} onDismiss={dismiss} />
    </div>
  );

  if (written) {
    return (
      <div className="flex min-w-0 flex-col gap-4">
        <PlaneProgress claim={written} sections={models.sections} />
        {toastView}
      </div>
    );
  }

  return (
    <div className="flex min-w-0 flex-col gap-4">
      {readErrors.map((e) => (
        <p key={e.kind} role="alert" className="text-sm text-bad">
          Could not read {e.kind}: {e.message}
        </p>
      ))}
      {existing !== undefined && (
        <NamespaceSecrets
          key={existing}
          namespace={existing}
          onChange={setSecrets}
        />
      )}
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
        <PlaneStepBody
          current={flow.current}
          draft={flow.draft}
          set={flow.set}
          models={models}
          secrets={existing === undefined ? [] : secrets}
          onSelectorErrors={setSelectorErrors}
          onSectionErrors={onSectionErrors}
          review={{
            mode,
            preview,
            create: createOnce,
            onWritten,
            blocked: earlier || writing,
          }}
        />
      </WizardStepper>
      {keptNamespace !== undefined && existing === keptNamespace && (
        <p
          role="note"
          className="rounded-md border border-warn/40 bg-warn/10 p-3 text-sm text-text"
        >
          {`The write created the namespace ${keptNamespace}, but not the claim. The namespace stays, and the plane now goes into it as an existing namespace. Fix the cause and create the plane again.`}
        </p>
      )}
      {toastView}
    </div>
  );
}

// NamespaceSecrets follows the Secrets of one namespace, so the flow watches
// Secrets only once a namespace is chosen, and only that one. It renders
// nothing: a wrapper around the step would remount the step, and with it
// the namespace picker, when the choice changes.
function NamespaceSecrets({
  namespace,
  onChange,
}: {
  namespace: string;
  onChange: (secrets: SecretRefs) => void;
}) {
  const refs = secretRefsOf(useResources("Secret", { namespace }, []).items);
  const key = JSON.stringify(refs);
  useEffect(() => {
    onChange(JSON.parse(key) as SecretRefs);
  }, [key, onChange]);
  return null;
}

// A Resource is plain JSON, so it differs from its model type only in its
// timestamps, which the steps do not read.
function modelsOf<T>(items: readonly unknown[]): T[] {
  return items as T[];
}
