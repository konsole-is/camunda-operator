"use client";

import type { V1Namespace } from "@kubernetes/client-node";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import { useCallback, useEffect, useRef, useState } from "react";
import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import type { DatabaseServerPreset } from "@/generated/crds/databaseserverpreset";
import type { ElasticsearchClusterPreset } from "@/generated/crds/elasticsearchclusterpreset";
import type { Environment } from "@/generated/crds/environment";
import type { ObjectStorageConfig } from "@/generated/crds/objectstorageconfig";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { SecondaryStorageConfig } from "@/generated/crds/secondarystorageconfig";
import { useAnswered } from "@/lib/k8s/answered";
import { useResources, useWatchesFresh } from "@/lib/k8s/hooks";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import type { ListOptions } from "@/lib/k8s/read";
import { blockingMergedErrors } from "@/components/cluster/editable-settings";
import {
  createReleaseChoices,
  releaseLabel,
} from "@/components/cluster/edit/release-options";
import {
  DELETION_LABELS,
  deletionQuestion,
} from "@/components/fleet/deletion-choice";
import type { SecretRefs } from "@/components/fleet/secret-ref-picker";
import { Toast, useWriteToast } from "@/components/fleet/toast";
import { WizardStepper } from "@/components/fleet/wizard-stepper";
import { Button } from "@/components/ui/button";
import { hostConflicts, renderHosts, takenHosts } from "@/lib/claims/hosts";
import { claimNameErrors, namesIn } from "@/lib/claims/names";
import { grantPreviews } from "@/lib/claims/templates";
import type { ReadError } from "@/lib/k8s/unread";
import { ACCESS_SECTION, SECTION_LABEL } from "@/lib/labels";
import { clusterFields, pathFrom, type Step } from "@/lib/fields";
import {
  byCodeUnit,
  type DeletionChoice,
  environmentOf,
  type PluginSelection,
} from "@/lib/policy";
import { clusterLocks, filledClaimSteps } from "@/lib/session";
import type { ValidationError } from "@/lib/schema";
import { environmentCounts, presetUsage } from "@/lib/usage";
import {
  CLUSTER_STEPS,
  type ClusterClaimObject,
  type ClusterDraft,
  attachmentsErrors,
  basicsErrors,
  clusterClaimOf,
  clusterCreateChange,
  draftCluster,
  draftOptimize,
  isWrittenClaim,
  offeredPresets,
  releaseOf,
  type SecretsRead,
  secretErrors,
  secretFieldErrors,
  settingsErrors,
  withClaimValue,
  withoutClaimValue,
  withoutClaimValues,
} from "@/lib/clusters/draft";
import { createdObjects } from "@/lib/clusters/review";
import {
  type StorageData,
  type StorageSummary,
  clusterDeletion,
  draftAccessPatterns,
  storageErrors,
  storageSummary,
  storageViews,
} from "@/lib/clusters/storage";
import { ClusterAttachmentsStep } from "./cluster-attachments-step";
import { ClusterBasicsStep } from "./cluster-basics-step";
import { ClusterProgress } from "./cluster-progress";
import {
  type ClusterReviewSummary,
  ClusterReviewStep,
} from "./cluster-review-step";
import { ClusterSettingsStep } from "./cluster-settings-step";
import { ClusterStorageStep } from "./cluster-storage-step";
import { NamespaceSecrets } from "./namespace-secrets";
import { type CreateStep, useCreateFlow } from "./use-create-flow";
import type {
  Change,
  Preview,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";

/**
 * ClusterCreateData is the first snapshot of the create cluster page, from
 * every namespace. accessRequests are the requests of the access section.
 * Step 3 lists releases and platformConfigs, and step 4 lists
 * managementAuthConfigs. readErrors are the failed server reads of the
 * lists.
 */
export type ClusterCreateData = {
  environments: Resource<"Environment">[];
  namespaces: Resource<"Namespace">[];
  claims: Resource<"CamundaClusterClaim">[];
  clusters: Resource<"CamundaCluster">[];
  presets: Resource<"CamundaClusterPreset">[];
  sections: Resource<"PluginSection">[];
  registrations: Resource<"PluginRegistration">[];
  accessRequests: Resource<"PluginRequest">[];
  elasticsearchPresets: Resource<"ElasticsearchClusterPreset">[];
  databaseServerPresets: Resource<"DatabaseServerPreset">[];
  secondaryStorageConfigs: Resource<"SecondaryStorageConfig">[];
  objectStorageConfigs: Resource<"ObjectStorageConfig">[];
  releases: Resource<"CamundaRelease">[];
  platformConfigs: Resource<"CamundaPlatformConfig">[];
  managementAuthConfigs: Resource<"ManagementAuthConfig">[];
  readErrors: ReadError[];
};

const EVERYWHERE = {};
const ACCESS_REQUESTS = { labelSelector: `${SECTION_LABEL}=${ACCESS_SECTION}` };

const INITIAL: ClusterDraft = { claim: { metadata: { name: "" }, spec: {} } };

// secretErrors starts the error of an attachment with this path.
const ATTACHMENTS = "attachments.";

/**
 * ClusterCreateFlow is the create cluster wizard. It follows initial live
 * and checks steps 1 to 4 against the newest data. A list that cannot be
 * read, other than the Secrets, blocks each step that reads it. The picks
 * of a section that goes away leave the draft. Step 3 edits the release,
 * the platform config, and the settings, and step 4 the attachments. A
 * claim value that a step 2 option fills leaves the draft. Step 5 reviews
 * the change, previews it with preview, and on Create writes it once with
 * create: the new namespace, when step 1 makes one, then the claim. After
 * the write, the flow follows the claim. When a refused write created the
 * new namespace, create must name it in createdNamespace: the draft then
 * moves to that namespace.
 */
export function ClusterCreateFlow({
  initial,
  mode,
  preview,
  create,
}: {
  initial: ClusterCreateData;
  mode: WriteMode;
  preview: (change: Change) => Promise<Preview>;
  create: (change: Change) => Promise<WriteResult>;
}) {
  const { readErrors } = initial;
  const live = {
    environments: useLive(
      "Environment",
      EVERYWHERE,
      initial.environments,
      readErrors,
    ),
    namespaces: useLive(
      "Namespace",
      EVERYWHERE,
      initial.namespaces,
      readErrors,
    ),
    claims: useLive(
      "CamundaClusterClaim",
      EVERYWHERE,
      initial.claims,
      readErrors,
    ),
    clusters: useLive(
      "CamundaCluster",
      EVERYWHERE,
      initial.clusters,
      readErrors,
    ),
    presets: useLive(
      "CamundaClusterPreset",
      EVERYWHERE,
      initial.presets,
      readErrors,
    ),
    sections: useLive(
      "PluginSection",
      EVERYWHERE,
      initial.sections,
      readErrors,
    ),
    registrations: useLive(
      "PluginRegistration",
      EVERYWHERE,
      initial.registrations,
      readErrors,
    ),
    accessRequests: useLive(
      "PluginRequest",
      ACCESS_REQUESTS,
      initial.accessRequests,
      readErrors,
    ),
  };
  const settingsLive = {
    releases: useLive(
      "CamundaRelease",
      EVERYWHERE,
      initial.releases,
      readErrors,
    ),
    platformConfigs: useLive(
      "CamundaPlatformConfig",
      EVERYWHERE,
      initial.platformConfigs,
      readErrors,
    ),
    managementAuthConfigs: useLive(
      "ManagementAuthConfig",
      EVERYWHERE,
      initial.managementAuthConfigs,
      readErrors,
    ),
  };
  const sources = {
    ElasticsearchClusterPreset: useLive(
      "ElasticsearchClusterPreset",
      EVERYWHERE,
      initial.elasticsearchPresets,
      readErrors,
    ),
    DatabaseServerPreset: useLive(
      "DatabaseServerPreset",
      EVERYWHERE,
      initial.databaseServerPresets,
      readErrors,
    ),
    SecondaryStorageConfig: useLive(
      "SecondaryStorageConfig",
      EVERYWHERE,
      initial.secondaryStorageConfigs,
      readErrors,
    ),
    ObjectStorageConfig: useLive(
      "ObjectStorageConfig",
      EVERYWHERE,
      initial.objectStorageConfigs,
      readErrors,
    ),
  };
  // A list that cannot be read would let a taken name or host, or a rule of
  // an Environment, pass unseen, so it blocks the step that reads it.
  const failures = Object.values(live).flatMap((l) =>
    l.failure ? [l.failure] : [],
  );
  const storageFailures = [
    live.environments,
    live.sections,
    live.registrations,
    live.accessRequests,
    ...Object.values(sources),
  ].flatMap((l) => (l.failure ? [l.failure] : []));
  const settingsFailures = [
    live.environments,
    live.sections,
    live.registrations,
    settingsLive.releases,
    settingsLive.platformConfigs,
  ].flatMap((l) => (l.failure ? [l.failure] : []));
  const unread: KindName[] = Object.values(settingsLive).flatMap((l) =>
    l.failure ? [l.kind] : [],
  );
  const attachmentsFailures = [
    ...storageFailures,
    ...(settingsLive.managementAuthConfigs.failure
      ? [settingsLive.managementAuthConfigs.failure]
      : []),
  ];
  const objects = {
    ElasticsearchClusterPreset: models<ElasticsearchClusterPreset>(
      sources.ElasticsearchClusterPreset.items,
    ),
    DatabaseServerPreset: models<DatabaseServerPreset>(
      sources.DatabaseServerPreset.items,
    ),
    SecondaryStorageConfig: models<SecondaryStorageConfig>(
      sources.SecondaryStorageConfig.items,
    ),
    ObjectStorageConfig: models<ObjectStorageConfig>(
      sources.ObjectStorageConfig.items,
    ),
  };
  // The errors that each section of step 2 last reported, by section name.
  const [cardErrors, setCardErrors] = useState<
    Record<string, ValidationError[]>
  >({});
  const environments = models<Environment>(live.environments.items);
  const namespaces = models<V1Namespace>(live.namespaces.items);
  const claims = live.claims.items;
  const clusters = models<CamundaCluster>(live.clusters.items);
  const allPresets = models<CamundaClusterPreset>(live.presets.items);
  const sections = models<PluginSection>(live.sections.items);
  const registrations = models<PluginRegistration>(live.registrations.items);
  const accessRequests = models<PluginRequest>(live.accessRequests.items);
  const releases = settingsLive.releases.items;
  const platformConfigs = settingsLive.platformConfigs.items;
  const managementAuthConfigs = settingsLive.managementAuthConfigs.items
    .map((a) => a.metadata?.name ?? "")
    .sort(byCodeUnit);
  const editor = useShownErrors();
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
    const data: StorageData = { environment, sections, registrations, objects };
    const views = storageViews(draft, data);
    const presets = offeredPresets(allPresets, environment);
    const name = draft.claim.metadata.name;
    const namespace = draft.namespace?.name ?? "";
    const takenNames =
      draft.namespace?.kind === "existing"
        ? namesIn(namespace, claims, clusters)
        : [];
    const patterns = draftAccessPatterns(draft, data);
    const hosts =
      patterns && draft.namespace && claimNameErrors(name, []).length === 0
        ? renderHosts(patterns, name, namespace)
        : [];
    const conflicts = hostConflicts(hosts, takenHosts(accessRequests));
    const deletion = clusterDeletion(draft, data);
    const drafted = draftCluster(
      draft,
      live.presets.items,
      releases,
      environment,
    );
    const fields = clusterFields({
      cluster: drafted.cluster,
      merged: drafted.merged,
      claim: drafted.claim as unknown as Resource<"CamundaClusterClaim">,
      sections: live.sections.items,
      registrations: live.registrations.items,
      optimize: draftOptimize(draft),
    });
    const locks = clusterLocks(fields);
    const summary = storageSummary(draft, data);
    return {
      environment,
      data,
      views,
      presets,
      takenNames,
      hosts,
      conflicts,
      deletion,
      drafted,
      fields,
      locks,
      summary,
    };
  };

  const [secrets, setSecrets] = useState<SecretsRead>();
  // The namespaces whose Secret watch answered once: before that, an empty
  // list does not mean that the namespace holds no Secret.
  const [secretsAnswered, setSecretsAnswered] = useState<ReadonlySet<string>>(
    new Set(),
  );
  const secretsRead =
    secrets &&
    (secretsAnswered.has(secrets.namespace) || secrets.error !== undefined)
      ? secrets
      : undefined;
  const checks: Record<string, CreateStep<ClusterDraft>["check"]> = {
    basics: (draft) => {
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
    storage: (draft) => {
      const { data, conflicts } = derive(draft);
      return [
        ...storageFailures,
        ...storageErrors(draft, data, cardErrors, conflicts),
      ];
    },
    settings: (draft) => {
      const { environment, fields, drafted } = derive(draft);
      return [
        ...settingsFailures,
        ...settingsErrors(draft, {
          environment,
          releases,
          platformConfigs,
          fields,
          unread,
        }),
        ...secretErrors(draft, secretsRead).filter(
          (e) => !e.startsWith(ATTACHMENTS),
        ),
        // The value of the cluster can fail while step 3 is not open, as
        // after a change of the preset.
        ...blockingMergedErrors(
          fields,
          drafted.merged.spec,
          draft.claim.spec.cluster ?? {},
        ).map((e) => `${e.label}: ${e.message}`),
        ...editor.shown("spec.cluster"),
      ];
    },
    attachments: (draft) => [
      ...attachmentsFailures,
      ...attachmentsErrors(draft, {
        summary: derive(draft).summary,
        managementAuthConfigs,
        unread,
      }),
      ...secretErrors(draft, secretsRead).filter((e) =>
        e.startsWith(ATTACHMENTS),
      ),
      ...editor.shown("spec.attachments"),
    ],
  };
  const steps: CreateStep<ClusterDraft>[] = CLUSTER_STEPS.map((step) =>
    Object.hasOwn(checks, step.id) ? { ...step, check: checks[step.id] } : step,
  );
  const flow = useCreateFlow({ steps, initial: INITIAL });
  const {
    environment,
    views,
    presets,
    takenNames,
    hosts,
    conflicts,
    deletion,
    drafted,
    fields,
    locks,
    summary,
  } = derive(flow.draft);
  // A claim must not select a section that step 2 no longer shows.
  const gone = Object.keys(flow.draft.storage ?? {}).filter(
    (name) => !views.some((v) => v.name === name),
  );
  useEffect(() => {
    for (const name of gone) flow.set(["storage", name], undefined);
  });
  // The claim controller refuses a claim value that a section fills.
  const unfilled = withoutClaimValues(flow.draft, filledClaimSteps(fields));
  useEffect(() => {
    if (unfilled !== flow.draft) flow.set(["claim"], unfilled.claim);
  });
  const editClaim = (edited: ClusterDraft) => flow.set(["claim"], edited.claim);
  const setClaim = (steps: Step[], value: unknown) =>
    editClaim(withClaimValue(flow.draft, steps, value));
  const resetClaim = (steps: Step[]) =>
    editClaim(withoutClaimValue(flow.draft, steps));
  const existing =
    flow.draft.namespace?.kind === "existing"
      ? flow.draft.namespace.name
      : undefined;
  const onSecrets = useCallback(
    (refs: SecretRefs, error?: string) =>
      setSecrets({
        namespace: existing ?? "",
        secrets: refs,
        ...(error !== undefined && { error }),
      }),
    [existing],
  );
  const [secretsFresh] = useWatchesFresh(
    existing === undefined
      ? []
      : [{ kind: "Secret", opts: { namespace: existing } }],
  );
  if (existing !== undefined && secretsFresh && !secretsAnswered.has(existing))
    setSecretsAnswered(new Set([...secretsAnswered, existing]));
  const secretsHere = secrets?.namespace === existing ? secrets : undefined;
  const secretsError = secretsHere?.error;
  const onSection = (
    name: string,
    selections: PluginSelection[],
    errors: ValidationError[],
  ) => {
    flow.set(["storage", name], selections);
    setCardErrors((current) => ({ ...current, [name]: errors }));
  };
  const { toast, show, dismiss } = useWriteToast();
  const [written, setWritten] = useState<ClusterClaimObject>();
  const [keptNamespace, setKeptNamespace] = useState<string>();
  const [refused, setRefused] = useState<{
    claim: ClusterClaimObject;
    unavailable: boolean;
  }>();
  // Each write of a claim after one lost its answer. A later refusal does
  // not show that an earlier write failed.
  const [lostWrites, setLostWrites] = useState<
    ReadonlyMap<string, readonly { spec: unknown; startedAt: number }[]>
  >(new Map());
  // After a lost answer, Create waits for Try again, which runs the dry run
  // again by mounting the review step again.
  const [waitsForRetry, setWaitsForRetry] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const startedAt = useRef(0);
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
    startedAt.current = Date.now();
    setWriting(true);
    try {
      return await create(change);
    } finally {
      setWriting(false);
    }
  };
  const onWritten = (result: WriteResult) => {
    const { name } = flow.draft.claim.metadata;
    show(result, `Created the cluster ${name}.`);
    if (result.ok) {
      setWritten(clusterClaimOf(flow.draft, environment, deletion));
      return;
    }
    const claim = clusterClaimOf(flow.draft, environment, deletion);
    const key = `${claim.metadata.namespace}/${claim.metadata.name}`;
    const lost = result.errors.some((e) => e.code === "Unavailable");
    if (lost) setWaitsForRetry(true);
    if (lost || lostWrites.has(key)) {
      const writes = [
        ...(lostWrites.get(key) ?? []),
        { spec: claim.spec, startedAt: startedAt.current },
      ];
      setLostWrites(new Map([...lostWrites, [key, writes]]));
    }
    setRefused({ claim, unavailable: lost || lostWrites.has(key) });
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
  const refusedRef = refused && {
    namespace: refused.claim.metadata.namespace,
    name: refused.claim.metadata.name,
  };
  const taken =
    refusedRef &&
    refusedRef.namespace === flow.draft.namespace?.name &&
    refusedRef.name === flow.draft.claim.metadata.name
      ? claims.find(
          (c) =>
            c.metadata?.namespace === refusedRef.namespace &&
            c.metadata?.name === refusedRef.name,
        )
      : undefined;
  // A write whose answer was lost can have landed.
  const landed =
    taken &&
    refusedRef &&
    (lostWrites.get(`${refusedRef.namespace}/${refusedRef.name}`) ?? []).some(
      (w) => isWrittenClaim(taken, w.spec, w.startedAt),
    )
      ? taken
      : undefined;
  const earlier = steps
    .filter((s) => s.id !== "review")
    .some((s) => (flow.errors[s.id] ?? []).length > 0);
  const unreadKinds = [
    ...Object.values(live),
    ...Object.values(sources),
    ...Object.values(settingsLive),
  ].flatMap((l) => (l.failure ? [l.kind] : []));

  const toastView = toast && (
    <div className="fixed right-4 bottom-4 z-50">
      <Toast result={toast} onDismiss={dismiss} />
    </div>
  );

  if (written) {
    return (
      <div className="flex min-w-0 flex-col gap-4">
        <ClusterProgress
          claim={written}
          sections={sections}
          registrations={registrations}
        />
        {toastView}
      </div>
    );
  }

  const claim = clusterClaimOf(flow.draft, environment, deletion);

  return (
    <div className="flex min-w-0 flex-col gap-4">
      {existing !== undefined && secretsError !== undefined && (
        <p role="alert" className="text-sm text-bad">
          Could not read Secret: {secretsError}
        </p>
      )}
      {existing !== undefined && (
        <NamespaceSecrets
          key={existing}
          namespace={existing}
          onChange={onSecrets}
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
        {flow.current === "basics" ? (
          <ClusterBasicsStep
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
        ) : flow.current === "storage" ? (
          <ClusterStorageStep
            draft={flow.draft}
            views={views}
            registrations={registrations}
            sections={sections}
            context={{ environment, objects }}
            hosts={hosts}
            conflicts={conflicts}
            onSection={onSection}
            deletion={deletion}
            onDeletion={(policy) =>
              flow.set(["claim", "spec", "deletionPolicy"], policy)
            }
          />
        ) : flow.current === "settings" ? (
          <ClusterSettingsStep
            draft={flow.draft}
            environment={environment}
            releases={createReleaseChoices(releases, environment)}
            platformConfigs={platformConfigs
              .map((p) => p.metadata?.name ?? "")
              .sort(byCodeUnit)}
            presets={live.presets.items.map((p) => p.metadata?.name ?? "")}
            fields={fields}
            merged={drafted.merged}
            // A Secret that cannot be read cannot be picked, and the alert
            // above names the failure.
            secrets={
              secretsHere && secretsError === undefined
                ? secretsHere.secrets
                : []
            }
            pick={(field, name) => flow.set(["claim", "spec", field], name)}
            set={setClaim}
            reset={resetClaim}
            invalid={editor.invalid}
            errors={secretFieldErrors(flow.draft, secretsRead)}
            unread={unread}
          />
        ) : flow.current === "attachments" ? (
          <ClusterAttachmentsStep
            value={flow.draft.claim.spec.attachments}
            summary={summary}
            managementAuthConfigs={managementAuthConfigs}
            lock={(steps) => locks(pathFrom("", steps))}
            set={setClaim}
            reset={resetClaim}
            invalid={editor.invalid}
            unread={unread}
          />
        ) : (
          <ClusterReviewStep
            key={attempt}
            change={clusterCreateChange(flow.draft, environment, deletion)}
            summary={reviewSummary(flow.draft, {
              environment,
              summary,
              deletion,
              releases: createReleaseChoices(releases, environment),
            })}
            effects={createdObjects({
              claim,
              newNamespace: flow.draft.namespace?.kind === "new",
              sections,
              registrations,
            })}
            grants={grantPreviews({
              claim: {
                kind: "CamundaClusterClaim",
                namespace: claim.metadata.namespace,
                name: claim.metadata.name,
                plugins: claim.spec.plugins ?? [],
                releaseRef: releaseOf(flow.draft, environment),
              },
              sections,
              registrations,
              environment,
            })}
            mode={mode}
            preview={preview}
            create={createOnce}
            onWritten={onWritten}
            onCancel={flow.back}
            blocked={earlier || writing || waitsForRetry}
            unread={unreadKinds}
          />
        )}
      </WizardStepper>
      {keptNamespace !== undefined &&
        existing === keptNamespace &&
        !refused?.unavailable && (
          <p
            role="note"
            className="rounded-md border border-warn/40 bg-warn/10 p-3 text-sm text-text"
          >
            {`The write created the namespace ${keptNamespace}, but not the claim. The namespace stays, and the cluster now goes into it as an existing namespace. Fix the cause and create the cluster again.`}
          </p>
        )}
      {waitsForRetry && !taken && refusedRef && (
        <div
          role="note"
          className="flex flex-wrap items-center gap-3 rounded-md border border-warn/40 bg-warn/10 p-3 text-sm text-text"
        >
          <p>
            {keptNamespace !== undefined && existing === keptNamespace
              ? `The write created the namespace ${keptNamespace}, and its answer for the claim was lost. The namespace stays, and the cluster now goes into it as an existing namespace. If the claim does not show up, try again.`
              : `The answer of the write was lost. The claim ${refusedRef.name} does not show in ${refusedRef.namespace} yet. If it does not show up, try again.`}
          </p>
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              setWaitsForRetry(false);
              setAttempt((n) => n + 1);
            }}
          >
            Try again
          </Button>
        </div>
      )}
      {taken && refusedRef && (
        <div
          role="note"
          className="flex flex-wrap items-center gap-3 rounded-md border border-warn/40 bg-warn/10 p-3 text-sm text-text"
        >
          {landed ? (
            <>
              <p>{`The write reported an error, but the claim ${refusedRef.name} exists in ${refusedRef.namespace} now.`}</p>
              <Button
                variant="outline"
                size="sm"
                onClick={() =>
                  setWritten(landed as unknown as ClusterClaimObject)
                }
              >
                Follow the claim
              </Button>
            </>
          ) : (
            <p>{`A claim ${refusedRef.name} exists in ${refusedRef.namespace} already. This create did not make it.`}</p>
          )}
        </div>
      )}
      {toastView}
    </div>
  );
}

// reviewSummary returns the summary of steps 1 to 4 of draft for the
// review. It shows no ARN: the picks of step 2 come from storageSummary.
function reviewSummary(
  draft: ClusterDraft,
  data: {
    environment: Environment | undefined;
    summary: StorageSummary;
    deletion: DeletionChoice;
    releases: readonly ReturnType<typeof createReleaseChoices>[number][];
  },
): ClusterReviewSummary {
  const { environment, summary, deletion } = data;
  const { spec } = draft.claim;
  const release = releaseOf(draft, environment);
  const option = data.releases.find((r) => r.name === release);
  const platformDefault = environment?.spec.platformConfigRef;
  const platform = spec.platformConfigRef ?? platformDefault;
  const changed = leafCount(spec.cluster);
  const { optimize, backupSchedule } = spec.attachments ?? {};
  const retained = backupSchedule?.retained;
  return [
    {
      label: "Environment and name",
      terms: [
        ...(environment
          ? [
              {
                label: "Environment",
                value:
                  environment.spec.displayName ||
                  (environment.metadata.name ?? ""),
              },
            ]
          : []),
        { label: "Name", value: draft.claim.metadata.name },
        {
          label: "Namespace",
          value: draft.namespace?.name ?? "",
          ...(draft.namespace?.kind === "new" && { note: "New" }),
        },
        { label: "Preset", value: spec.presetRef ?? "" },
      ],
    },
    {
      label: "Storage and access",
      terms: [
        ...summary.picks.map((pick) => ({
          label: pick.sectionLabel,
          value: pick.optionLabel,
          ...(pick.values.length > 0 && {
            note: pick.values.map((v) => `${v.label}: ${v.value}`).join(", "),
          }),
        })),
        {
          label: deletionQuestion("cluster"),
          value: DELETION_LABELS[deletion.policy],
        },
      ],
    },
    {
      label: "Cluster settings",
      terms: [
        {
          label: "Release",
          value: option
            ? releaseLabel(option, option.isDefault)
            : (release ?? ""),
        },
        {
          label: "Platform config",
          value: platform ?? "",
          ...(spec.platformConfigRef === undefined &&
            platform !== undefined && { note: "Default of the environment" }),
        },
        {
          label: "Changed settings",
          value: changed === 0 ? "None" : String(changed),
        },
      ],
    },
    {
      label: "Attachments",
      terms: [
        {
          label: "Optimize",
          value: optimize
            ? `On, with the ManagementAuthConfig ${optimize.managementAuthConfigRef}`
            : "Off",
        },
        {
          label: "Backup schedule",
          value: backupSchedule ? backupSchedule.schedule : "Off",
          ...(retained && {
            note: [
              retained.completed !== undefined &&
                `Keep ${retained.completed} completed`,
              retained.failed !== undefined && `Keep ${retained.failed} failed`,
            ]
              .filter(Boolean)
              .join(", "),
          }),
        },
      ],
    },
  ];
}

// leafCount returns the number of values in value that are not plain
// objects. A list counts as one value, and undefined counts as none.
function leafCount(value: unknown): number {
  if (typeof value !== "object" || value === null || Array.isArray(value))
    return value === undefined ? 0 : 1;
  return Object.values(value).reduce<number>((n, v) => n + leafCount(v), 0);
}

// useShownErrors holds the error that each editor of steps 3 and 4 shows,
// by claim path. invalid is stable and ignores a message that the path
// already shows: an editor clears its error in an effect that depends on
// invalid, so a new invalid on each render would clear it again and again.
function useShownErrors(): {
  shown: (prefix: string) => string[];
  invalid: (steps: Step[], message: string | undefined) => void;
} {
  const [errors, setErrors] = useState<ReadonlyMap<string, string>>(new Map());
  const errorsRef = useRef(errors);
  const invalid = useCallback((steps: Step[], message: string | undefined) => {
    const path = pathFrom("", steps);
    const current = errorsRef.current;
    if (current.get(path) === message) return;
    const next = new Map(current);
    if (message === undefined) next.delete(path);
    else next.set(path, message);
    errorsRef.current = next;
    setErrors(next);
  }, []);
  const shown = (prefix: string) =>
    [...errors]
      .filter(([path]) => path === prefix || path.startsWith(`${prefix}.`))
      .map(([, message]) => message);
  return { shown, invalid };
}

function useLive<K extends KindName>(
  kind: K,
  opts: ListOptions,
  initial: Resource<K>[],
  readErrors: readonly ReadError[],
): { kind: K; items: Resource<K>[]; failure?: string } {
  const { items, error: watchError } = useResources(kind, opts, initial);
  // A failed read of the server counts until the watch answers once.
  const answered = useAnswered([{ kind, opts }]).has(kind);
  const readError = answered
    ? undefined
    : readErrors.find((e) => e.kind === kind)?.message;
  const error = watchError ?? readError;
  return error === undefined
    ? { kind, items }
    : {
        kind,
        items,
        failure: `The fleet manager could not read the ${kind} objects: ${error}`,
      };
}

// A Resource is plain JSON, so it differs from its model type only in its
// timestamps, which neither the policy rules nor the step read.
function models<T>(items: readonly unknown[]): T[] {
  return items as T[];
}
