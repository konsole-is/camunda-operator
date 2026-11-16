// This module runs in the browser and on the server.
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { Resource } from "@/lib/k8s/kinds";
import {
  type DeletableClaim,
  deletionPolicyOf,
  type DeletionPolicy,
  type Slot,
  slotOf,
} from "@/lib/policy";
import { changeOf, type Session, setValue } from "@/lib/session";
import { refKey } from "@/lib/write/refs";
import type { Change } from "@/lib/write/types";
import { resourceFate, type ResourceRow } from "./resources";
import {
  type SectionClaim,
  type SectionComponentId,
  sectionComponents,
} from "./sections";

/**
 * deleteClaimSteps returns the writes that delete claim under policy.
 * remove deletes the claim with its UID in uids and its resourceVersion in
 * preconditions, so that it fails with Stale when the claim changed since;
 * each is left out when the claim has none. policy is present only when
 * the deletion policy of the claim (deletionPolicyOf) differs from policy:
 * it writes spec.deletionPolicy as the whole claim, with the
 * resourceVersion of claim as its precondition. After policy, take remove
 * from the claim that carries the new policy.
 */
export function deleteClaimSteps<C extends DeletableClaim>(
  claim: C,
  policy: DeletionPolicy,
  intent: string,
): { policy?: Change; remove: Change } {
  const { apiVersion, kind, metadata, spec, status } = claim;
  const name = metadata.name ?? "";
  const ref = { apiVersion, kind, namespace: metadata.namespace, name };
  const remove: Change = {
    intent,
    apply: [],
    delete: [ref],
    ...(metadata.uid && { uids: { [refKey(ref)]: metadata.uid } }),
    ...(metadata.resourceVersion && {
      preconditions: { [refKey(ref)]: metadata.resourceVersion },
    }),
  };
  if (deletionPolicyOf(spec?.deletionPolicy, status?.resolvedPolicy) === policy)
    return { remove };
  const open: Session<C> = {
    claim,
    pending: new Map(),
    locks: () => undefined,
  };
  // A session without locks never refuses a change.
  const { session } = setValue(
    open,
    [{ field: "spec" }, { field: "deletionPolicy" }],
    policy,
  ) as { session: Session<C> };
  return {
    policy: changeOf(
      session,
      `Set the deletion policy of ${name} to ${policy} before the delete`,
    ),
    remove,
  };
}

/**
 * DeletionGroup is one selection of a claim, as the delete of the claim
 * shows it. label is the label of the section. slot is the slot of the
 * section, unset for a section without one or that is not found.
 * alwaysDeletes says that the registration of the selection does not
 * support Orphan. rows are the resources that the request of the selection
 * reports, each with its fate.
 */
export type DeletionGroup = {
  id: SectionComponentId;
  section: string;
  label: string;
  registration: string;
  slot?: Slot;
  alwaysDeletes: boolean;
  rows: (ResourceRow & { fate: "Kept" | "Deleted" })[];
};

/**
 * deletionGroups returns one group for each selection of claim, in the
 * order of sectionComponents, with the fate of each row under policy, or
 * under the policy of its request once that request is being deleted. Only
 * the requests that claim controls by its UID count, so requests can hold
 * those of other claims. The request of a selection is the one that the
 * claim status names, else the request with the section and the
 * registration of the selection. A selection without a request, or whose
 * request reports no resources, has no rows. A registration that is not
 * found counts as one that keeps.
 */
export function deletionGroups(input: {
  claim: SectionClaim & { metadata?: { uid?: string } };
  sections: readonly Resource<"PluginSection">[];
  registrations: readonly Resource<"PluginRegistration">[];
  requests: readonly Resource<"PluginRequest">[];
  policy: DeletionPolicy;
}): DeletionGroup[] {
  const { claim, sections, registrations, policy } = input;
  const uid = claim.metadata?.uid;
  const requests = input.requests.filter((r) => controlledBy(r, uid));
  return sectionComponents({ claim, sections, requests }).map((component) => {
    const { registration } = component.selection;
    const request =
      component.request ??
      requests.find(
        (r) =>
          r.spec?.section === component.section &&
          r.spec?.registration === registration,
      );
    const found = sections.find((s) => s.metadata?.name === component.section);
    // The cache object differs from the generated type only in its
    // timestamps, which slotOf does not read.
    const slot = found ? slotOf(found as unknown as PluginSection) : undefined;
    const deletionPolicies = registrations.find(
      (r) => r.metadata?.name === registration,
    )?.spec.deletionPolicies;
    return {
      id: component.id,
      section: component.section,
      label: component.label,
      registration,
      ...(slot && { slot }),
      alwaysDeletes:
        deletionPolicies !== undefined && !deletionPolicies.includes("Orphan"),
      rows: (request?.status?.resources ?? []).map((row) => ({
        ...row,
        fate: resourceFate(
          row,
          request ? requestPolicy(request, policy) : policy,
        ),
      })),
    };
  });
}

/**
 * controlledBy says whether the claim with uid is the controller of
 * request. A claim without a UID controls no request.
 */
export function controlledBy(
  request: Resource<"PluginRequest">,
  uid: string | undefined,
): boolean {
  return (
    uid !== undefined &&
    (request.metadata?.ownerReferences ?? []).some(
      (o) => o.controller === true && o.uid === uid,
    )
  );
}

/**
 * requestPolicy returns the deletion policy that decides the fate of the
 * resources of request: policy, the policy of its claim, until the request
 * is being deleted, and then the policy of the request itself.
 */
export function requestPolicy(
  request: Resource<"PluginRequest">,
  policy: DeletionPolicy,
): DeletionPolicy {
  // The claim finalizer writes the policy only on requests that are not
  // being deleted yet (writeDeletionPolicy of internal/claimdeletion).
  return request.metadata?.deletionTimestamp
    ? request.spec.deletionPolicy
    : policy;
}

/**
 * backupWarning returns the warning that the delete of the cluster name
 * takes its backup bucket: a group of the backupStorage slot holds a row
 * with the fate Deleted. It is undefined otherwise.
 */
export function backupWarning(
  name: string,
  groups: readonly DeletionGroup[],
): string | undefined {
  const goes = groups.some(
    (g) =>
      g.slot === "backupStorage" && g.rows.some((r) => r.fate === "Deleted"),
  );
  return goes
    ? `You cannot restore ${name} after this: the backup bucket goes too.`
    : undefined;
}
