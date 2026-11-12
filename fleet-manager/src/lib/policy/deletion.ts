import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSelection, PolicyResult, SectionView } from "./types";

/** DeletionPolicy is a deletion policy of a claim or of a request. */
export type DeletionPolicy =
  PluginRegistration["spec"]["deletionPolicies"][number];

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

  const unsupported = selections.filter((s) => {
    const registration = registrations.find(
      (r) => r.metadata.name === s.registration,
    );
    return (
      registration !== undefined &&
      !registration.spec.deletionPolicies.includes(policy)
    );
  });
  if (unsupported.length === 0) return { value: {} };

  const names = unsupported
    .map(
      (s) => `PluginRegistration "${s.registration}" of section "${s.section}"`,
    )
    .join(", ");
  return {
    refusal: {
      reason: "NotAllowed",
      message: `the deletion policy is Orphan, and these registrations do not support it: ${names}`,
    },
  };
}

/**
 * DeletionChoice is the deletion choice of a create flow. defaultPolicy is
 * spec.deletionPolicy of the Environment, Delete when unset or without an
 * Environment. keepAllowed is false when checkDeletionPolicy refuses Orphan
 * for the selections. alwaysDeleted lists each selection whose registration
 * does not list Orphan, as "<option label> (<section label>)". policy is
 * the policy that the claim gets: the pick, else the default, and Delete
 * when that is Orphan and keepAllowed is false.
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
 * registration and the name of its section.
 */
export function deletionChoice(
  picked: DeletionPolicy | undefined,
  selections: readonly PluginSelection[],
  environment: Environment | undefined,
  registrations: readonly PluginRegistration[],
  views: readonly SectionView[] = [],
): DeletionChoice {
  const defaultPolicy = environment?.spec.deletionPolicy || "Delete";
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
