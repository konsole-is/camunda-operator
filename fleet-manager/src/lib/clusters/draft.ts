// This module runs in the browser and on the server.
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import type { Environment } from "@/generated/crds/environment";
import type {
  NamespaceChoice,
  PickerEnvironment,
} from "@/components/fleet/namespace-picker";
import { claimNameErrors } from "@/lib/claims/names";
import { environmentReady } from "@/lib/environments";
import {
  type ClusterField,
  lockLabel,
  pathFrom,
  type Step,
} from "@/lib/fields";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL, ENVIRONMENT_LABEL } from "@/lib/labels";
import { mergeCluster, type Merged } from "@/lib/merge";
import {
  byCodeUnit,
  type DeletionChoice,
  type PluginSelection,
  releaseAllowed,
} from "@/lib/policy";
import type { FieldError } from "@/lib/session/cluster";
import { filledClaimSteps } from "@/lib/session/locks";
import { valueAt, without, withValue } from "@/lib/session/steps";
import { createChangeOf } from "@/lib/write/create";
import type { Change } from "@/lib/write/types";
import {
  MANAGEMENT_AUTH_CONFIGS_UNREAD,
  optimizeBlocked,
  SCHEDULE_NEEDS_BACKUP_STORAGE,
  scheduleBlocked,
} from "./attachments";
import type { StorageSummary } from "./storage";

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

/** releaseOf returns the release of draft: its pick, else the default release of environment, else undefined. */
export function releaseOf(
  draft: ClusterDraft,
  environment: Environment | undefined,
): string | undefined {
  return draft.claim.spec.releaseRef ?? environment?.spec.releases?.default;
}

/**
 * clusterClaimOf returns the claim that draft writes. releaseRef is
 * releaseOf, set when one is known. platformConfigRef is set only when the
 * pick differs from spec.platformConfigRef of environment. plugins are
 * draftSelections, left out when empty. cluster and attachments are copied
 * when the draft sets them. deletionPolicy is deletion.policy, set only when
 * it differs from deletion.defaultPolicy. The deletion pick of draft counts
 * only through deletion.
 */
