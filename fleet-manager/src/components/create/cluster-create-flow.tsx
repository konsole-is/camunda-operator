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
import { useResources } from "@/lib/k8s/hooks";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import type { ListOptions } from "@/lib/k8s/read";
import { createReleaseChoices } from "@/components/cluster/edit/release-options";
import type { SecretRefs } from "@/components/fleet/secret-ref-picker";
import { WizardStepper } from "@/components/fleet/wizard-stepper";
import { Button } from "@/components/ui/button";
import { hostConflicts, renderHosts, takenHosts } from "@/lib/claims/hosts";
import { claimNameErrors, namesIn } from "@/lib/claims/names";
import { ACCESS_SECTION, SECTION_LABEL } from "@/lib/labels";
import { clusterFields, pathFrom, type Step } from "@/lib/fields";
import { byCodeUnit, environmentOf, type PluginSelection } from "@/lib/policy";
import { clusterLocks } from "@/lib/session";
import type { ValidationError } from "@/lib/schema";
import { environmentCounts, presetUsage } from "@/lib/usage";
import {
  CLUSTER_STEPS,
  type ClusterDraft,
  attachmentsErrors,
  basicsErrors,
  draftCluster,
  draftOptimize,
  offeredPresets,
  settingsErrors,
  withClaimValue,
  withoutClaimValue,
} from "@/lib/clusters/draft";
import {
  type StorageData,
  clusterDeletion,
  draftAccessPatterns,
  storageErrors,
  storageSummary,
  storageViews,
} from "@/lib/clusters/storage";
import { ClusterAttachmentsStep } from "./cluster-attachments-step";
import { ClusterBasicsStep } from "./cluster-basics-step";
import { ClusterSettingsStep } from "./cluster-settings-step";
import { ClusterStorageStep } from "./cluster-storage-step";
import { NamespaceSecrets } from "./namespace-secrets";
import { type CreateStep, useCreateFlow } from "./use-create-flow";

/**
 * ClusterCreateData is the first snapshot of the create cluster page, from
 * every namespace. accessRequests are the requests of the access section.
 * Step 3 lists releases and platformConfigs, and step 4 lists
 * managementAuthConfigs.
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
};

const EVERYWHERE = {};
const ACCESS_REQUESTS = { labelSelector: `${SECTION_LABEL}=${ACCESS_SECTION}` };

const INITIAL: ClusterDraft = { claim: { metadata: { name: "" }, spec: {} } };

const OPTIMIZE_URL: Step[] = [
  "spec",
  "attachments",
  "optimize",
  "externalUrl",
].map((field) => ({ field }));

/**
 * ClusterCreateFlow is the create cluster wizard. It follows initial live
 * and checks steps 1 to 4 against the newest data. A list that cannot be
 * read blocks each step that reads it. The picks of a section that goes
 * away leave the draft. Step 3 edits the release, the platform config, and
 * the settings, and step 4 the attachments. An Optimize URL that a step 2
 * option fills leaves the draft. Step 5 holds no fields yet, and the flow
 * writes nothing. The review step holds the Back of the last step.
 */
export function ClusterCreateFlow({ initial }: { initial: ClusterCreateData }) {
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
  const settingsLive = {
    releases: useLive("CamundaRelease", EVERYWHERE, initial.releases),
    platformConfigs: useLive(
      "CamundaPlatformConfig",
      EVERYWHERE,
      initial.platformConfigs,
    ),
    managementAuthConfigs: useLive(
      "ManagementAuthConfig",
      EVERYWHERE,
      initial.managementAuthConfigs,
    ),
  };
  const sources = {
    ElasticsearchClusterPreset: useLive(
      "ElasticsearchClusterPreset",
      EVERYWHERE,
      initial.elasticsearchPresets,
    ),
    DatabaseServerPreset: useLive(
      "DatabaseServerPreset",
      EVERYWHERE,
      initial.databaseServerPresets,
    ),
    SecondaryStorageConfig: useLive(
      "SecondaryStorageConfig",
      EVERYWHERE,
      initial.secondaryStorageConfigs,
    ),
    ObjectStorageConfig: useLive(
      "ObjectStorageConfig",
      EVERYWHERE,
      initial.objectStorageConfigs,
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
    settings: (draft) => [
      ...settingsFailures,
      ...settingsErrors(draft, {
        environment: derive(draft).environment,
        releases,
        platformConfigs,
      }),
      ...editor.shown("spec.cluster"),
    ],
    attachments: (draft) => [
      ...attachmentsFailures,
      ...attachmentsErrors(draft, {
        summary: derive(draft).summary,
        managementAuthConfigs,
      }),
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
  // The claim controller refuses an Optimize URL that a section fills.
  const urlLocked = locks(pathFrom("", OPTIMIZE_URL)) !== undefined;
  const typedUrl = flow.draft.claim.spec.attachments?.optimize?.externalUrl;
  const setClaim = (steps: Step[], value: unknown) =>
    flow.set(["claim"], withClaimValue(flow.draft, steps, value).claim);
  const resetClaim = (steps: Step[]) =>
    flow.set(["claim"], withoutClaimValue(flow.draft, steps).claim);
  useEffect(() => {
    if (urlLocked && typedUrl !== undefined) resetClaim(OPTIMIZE_URL);
  });
  const [secrets, setSecrets] = useState<SecretRefs>([]);
  const [secretsError, setSecretsError] = useState<string>();
  const onSecrets = useCallback((refs: SecretRefs, error?: string) => {
    setSecrets(refs);
    setSecretsError(error);
  }, []);
  const existing =
    flow.draft.namespace?.kind === "existing"
      ? flow.draft.namespace.name
      : undefined;
  const onSection = (
    name: string,
    selections: PluginSelection[],
    errors: ValidationError[],
  ) => {
    flow.set(["storage", name], selections);
    setCardErrors((current) => ({ ...current, [name]: errors }));
  };

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
            fields={fields}
            merged={drafted.merged}
            // A Secret that cannot be read cannot be picked, and the alert
            // above names the failure.
            secrets={
              existing !== undefined && secretsError === undefined
                ? secrets
                : []
            }
            pick={(field, name) => flow.set(["claim", "spec", field], name)}
            set={setClaim}
            reset={resetClaim}
            invalid={editor.invalid}
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
          />
        ) : (
          <div className="flex flex-col items-start gap-4">
            <p className="text-sm text-muted">
              This step comes in a later version of the fleet manager.
            </p>
            {flow.current === "review" && (
              <Button variant="ghost" onClick={flow.back}>
                Back
              </Button>
            )}
          </div>
        )}
      </WizardStepper>
    </div>
  );
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
