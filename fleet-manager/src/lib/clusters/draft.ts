// This module runs in the browser and on the server.
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import type { Environment } from "@/generated/crds/environment";
import type {
  NamespaceChoice,
  PickerEnvironment,
} from "@/components/fleet/namespace-picker";
import { claimNameErrors } from "@/lib/claims/names";
import { environmentReady } from "@/lib/environments";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import { byCodeUnit, type PluginSelection } from "@/lib/policy";
import type { DeletionChoice } from "./storage";

/** ClusterClaimSpec is the spec of a cluster claim with typed plugin parameters. */
export type ClusterClaimSpec = Omit<CamundaClusterClaim["spec"], "plugins"> & {
  plugins?: PluginSelection[];
};

/**
 * ClusterDraft is the draft of the cluster create flow. environment stays
 * unset while no Environment exists. storage holds the selections of each
 * section by section name, as step 2 last reported them. A section that
 * has a key was shown; an empty list is a picked empty option.
 */
export type ClusterDraft = {
  environment?: string;
  namespace?: NamespaceChoice;
  claim: {
    metadata: { name: string };
    spec: Partial<Omit<ClusterClaimSpec, "plugins">>;
  };
  storage?: Record<string, PluginSelection[]>;
};

/** ClusterClaimObject is the claim that a create writes. */
export type ClusterClaimObject = {
  apiVersion: "fleet.camunda.io/v1alpha1";
  kind: "CamundaClusterClaim";
  metadata: { name: string; namespace: string };
  spec: ClusterClaimSpec;
};

/** CLUSTER_STEPS are the steps of the cluster create flow, in order. */
export const CLUSTER_STEPS: readonly { id: string; label: string }[] = [
  { id: "basics", label: "Environment and name" },
  { id: "storage", label: "Storage and access" },
  { id: "settings", label: "Cluster settings" },
  { id: "attachments", label: "Attachments" },
  { id: "review", label: "Review" },
];

/**
 * BasicsContext is the live data that step 1 checks a draft against.
 * namespaces are the live namespaces, each with the name of its
 * Environment. takenNames are the names of the claims and clusters in the
 * namespace of the draft. conflicts are the messages of the hosts that
 * another claim holds. presets are the names of the presets that the step
 * offers.
 */
export type BasicsContext = {
  environments: readonly Environment[];
  namespaces: readonly { name: string; environment?: string }[];
  takenNames: readonly string[];
  conflicts: readonly string[];
  presets: readonly string[];
};

/**
 * basicsErrors returns the errors of step 1 for draft, in screen order: the
 * environment (only while an Environment exists), the name, the hosts, the
 * namespace, and the preset. A picked environment that is gone counts as
 * none, and one that turned Ready=False refuses the step with its message.
 * A picked namespace that the live data no longer allows is refused.
 */
export function basicsErrors(
  draft: ClusterDraft,
  context: BasicsContext,
): string[] {
  const errors: string[] = [];
  if (context.environments.length > 0) {
    const environment = context.environments.find(
      (e) => e.metadata.name === draft.environment,
    );
    if (!environment) {
      errors.push("Choose an environment");
    } else {
      const { ready, message } = environmentReady(environment);
      if (!ready) {
        errors.push(
          `The environment ${environment.spec.displayName || draft.environment} is not ready${message ? `: ${message}` : ""}`,
        );
      }
    }
  }
  errors.push(
    ...claimNameErrors(draft.claim.metadata.name, context.takenNames),
    ...context.conflicts,
  );
  errors.push(...namespaceErrors(draft, context));
  const preset = draft.claim.spec.presetRef;
  if (!preset || !context.presets.includes(preset)) {
    errors.push("Choose a preset");
  }
  return errors;
}

function namespaceErrors(
  draft: ClusterDraft,
  context: BasicsContext,
): string[] {
  const choice = draft.namespace;
  if (!choice) return ["Choose a namespace"];
  const live = context.namespaces.find((ns) => ns.name === choice.name);
  if (choice.kind === "new") {
    const environment = context.environments.find(
      (e) => e.metadata.name === draft.environment,
    );
    const creation = environment?.spec.namespaceCreation;
    if (!creation?.allowed) return ["Choose a namespace"];
    const missing = (creation.requiredLabelKeys ?? []).filter(
      (key) =>
        !Object.hasOwn(creation.labels ?? {}, key) && !choice.labels[key],
    );
    return [
      ...(live ? [`The namespace ${choice.name} already exists`] : []),
      ...missing.map((key) => `Enter a value for ${key}`),
    ];
  }
  const inEnvironment =
    context.environments.length === 0 ||
    live?.environment === draft.environment;
  return live && inEnvironment ? [] : ["Choose a namespace"];
}

