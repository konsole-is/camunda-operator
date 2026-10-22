import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSelection, PolicyResult } from "./types";

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