export function clusterClaimOf(
  draft: ClusterDraft,
  environment: Environment | undefined,
  deletion: DeletionChoice,
): ClusterClaimObject {
  const { spec } = draft.claim;
  const releaseRef = releaseOf(draft, environment);
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
 * clusterCreateChange returns the change that creates the cluster of
 * draft: the new namespace first, with the labels of syncedNamespace, then
 * the claim of clusterClaimOf. Without an Environment the new namespace
 * keeps the labels of the draft.
 */
export function clusterCreateChange(
  draft: ClusterDraft,
  environment: Environment | undefined,
  deletion: DeletionChoice,
): Change {
  const claim = clusterClaimOf(draft, environment, deletion);
  const { name, namespace } = claim.metadata;
  const choice = syncedNamespace(draft.namespace, environment);
  return createChangeOf(
    `Create the cluster ${name} in ${namespace}`,
    claim,
    choice?.kind === "new"
      ? { name: choice.name, labels: choice.labels }
      : undefined,
  );
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

/** DraftedCluster is what draftCluster returns. */
export type DraftedCluster = {
  cluster: Resource<"CamundaCluster">;
  claim: ClusterClaimObject;
  merged: Merged<CamundaClusterSpec>;
};

// The write takes the deletion choice of step 2, so the draft claim of the
// field states keeps the default policy.
const DEFAULT_DELETION: DeletionChoice = {
  defaultPolicy: "Delete",
  policy: "Delete",
  keepAllowed: true,
  alwaysDeleted: [],
};

/**
 * draftCluster returns the CamundaCluster that the claim of draft would
 * make, for the field states of step 3. cluster carries CLAIM_LABEL with
 * the claim name, so that a selected section locks the fields that it
 * fills. Its spec is spec.cluster of the draft with presetRef, releaseOf,
 * and the platform config (the pick, else the Environment default). merged
 * is mergeCluster of the preset and the release under that spec, without a
 * preset or a release that the lists do not hold. claim is clusterClaimOf
 * with no deletion policy.
 */
export function draftCluster(
  draft: ClusterDraft,
  presets: readonly Resource<"CamundaClusterPreset">[],
  releases: readonly Resource<"CamundaRelease">[],
  environment: Environment | undefined,
): DraftedCluster {
  const { spec } = draft.claim;
  const releaseRef = releaseOf(draft, environment);
  const platformConfigRef =
    spec.platformConfigRef ?? environment?.spec.platformConfigRef;
  const clusterSpec: CamundaClusterSpec = {
    ...structuredClone(spec.cluster ?? {}),
    ...(spec.presetRef && { presetRef: spec.presetRef }),
    ...(releaseRef && { releaseRef }),
    ...(platformConfigRef && { platformConfigRef }),
  };
  const cluster = {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaCluster",
    metadata: {
      name: draft.claim.metadata.name,
      namespace: draft.namespace?.name ?? "",
      labels: { [CLAIM_LABEL]: draft.claim.metadata.name },
    },
    spec: clusterSpec,
  } as Resource<"CamundaCluster">;
  const preset = presets.find((p) => p.metadata?.name === spec.presetRef);
  const release = releases.find((r) => r.metadata?.name === releaseRef);
  return {
    cluster,
    claim: clusterClaimOf(draft, environment, DEFAULT_DELETION),
    merged: mergeCluster(preset?.spec, release?.spec, clusterSpec),
  };
}

/**
 * draftOptimize returns the empty CamundaOptimize that the claim of draft
 * would make while it attaches Optimize, else undefined. clusterFields
 * gives the fields of an attachment, with the locks of the sections that
 * fill them, only beside its object.
 */
export function draftOptimize(
  draft: ClusterDraft,
): Resource<"CamundaOptimize"> | undefined {
  if (!draft.claim.spec.attachments?.optimize) return undefined;
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaOptimize",
    metadata: {
      name: draft.claim.metadata.name,
      namespace: draft.namespace?.name ?? "",
    },
    spec: {},
  } as unknown as Resource<"CamundaOptimize">;
}

/**
 * SettingsData is the live data that step 3 checks a draft against.
 * fields are clusterFields of the draft cluster. unread are the kinds that
 * could not be read.
 */
export type SettingsData = {
  environment: Environment | undefined;
  releases: readonly Resource<"CamundaRelease">[];
  platformConfigs: readonly Resource<"CamundaPlatformConfig">[];
  fields: readonly ClusterField[];
  unread?: readonly KindName[];
};

/**
 * settingsErrors returns the errors of step 3. A claim needs a release,
 * because neither a claim nor a preset can set the version. A release or a
 * platform config must exist, and the Environment must allow a picked
 * release. It does not say that one of an unread kind is missing.
 */
export function settingsErrors(
  draft: ClusterDraft,
  data: SettingsData,
): string[] {
  const { spec } = draft.claim;
  const errors: string[] = [];
  const unread = (kind: KindName) => data.unread?.includes(kind) ?? false;
  const release = releaseOf(draft, data.environment);
  if (!release) {
    errors.push("Pick a release.");
  } else if (
    !unread("CamundaRelease") &&
    !data.releases.some((r) => r.metadata?.name === release)
  ) {
    errors.push(
      spec.releaseRef
        ? `Release ${release} does not exist. Pick another one.`
        : `Release ${release} of the environment does not exist. Pick another one.`,
    );
  } else if (spec.releaseRef && !releaseAllowed(release, data.environment)) {
    errors.push(
      `The environment does not allow release ${release}. Pick another one.`,
    );
  }
  const platform =
    spec.platformConfigRef ?? data.environment?.spec.platformConfigRef;
  if (!platform) {
    errors.push("Pick a platform config.");
  } else if (
    !unread("CamundaPlatformConfig") &&
    !data.platformConfigs.some((p) => p.metadata?.name === platform)
  ) {
    errors.push(
      spec.platformConfigRef
        ? `Platform config ${platform} does not exist. Pick another one.`
        : `Platform config ${platform} of the environment does not exist. Pick another one.`,
    );
  }
  // The claim controller refuses a claim value that a section fills.
  for (const field of data.fields) {
    const [steps] = filledClaimSteps([field]);
    if (
      field.target.kind === "CamundaCluster" &&
      field.lock &&
      steps &&
      valueAt(draft.claim, steps) !== undefined
    )
      errors.push(
        `${field.label}: ${lockLabel(field.lock)}, so the claim cannot set it.`,
      );
  }
  return errors;
}

/**
 * AttachmentsData is the live data that step 4 checks a draft against.
 * summary is storageSummary of the draft. managementAuthConfigs are names.
 * unread are the kinds that could not be read.
 */
export type AttachmentsData = {
  summary: StorageSummary;
  managementAuthConfigs: readonly string[];
  unread?: readonly KindName[];
};

/**
 * attachmentsErrors returns the errors of step 4: an Optimize attachment
 * that cannot attach or names a ManagementAuthConfig that does not exist,
 * then a backup schedule without backup storage or with no schedule.
 * While ManagementAuthConfig is unread, it gives no error about the
 * ManagementAuthConfigs.
 */
export function attachmentsErrors(
  draft: ClusterDraft,
  data: AttachmentsData,
): string[] {
  const { optimize, backupSchedule } = draft.claim.spec.attachments ?? {};
  const errors: string[] = [];
  const authsUnread = data.unread?.includes("ManagementAuthConfig") ?? false;
  if (optimize) {
    const blocked = optimizeBlocked(
      data.summary.secondaryStorageType,
      data.managementAuthConfigs,
      authsUnread,
    );
    if (blocked === undefined) {
      if (
        !data.managementAuthConfigs.includes(optimize.managementAuthConfigRef)
      )
        errors.push(
          `ManagementAuthConfig ${optimize.managementAuthConfigRef} does not exist. Pick another one.`,
        );
    } else if (blocked !== MANAGEMENT_AUTH_CONFIGS_UNREAD) errors.push(blocked);
  }
  if (backupSchedule) {
    if (scheduleBlocked(data.summary.backupStorage !== undefined))
      errors.push(`Back up on a schedule: ${SCHEDULE_NEEDS_BACKUP_STORAGE}`);
    else if (!backupSchedule.schedule?.trim())
      errors.push("Schedule (cron, UTC): Enter a schedule.");
  }
  return errors;
}

/** withClaimValue returns draft with value at steps of its claim, steps from the root of the claim. */
export function withClaimValue(
  draft: ClusterDraft,
  steps: readonly Step[],
  value: unknown,
): ClusterDraft {
  return { ...draft, claim: withValue(draft.claim, steps, value) };
}

/** withoutClaimValues returns draft without the value at each of paths of its claim, or draft itself when it holds none of them. */
export function withoutClaimValues(
  draft: ClusterDraft,
  paths: readonly (readonly Step[])[],
): ClusterDraft {
  return paths
    .filter((steps) => valueAt(draft.claim, steps) !== undefined)
    .reduce(withoutClaimValue, draft);
}

/** withoutClaimValue returns draft without the value at steps of its claim. Objects that become empty go, but the claim keeps a spec. */
export function withoutClaimValue(
  draft: ClusterDraft,
  steps: readonly Step[],
): ClusterDraft {
  const claim = without(draft.claim, steps);
  return { ...draft, claim: { ...claim, spec: claim.spec ?? {} } };
}

// CLAIM_DEFAULTS holds each default of the CRD of the claim. The API server
// adds one to an object that a write sent without it.
const CLAIM_DEFAULTS: Record<string, unknown> = {
  "secretKeyRef.name": "",
  "configMapKeyRef.name": "",
  "secretRef.name": "",
  "configMapRef.name": "",
  "fileKeyRef.optional": false,
  "retained.completed": 7,
  "retained.failed": 3,
  "persistentVolumeClaimRetentionPolicy.whenDeleted": "Delete",
};

// The clocks of the browser and of the API server can differ by this much.
const CLOCK_ALLOWANCE_MS = 60_000;

/**
 * isWrittenClaim says whether live is the claim that a write sent at
 * startedAt, a time in milliseconds of the browser: it was made after the
 * write started, and holdsWrittenSpec holds for its spec.
 */
export function isWrittenClaim(
  live: { metadata?: { creationTimestamp?: string }; spec?: unknown },
  sent: unknown,
  startedAt: number,
): boolean {
  const created = Date.parse(live.metadata?.creationTimestamp ?? "");
  return (
    created >= startedAt - CLOCK_ALLOWANCE_MS &&
    holdsWrittenSpec(live.spec, sent)
  );
}

/**
 * holdsWrittenSpec says whether live, the spec of a live claim, is the spec
 * sent with the defaults that the API server adds, and nothing else.
 */
export function holdsWrittenSpec(live: unknown, sent: unknown): boolean {
  return holdsSent(live, sent, "spec");
}

function holdsSent(live: unknown, sent: unknown, parent: string): boolean {
  if (Array.isArray(sent))
    return (
      Array.isArray(live) &&
      live.length === sent.length &&
      sent.every((item, i) => holdsSent(live[i], item, parent))
    );
  if (typeof sent !== "object" || sent === null) return live === sent;
  if (typeof live !== "object" || live === null || Array.isArray(live))
    return false;
  const fields = sent as Record<string, unknown>;
  // The parameters of a selection are free JSON and get no default.
  const defaults = parent === "plugins" ? {} : CLAIM_DEFAULTS;
  return (
    Object.entries(live).every(([key, value]) =>
      Object.hasOwn(fields, key)
        ? holdsSent(value, fields[key], parent === "plugins" ? parent : key)
        : Object.hasOwn(defaults, `${parent}.${key}`) &&
          value === defaults[`${parent}.${key}`],
    ) && Object.keys(fields).every((key) => Object.hasOwn(live, key))
  );
}

/**
 * SecretReference is one Secret reference of a draft claim. path is the
 * path under spec.cluster, or under spec and starting with "attachments."
 * for an attachment. steps lead from the root of the claim to it.
 */
export type SecretReference = { path: string; steps: Step[] };

/**
 * secretReferences returns each Secret reference in spec.cluster and
 * spec.attachments of the draft claim, in document order: a secretKeyRef or
 * a secretRef, and an object whose name ends in Ref with exactly the
 * fields name and key, other than a configMapKeyRef.
 */
export function secretReferences(draft: ClusterDraft): SecretReference[] {
  const found: SecretReference[] = [];
  const walk = (value: unknown, steps: Step[], root: string, from: number) => {
    const entries: [Step, unknown][] = Array.isArray(value)
      ? value.map((item, index) => [{ index }, item])
      : typeof value === "object" && value !== null
        ? Object.entries(value).map(([field, item]) => [{ field }, item])
        : [];
    for (const [step, item] of entries) {
      const at = [...steps, step];
      if ("field" in step && isSecretReference(step.field, item))
        found.push({ path: pathFrom(root, at.slice(from)), steps: at });
      else walk(item, at, root, from);
    }
  };
  const spec = draft.claim.spec;
  walk(spec.cluster, [{ field: "spec" }, { field: "cluster" }], "", 2);
  walk(
    spec.attachments,
    [{ field: "spec" }, { field: "attachments" }],
    "attachments",
    2,
  );
  return found;
}

function isSecretReference(field: string, value: unknown): boolean {
  if (typeof value !== "object" || value === null || Array.isArray(value))
    return false;
  if (field === "secretKeyRef" || field === "secretRef") return true;
  const keys = Object.keys(value).sort();
  return (
    field.endsWith("Ref") &&
    field !== "configMapKeyRef" &&
    keys.join() === "key,name"
  );
}

/**
 * SecretsRead is the last answer of the read of the Secrets of namespace:
 * the name and the keys of each Secret, or the error of the read.
 */
export type SecretsRead = {
  namespace: string;
  secrets: { name: string; keys: string[] }[];
  error?: string;
};

/**
 * secretErrors returns one error for each Secret reference of draft whose
 * Secret, or key, the namespace of draft does not hold. A new namespace
 * holds no Secret. read must be the answer for the namespace of draft: for
 * an existing namespace, without it or with its error, each reference is
 * an error that says it cannot be checked. An optional reference is never
 * an error.
 */
export function secretErrors(
  draft: ClusterDraft,
  read: SecretsRead | undefined,
): string[] {
  return secretChecks(draft, read).map(
    ({ reference, message }) => `${reference.path}: ${message}`,
  );
}

/**
 * secretFieldErrors returns the errors of secretErrors as field errors,
 * each at the path of its reference from the root of the claim.
 */
export function secretFieldErrors(
  draft: ClusterDraft,
  read: SecretsRead | undefined,
): FieldError[] {
  return secretChecks(draft, read).map(({ reference, message }) => ({
    path: pathFrom("", reference.steps),
    message,
  }));
}

function secretChecks(
  draft: ClusterDraft,
  read: SecretsRead | undefined,
): { reference: SecretReference; message: string }[] {
  const namespace = draft.namespace;
  if (namespace === undefined) return [];
  return secretReferences(draft).flatMap((reference) => {
    const { name, key, optional } = valueAt(draft.claim, reference.steps) as {
      name?: string;
      key?: string;
      optional?: boolean;
    };
    // The pod starts without a Secret or a key that is optional.
    if (optional === true) return [];
    const ns = namespace.name;
    let error: string | undefined;
    if (namespace.kind === "new")
      error = `The new namespace ${ns} holds no Secret yet. Reset the field.`;
    else if (read?.namespace !== ns)
      error = `The Secrets of ${ns} are not read yet, so the fleet manager cannot check this reference.`;
    else if (read.error !== undefined)
      error = `The fleet manager could not read the Secrets of ${ns}, so it cannot check this reference: ${read.error}`;
    else {
      const secret = read.secrets.find((s) => s.name === name);
      if (secret === undefined)
        error = `The namespace ${ns} holds no Secret ${name}. Pick a Secret of ${ns}, or reset the field.`;
      else if (key !== undefined && !secret.keys.includes(key))
        error = `The Secret ${name} of ${ns} holds no key ${key}. Pick a key of it, or reset the field.`;
    }
    return error === undefined ? [] : [{ reference, message: error }];
  });
}
