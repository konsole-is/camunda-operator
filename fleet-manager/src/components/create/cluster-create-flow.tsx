"use client";

import type { V1Namespace } from "@kubernetes/client-node";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import { useEffect, useState } from "react";
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
import { WizardStepper } from "@/components/fleet/wizard-stepper";
import { hostConflicts, renderHosts, takenHosts } from "@/lib/claims/hosts";
import { claimNameErrors, namesIn } from "@/lib/claims/names";
import { ACCESS_SECTION, SECTION_LABEL } from "@/lib/labels";
import { environmentOf, type PluginSelection } from "@/lib/policy";
import type { ValidationError } from "@/lib/schema";
import { environmentCounts, presetUsage } from "@/lib/usage";
import {
  CLUSTER_STEPS,
  type ClusterDraft,
  basicsErrors,
  offeredPresets,
} from "@/lib/clusters/draft";
import {
  type StorageData,
  clusterDeletion,
  draftAccessPatterns,
  storageErrors,
  storageViews,
} from "@/lib/clusters/storage";
import { ClusterBasicsStep } from "./cluster-basics-step";
import { ClusterStorageStep } from "./cluster-storage-step";
import { type CreateStep, useCreateFlow } from "./use-create-flow";

/**
 * ClusterCreateData is the first snapshot of the create cluster page, from
 * every namespace. accessRequests are the requests of the access section.
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
};

const EVERYWHERE = {};
const ACCESS_REQUESTS = { labelSelector: `${SECTION_LABEL}=${ACCESS_SECTION}` };

const INITIAL: ClusterDraft = { claim: { metadata: { name: "" }, spec: {} } };

/**
 * ClusterCreateFlow is the create cluster wizard. It follows initial live
 * and checks steps 1 and 2 against the newest data. The picks of a section
 * that goes away leave the draft. Steps 3 to 5 hold no fields yet, and the
 * flow writes nothing.
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
    return {
      environment,
      data,
      views,
      presets,
      takenNames,
      hosts,
      conflicts,
      deletion,
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
  } = derive(flow.draft);
  // A claim must not select a section that step 2 no longer shows.
  const gone = Object.keys(flow.draft.storage ?? {}).filter(
    (name) => !views.some((v) => v.name === name),
  );
  useEffect(() => {
    for (const name of gone) flow.set(["storage", name], undefined);
  });
  const onSection = (
    name: string,
    selections: PluginSelection[],
    errors: ValidationError[],
  ) => {
    flow.set(["storage", name], selections);
    setCardErrors((current) => ({ ...current, [name]: errors }));
  };

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
