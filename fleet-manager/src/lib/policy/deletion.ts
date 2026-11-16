import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { offeredSection } from "./options";
import type { ResolvedPolicy } from "./resolve";
import type { PluginSelection, PolicyResult, SectionView } from "./types";

/** DeletionPolicy is a deletion policy of a claim or of a request. */
export type DeletionPolicy =
  PluginRegistration["spec"]["deletionPolicies"][number];

/**
 * deletionPolicyOf returns the deletion policy that the requests of a claim
 * get, by the rule of DeletionPolicy of pkg/claimpolicy: own, the
 * spec.deletionPolicy of the claim, when it is set, else the default of
 * resolved, its status.resolvedPolicy, else Delete. A claim falls back to
 * Delete, not to the current default of its Environment.
 */
export function deletionPolicyOf(
  own: DeletionPolicy | "" | undefined,
  resolved: ResolvedPolicy | undefined,
): DeletionPolicy {
  return own || resolved?.deletionPolicy || "Delete";
}

/**
 * checkDeletionPolicy refuses with NotAllowed the deletion policy Orphan when
 * the registration of a selection does not list Orphan in
 * spec.deletionPolicies, by the rule of CheckDeletionPolicy of
 * pkg/claimpolicy. Delete always passes, and so does a selection whose
 * registration is not in registrations. The value is an empty object when
 * the policy passes.
 */
export function checkDeletionPolicy(
  policy: DeletionPolicy,
  selections: readonly PluginSelection[],
  registrations: readonly PluginRegistration[],
): PolicyResult<Record<string, never>> {
  if (policy !== "Orphan") return { value: {} };

  const unsupported = orphanRefusedBy(selections, registrations);
  if (unsupported.length === 0) return { value: {} };

  return {
    refusal: {
      reason: "NotAllowed",
      message: `the deletion policy is Orphan, and these registrations do not support it: ${unsupported.join(", ")}`,
    },
  };
}

/**
 * orphanRefusedBy returns each selection whose registration does not list
 * Orphan in spec.deletionPolicies, as PluginRegistration "<registration>"
 * of section "<section>", the names that CheckDeletionPolicy of
 * pkg/claimpolicy gives. A selection whose registration is not in
 * registrations passes. It is empty when the claim can keep.
 */
export function orphanRefusedBy(
  selections: readonly { section: string; registration: string }[],
  registrations: readonly {
    metadata?: { name?: string };
    spec: { deletionPolicies: readonly string[] };
  }[],
): string[] {
  return selections
    .filter((s) => {
      const registration = registrations.find(
        (r) => r.metadata?.name === s.registration,
      );
      return (
        registration !== undefined &&
        !registration.spec.deletionPolicies.includes("Orphan")
      );
    })
    .map(
      (s) => `PluginRegistration "${s.registration}" of section "${s.section}"`,
    );
}

/**
 * DeletionChoice is the deletion choice of a create flow. defaultPolicy is
 * the default that deletionChoice took. keepAllowed is false when
 * checkDeletionPolicy refuses Orphan for the selections. alwaysDeleted lists
 * each selection whose registration does not list Orphan, as "<option
 * label> (<section label>)". policy is the policy that the claim gets: the
 * pick, else the default, and Delete when that is Orphan and keepAllowed is
 * false.
 */
export type DeletionChoice = {
  defaultPolicy: DeletionPolicy;
  keepAllowed: boolean;
  alwaysDeleted: { section: string; label: string }[];
  policy: DeletionPolicy;
};

/**
 * deletionChoice returns the deletion choice of a claim with selections
 * and the deletion pick picked under environment. views give the labels of
 * alwaysDeleted. A selection outside views shows by the label of its
 * registration and the name of its section. defaultPolicy replaces the
 * default of environment, Delete when unset or without an Environment, as
 * for a claim whose resolved policy records a deletion policy.
 */
export function deletionChoice(
  picked: DeletionPolicy | undefined,
  selections: readonly PluginSelection[],
  environment: Environment | undefined,
  registrations: readonly PluginRegistration[],
  views: readonly SectionView[] = [],
  defaultPolicy: DeletionPolicy = environment?.spec.deletionPolicy || "Delete",
): DeletionChoice {
  const keepAllowed =
    "value" in checkDeletionPolicy("Orphan", selections, registrations);
  const alwaysDeleted = selections.flatMap((s) => {
    const registration = registrations.find(
      (r) => r.metadata.name === s.registration,
    );
    if (
      !registration ||
      registration.spec.deletionPolicies.includes("Orphan")
    ) {
      return [];
    }
    const view = views.find((v) => v.name === s.section);
    const option =
      view?.options.find((o) => o.registration === s.registration)?.label ??
      registration.spec.label;
    return [
      { section: s.section, label: `${option} (${view?.label ?? s.section})` },
    ];
  });
  const policy = picked ?? defaultPolicy;
  return {
    defaultPolicy,
    keepAllowed,
    alwaysDeleted,
    policy: policy === "Orphan" && !keepAllowed ? "Delete" : policy,
  };
}

/**
 * DeletableClaim is the part of a live claim of either kind that a delete
 * reads: its reference, its resourceVersion and UID, its own deletion
 * policy, its selections, and the policy that it recorded when it was made.
 */
export type DeletableClaim = {
  apiVersion: string;
  kind: string;
  metadata: {
    name?: string;
    namespace?: string;
    resourceVersion?: string;
    uid?: string;
  };
  spec?: {
    deletionPolicy?: DeletionPolicy | "";
    plugins?: readonly { section: string; registration: string }[];
  };
  status?: { resolvedPolicy?: ResolvedPolicy };
};

/**
 * claimDeletionChoice returns the deletion choice of a live claim of kind.
 * defaultPolicy is the policy that the claim recorded, else Delete, never
 * the current default of environment. policy is the policy of the claim,
 * else that default. Unlike deletionChoice, policy stays Orphan when the
 * claim keeps and an option cannot: keepAllowed is then false. The labels
 * of alwaysDeleted come from the sections as a claim of kind sees them.
 */
export function claimDeletionChoice(input: {
  claim: DeletableClaim & { spec?: { plugins?: PluginSelection[] } };
  environment: Environment | undefined;
  sections: readonly PluginSection[];
  registrations: readonly PluginRegistration[];
  kind: "CamundaClusterClaim" | "CamundaManagementClusterClaim";
}): DeletionChoice {
  const { claim, environment, sections, registrations, kind } = input;
  const views = sections.map((s) =>
    offeredSection(s, registrations, environment, kind),
  );
  const resolved = claim.status?.resolvedPolicy;
  const choice = deletionChoice(
    claim.spec?.deletionPolicy || undefined,
    claim.spec?.plugins ?? [],
    environment,
    registrations,
    views,
    deletionPolicyOf(undefined, resolved),
  );
  return {
    ...choice,
    policy: deletionPolicyOf(claim.spec?.deletionPolicy, resolved),
  };
}
