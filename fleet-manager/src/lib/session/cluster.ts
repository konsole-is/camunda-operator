import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { clusterControllerParameters } from "@/lib/clusters/storage";
import {
  pathFrom,
  REFUSED_CLUSTER_FIELDS,
  refusedSteps,
  type RefusedField,
} from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import type { FieldPath } from "@/lib/merge";
import {
  checkAllowed,
  checkDeletionPolicy,
  checkSlotConflicts,
  deletionPolicyOf,
  releaseMove,
  type BackupLike,
  type MovingCluster,
} from "@/lib/policy";
import {
  changedParameterErrors,
  generated,
  messagesOf,
  selectionsOf,
} from "./parameters";
import { claimAfter, type Session } from "./session";
import { valueAt } from "./steps";

/** FieldError is one error of a session. Without a path, it shows above the fields. */
export type FieldError = { path?: FieldPath; message: string };

/**
 * ClusterClaimContext is what validateClusterClaim reads beside the session.
 * environment is the Environment of the namespace, cluster the live
 * CamundaCluster, backups the backups of its namespace, and requests the
 * live PluginRequests of the claim.
 */
export type ClusterClaimContext = {
  environment?: Resource<"Environment">;
  sections: readonly Resource<"PluginSection">[];
  registrations: readonly Resource<"PluginRegistration">[];
  releases: readonly Resource<"CamundaRelease">[];
  cluster: MovingCluster;
  backups: readonly BackupLike[];
  requests: readonly Resource<"PluginRequest">[];
};

type Claim = Resource<"CamundaClusterClaim">;

/**
 * validateClusterClaim returns the errors of the claim after the change of
 * s, before review: each refused field in spec.cluster, the first value that
 * the resolved policy does not allow, the first field that a selected slot
 * fills, each section parameter that s sets, changes, or removes and that
 * the schema of its registration refuses in the request, a refused deletion
 * policy, and a release move that skips a minor, downgrades without
 * allowVersionDowngrade set to its version, or names no release. It checks
 * a preset and a release that s sets against the Environment, also without
 * a resolved policy. After a reset of the release, the move goes to the
 * default release of the Environment. Types and ranges of the CRD are left
 * to the dry run.
 */
export function validateClusterClaim(
  s: Session<Claim>,
  ctx: ClusterClaimContext,
): FieldError[] {
  const after = claimAfter(s);
  // A reset of the last field of spec removes spec.
  const spec = (after.spec ?? {}) as Claim["spec"];
  const changed = (field: "presetRef" | "releaseRef") =>
    Boolean(spec[field]) && spec[field] !== s.claim.spec?.[field];
  const selections = selectionsOf(after);
  const errors: FieldError[] = [];

  for (const row of REFUSED_CLUSTER_FIELDS) {
    const steps = refusedSteps(row);
    if (isSet(valueAt(after, steps)))
      errors.push({ path: pathFrom("", steps), message: refusedMessage(row) });
  }

  // Without the snapshot of the claim, only the Environment can be
  // checked: the selections need the snapshot.
  const policy = after.status?.resolvedPolicy;
  errors.push(
    ...messagesOf(
      checkAllowed({
        kind: "CamundaClusterClaim",
        sections: generated<PluginSection[]>(ctx.sections),
        environment: generated<Environment | undefined>(ctx.environment),
        policy: policy ?? {},
        selections: policy ? selections : [],
        requests: generated<PluginRequest[]>(ctx.requests),
        ...(changed("presetRef") && { presetRef: spec.presetRef }),
        ...(changed("releaseRef") && { releaseRef: spec.releaseRef }),
      }),
    ),
  );

  errors.push(
    ...messagesOf(
      checkSlotConflicts(generated<PluginSection[]>(ctx.sections), selections, {
        cluster: spec.cluster,
        optimize: spec.attachments?.optimize,
      }),
    ),
  );

  errors.push(
    ...changedParameterErrors({
      claim: s.claim,
      after,
      environment: ctx.environment,
      registrations: ctx.registrations,
      policy,
      added: clusterControllerParameters(
        after.spec?.releaseRef || ctx.environment?.spec.releases?.default,
      ),
    }),
  );

  errors.push(
    ...messagesOf(
      checkDeletionPolicy(
        deletionPolicyOf(spec.deletionPolicy, policy),
        selections,
        generated<PluginRegistration[]>(ctx.registrations),
      ),
    ),
  );

  // A claim without a release runs the default release of the Environment.
  const fallback = ctx.environment?.spec.releases?.default;
  const release = spec.releaseRef || fallback;
  if (release && release !== (s.claim.spec?.releaseRef || fallback))
    errors.push(...releaseErrors(release, spec, ctx));

  return errors;
}

function refusedMessage(row: RefusedField): string {
  switch (row.setBy) {
    case "claim":
      return `The claim sets ${row.names.join(".")} in its own field, not in spec.cluster.`;
    case "release":
      return "The release sets this field. Use allowVersionDowngrade for a downgrade.";
    case "claimController":
      return "The claim controller sets this field.";
    case "section":
      return "A selected section sets this field.";
  }
}

// The CEL rule counts suspend: false and an empty string as unset.
function isSet(value: unknown): boolean {
  return (
    value !== undefined && value !== null && value !== false && value !== ""
  );
}

function releaseErrors(
  name: string,
  spec: Claim["spec"],
  ctx: ClusterClaimContext,
): FieldError[] {
  const path = "spec.releaseRef";
  const release = ctx.releases.find((r) => r.metadata?.name === name);
  if (!release) return [{ path, message: `Release ${name} does not exist.` }];
  const move = releaseMove(
    ctx.cluster,
    generated<CamundaRelease>(release),
    generated<Environment | undefined>(ctx.environment),
    ctx.backups,
  );
  if (move.skippedMinor)
    return [
      {
        path,
        message: `Skips ${move.skippedMinor}. Move to an ${move.skippedMinor} release first.`,
      },
    ];
  if (move.downgrade && spec.allowVersionDowngrade !== move.target)
    return [
      {
        path,
        message: `${move.current} to ${move.target} is a downgrade. A downgrade needs allowVersionDowngrade set to ${move.target}.`,
      },
    ];
  return [];
}