/**
 * offeredPresets returns the presets that a claim can use: with an
 * environment, the presets of spec.presets.camundaCluster in that order,
 * skipping a name that no preset has; without one, every preset by name.
 * This is the rule of checkAllowed.
 */
export function offeredPresets(
  presets: readonly CamundaClusterPreset[],
  environment: Environment | undefined,
): CamundaClusterPreset[] {
  if (!environment) {
    return [...presets].sort((a, b) =>
      byCodeUnit(a.metadata.name ?? "", b.metadata.name ?? ""),
    );
  }
  return (environment.spec.presets?.camundaCluster ?? []).flatMap((name) =>
    presets.filter((p) => p.metadata.name === name).slice(0, 1),
  );
}

/** draftSelections returns the selections of draft.storage, section after section, in the order the step reported them. */
export function draftSelections(draft: ClusterDraft): PluginSelection[] {
  return Object.values(draft.storage ?? {}).flat();
}

/**
 * clusterClaimOf returns the claim that draft writes. releaseRef is always
 * set when one is known: the pick of the draft, else the default release
 * of environment. platformConfigRef is set only when the pick differs from
 * spec.platformConfigRef of environment. plugins are draftSelections, left
 * out when empty. cluster and attachments are copied when the draft sets
 * them. deletionPolicy is deletion.policy, set only when it differs from
 * deletion.defaultPolicy. The deletion pick of draft counts only through
 * deletion.
 */
export function clusterClaimOf(
  draft: ClusterDraft,
  environment: Environment | undefined,
  deletion: DeletionChoice,
): ClusterClaimObject {
  const { spec } = draft.claim;
  const releaseRef = spec.releaseRef ?? environment?.spec.releases?.default;
  const platformConfigRef =
    spec.platformConfigRef !== environment?.spec.platformConfigRef
      ? spec.platformConfigRef
      : undefined;
  const deletionPolicy =
    deletion.policy !== deletion.defaultPolicy ? deletion.policy : undefined;
  const plugins = draftSelections(draft);
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: {
      name: draft.claim.metadata.name,
      namespace: draft.namespace?.name ?? "",
    },
    spec: {
      presetRef: spec.presetRef ?? "",
      ...(releaseRef !== undefined && { releaseRef }),
      ...(platformConfigRef !== undefined && { platformConfigRef }),
      ...(deletionPolicy !== undefined && { deletionPolicy }),
      ...(spec.cluster && { cluster: spec.cluster }),
      ...(spec.attachments && { attachments: spec.attachments }),
      ...(plugins.length > 0 && { plugins }),
    },
  };
}

/**
 * syncedNamespace returns choice with the labels that a new namespace gets
 * under the current rules of environment: its fixed labels, the values of
 * its required label keys from choice, and the environment label. It
 * returns choice itself when nothing changes, and for an existing
 * namespace.
 */
export function syncedNamespace(
  choice: NamespaceChoice | undefined,
  environment: Environment | undefined,
): NamespaceChoice | undefined {
  if (choice?.kind !== "new" || !environment) return choice;
  const creation = environment.spec.namespaceCreation;
  const fixed = creation?.labels ?? {};
  const asked = (creation?.requiredLabelKeys ?? []).filter(
    (key) => !Object.hasOwn(fixed, key) && Object.hasOwn(choice.labels, key),
  );
  const labels = {
    ...fixed,
    ...Object.fromEntries(asked.map((key) => [key, choice.labels[key]])),
    [ENVIRONMENT_LABEL]: environment.metadata.name ?? "",
  };
  return JSON.stringify(sorted(labels)) ===
    JSON.stringify(sorted(choice.labels))
    ? choice
    : { ...choice, labels };
}

function sorted(labels: Record<string, string>): [string, string][] {
  return Object.entries(labels).sort(([a], [b]) => byCodeUnit(a, b));
}

/** pickerEnvironment returns the part of environment that NamespacePicker reads. */
export function pickerEnvironment(environment: Environment): PickerEnvironment {
  return {
    name: environment.metadata.name ?? "",
    ...(environment.spec.displayName && {
      displayName: environment.spec.displayName,
    }),
    ...(environment.spec.namespaceCreation && {
      namespaceCreation: environment.spec.namespaceCreation,
    }),
  };
}
