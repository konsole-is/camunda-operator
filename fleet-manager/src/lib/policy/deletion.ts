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
 * Missing says how checkDeletionPolicy treats a selection whose registration
 * is not in its list, as Missing of pkg/claimpolicy. "Passes" is the rule of
 * a claim that has not recorded its policy, whose reference check refuses
 * such a registration. "Refuses" is the rule of a claim that recorded its
 * policy, whose controller writes the policy before that check.
 */
export type Missing = "Passes" | "Refuses";

/**
 * checkDeletionPolicy refuses with NotAllowed the deletion policy Orphan when
 * the registration of a selection does not list Orphan in
 * spec.deletionPolicies, by the rule of CheckDeletionPolicy of
 * pkg/claimpolicy. Delete always passes. A selection whose registration is
 * not in registrations passes with Passes, and refuses Orphan with Refuses.
 * The value is an empty object when the policy passes.
 */
export function checkDeletionPolicy(
  policy: DeletionPolicy,
  selections: readonly PluginSelection[],
  registrations: readonly PluginRegistration[],
  missing: Missing,
): PolicyResult<Record<string, never>> {
  if (policy !== "Orphan") return { value: {} };

  const unsupported = orphanRefusedBy(selections, registrations);
  const unknown =
    missing === "Refuses"
      ? missingRegistrations(selections, registrations)
      : [];
  const parts = [
    ...(unsupported.length > 0
      ? [`these registrations do not support it: ${unsupported.join(", ")}`]
      : []),
    ...(unknown.length > 0
      ? [
          `the claim cannot tell that these registrations support it, because they do not exist: ${unknown.join(", ")}`,
        ]
      : []),
  ];
  if (parts.length === 0) return { value: {} };

  return {
    refusal: {
      reason: "NotAllowed",
      message: `the deletion policy is Orphan, and ${parts.join(", and ")}`,
    },
  };
}

/**
 * orphanRefusedBy returns each selection whose registration does not list
 * Orphan in spec.deletionPolicies, as PluginRegistration "<registration>"
 * of section "<section>", the names that CheckDeletionPolicy of
 * pkg/claimpolicy gives. A selection whose registration is not in
 * registrations is not in the result: missingRegistrations gives it.
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
    .map(selectionName);
}

/**
 * missingRegistrations returns each selection whose registration is not in
 * registrations, with the names of orphanRefusedBy. A claim that recorded
 * its policy cannot keep while it is not empty.
 */
export function missingRegistrations(
  selections: readonly { section: string; registration: string }[],
  registrations: readonly { metadata?: { name?: string } }[],
): string[] {
  return selections
    .filter(
      (s) => !registrations.some((r) => r.metadata?.name === s.registration),
    )
    .map(selectionName);
}

function selectionName(s: { section: string; registration: string }): string {
  return `PluginRegistration "${s.registration}" of section "${s.section}"`;
}

/**
 * DeletionChoice is the deletion choice of a create flow. defaultPolicy is
 * the default that deletionChoice took. keepAllowed is false when
 * checkDeletionPolicy refuses Orphan for the selections, or while missing
 * is set. alwaysDeleted lists each selection whose registration does not
 * list Orphan, as "<option label> (<section label>)". missing lists each
 * selection whose registration does not exist and that makes keepAllowed
 * false, as "<registration> (<section label>)". It is set only when it is
 * not empty. policy is the policy that the claim gets: the pick, else the
 * default, and Delete when that is Orphan and keepAllowed is false.
 */
export type DeletionChoice = {
  defaultPolicy: DeletionPolicy;
  keepAllowed: boolean;
  alwaysDeleted: { section: string; label: string }[];
  missing?: { section: string; label: string }[];
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
    "value" in
    checkDeletionPolicy("Orphan", selections, registrations, "Passes");
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
 * reads: its reference, its resourceVersion, UID, and generation, its own
 * deletion policy, its selections, and the policy that it recorded when it
 * was made.
 */
export type DeletableClaim = {
  apiVersion: string;
  kind: string;
  metadata: {
    name?: string;
    namespace?: string;
    resourceVersion?: string;
    uid?: string;
    generation?: number;
  };
  spec?: {
    deletionPolicy?: DeletionPolicy | "";
    plugins?: readonly { section: string; registration: string }[];
  };
  status?: { resolvedPolicy?: ResolvedPolicy };
};

/** CLEANUP_FINALIZER is the finalizer of a claim that gives its policy to its requests on delete. */
export const CLEANUP_FINALIZER = "fleet.camunda.io/cleanup";

/** ClaimState is the part of a live claim that the delete waits on. */
export type ClaimState = {
  metadata: {
    uid?: string;
    generation?: number;
    finalizers?: readonly string[];
  };
  status?: {
    observedGeneration?: number;
    resolvedPolicy?: ResolvedPolicy;
    conditions?: readonly { type: string; reason?: string; message?: string }[];
  };
};

/** RequestState is the part of a PluginRequest that the delete waits on. */
export type RequestState = {
  metadata?: {
    name?: string;
    deletionTimestamp?: string;
    ownerReferences?: readonly { controller?: boolean; uid?: string }[];
  };
  spec?: { deletionPolicy?: string };
};

/**
 * claimObserved says that the claim controller read the current generation
 * of claim. A claim with a generation and without status.observedGeneration
 * counts as read only when it has no status.resolvedPolicy.
 */
export function claimObserved(claim: ClaimState): boolean {
  const { generation } = claim.metadata;
  const seen = claim.status?.observedGeneration;
  if (generation === undefined) return true;
  // The pass that records status.resolvedPolicy sets no observedGeneration.
  if (seen === undefined) return claim.status?.resolvedPolicy === undefined;
  return seen >= generation;
}

/**
 * requestsWithout returns the names of the requests that claim controls,
 * that are not being deleted, and that hold another deletion policy than
 * policy. claim must have a UID.
 */
export function requestsWithout(
  claim: ClaimState,
  requests: readonly RequestState[],
  policy: DeletionPolicy,
): string[] {
  return requests
    .filter(
      (r) =>
        // The claim finalizer writes no policy on a request that is already
        // being deleted.
        r.metadata?.deletionTimestamp === undefined &&
        (r.metadata?.ownerReferences ?? []).some(
          (o) => o.controller === true && o.uid === claim.metadata.uid,
        ) &&
        r.spec?.deletionPolicy !== policy,
    )
    .map((r) => r.metadata?.name ?? "");
}

/**
 * orphanNeedsLivePass says that a delete of claim gives Orphan to a request
 * only through a pass of the claim controller on the live claim, which
 * refuses Orphan for a selected registration that does not exist. That is
 * so when the delete waits for the policy, because direct is false or the
 * claim has no cleanup finalizer, and a request that claim controls does
 * not hold Orphan yet. claim must have a UID.
 */
export function orphanNeedsLivePass(
  claim: ClaimState,
  requests: readonly RequestState[],
  direct: boolean,
): boolean {
  const finalized = (claim.metadata.finalizers ?? []).includes(
    CLEANUP_FINALIZER,
  );
  return (
    (!direct || !finalized) &&
    requestsWithout(claim, requests, "Orphan").length > 0
  );
}

/**
 * requestsBehind returns a message that names the requests in names as
 * lacking the deletion policy of claim. names must not be empty.
 */
export function requestsBehind(
  claim: ClaimState,
  names: readonly string[],
): string {
  const ready = claim.status?.conditions?.find((c) => c.type === "Ready");
  const [which, them] =
    names.length === 1
      ? [`The request ${names[0]} does not have`, "it"]
      : [`The requests ${names.join(", ")} do not have`, "them"];
  // The claim controller writes the policy before its other checks. Only
  // its refusal of Orphan stops that write.
  return `${which} the deletion policy of the claim. ${
    ready?.reason === "NotAllowed"
      ? `The operator does not update ${them} while the claim reports NotAllowed: ${ready.message}`
      : `The operator has not updated ${them} yet.`
  }`;
}

/**
 * claimDeletionChoice returns the deletion choice of a live claim of kind.
 * defaultPolicy is the policy that the claim recorded, else Delete, never
 * the current default of environment. policy is the policy of the claim,
 * else that default. Unlike deletionChoice, policy stays Orphan when the
 * claim keeps and an option cannot: keepAllowed is then false. A claim that
 * recorded its policy cannot keep either while a selected registration is
 * not in registrations and livePass says that Orphan reaches the requests
 * through a live pass of its controller, which refuses it. A registration
 * is never missing while registrationsRead is false. The labels of
 * alwaysDeleted and missing come from the sections as a claim of kind sees
 * them.
 */
export function claimDeletionChoice(input: {
  claim: DeletableClaim & { spec?: { plugins?: PluginSelection[] } };
  environment: Environment | undefined;
  sections: readonly PluginSection[];
  registrations: readonly PluginRegistration[];
  registrationsRead: boolean;
  livePass: boolean;
  kind: "CamundaClusterClaim" | "CamundaManagementClusterClaim";
}): DeletionChoice {
  const { claim, environment, sections, registrations, kind } = input;
  const views = sections.map((s) =>
    offeredSection(s, registrations, environment, kind),
  );
  const resolved = claim.status?.resolvedPolicy;
  const selections = claim.spec?.plugins ?? [];
  const choice = deletionChoice(
    claim.spec?.deletionPolicy || undefined,
    selections,
    environment,
    registrations,
    views,
    deletionPolicyOf(undefined, resolved),
  );
  const missing =
    resolved !== undefined && input.registrationsRead && input.livePass
      ? selections
          .filter(
            (s) =>
              !registrations.some((r) => r.metadata.name === s.registration),
          )
          .map((s) => ({
            section: s.section,
            label: `${s.registration} (${views.find((v) => v.name === s.section)?.label ?? s.section})`,
          }))
      : [];
  return {
    ...choice,
    keepAllowed: choice.keepAllowed && missing.length === 0,
    ...(missing.length > 0 && { missing }),
    policy: deletionPolicyOf(claim.spec?.deletionPolicy, resolved),
  };
}
