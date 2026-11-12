import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { clusterControllerParameters } from "@/lib/clusters/storage";
import { requestSchemaErrors } from "@/lib/drafts";
import {
  pathFrom,
  REFUSED_CLUSTER_FIELDS,
  refusedSteps,
  type RefusedField,
  type Step,
} from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import type { FieldPath } from "@/lib/merge";
import {
  checkAllowed,
  checkDeletionPolicy,
  checkSlotConflicts,
  deletionPolicyOf,
  equalJSON,
  releaseMove,
  resolve,
  type BackupLike,
  type MovingCluster,
  type PluginSelection,
  type PolicyResult,
  type ResolvedPolicy,
} from "@/lib/policy";
import { schemaError, type OpenAPISchema } from "@/lib/schema";
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

  errors.push(...changedParameterErrors(s.claim, after, ctx, policy));

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

function messagesOf(result: PolicyResult<unknown>): FieldError[] {
  if ("refusal" in result) return [{ message: result.refusal.message }];
  if ("error" in result) return [{ message: result.error }];
  return [];
}

// changedParameterErrors checks the request of each selection of after
// against the schema of its registration, as the create flows do. An error
// counts only at a parameter that the change sets, changes, or removes, so
// that a claim that breaks a rule already does not block every later change.
function changedParameterErrors(
  claim: Claim,
  after: Claim,
  ctx: ClusterClaimContext,
  policy: ResolvedPolicy | undefined,
): FieldError[] {
  const selections = selectionsOf(after);
  const requests = requestsOf(selections, ctx, policy);
  const added = clusterControllerParameters(
    after.spec?.releaseRef || ctx.environment?.spec.releases?.default,
  );
  return selections.flatMap((selection, i) => {
    const keys = {
      section: selection.section,
      registration: selection.registration,
    };
    const steps: Step[] = [
      { field: "spec" },
      { field: "plugins" },
      { item: keys },
      { field: "parameters" },
    ];
    const before = asObject(valueAt(claim, steps));
    const now = asObject(selection.parameters);
    const touched = [
      ...new Set([...Object.keys(before), ...Object.keys(now)]),
    ].filter((name) => !equalJSON(ownValue(now, name), ownValue(before, name)));
    const registration = ctx.registrations.find(
      (r) => r.metadata?.name === keys.registration,
    );
    if (!registration || touched.length === 0) return [];
    const root = asObject(registration.spec.parameters) as OpenAPISchema;
    if (Object.keys(root).length === 0) return [];
    const at = (name: string) => pathFrom("", [...steps, { field: name }]);
    const broken = schemaError(root);
    if (broken !== undefined)
      return touched.map((name) => ({
        path: at(name),
        message: `The schema is not valid: ${broken}`,
      }));
    const { errors } = requestSchemaErrors(
      generated<PluginRegistration>(registration),
      requests[i],
      added(
        { ...keys, parameters: requests[i] },
        generated<PluginRegistration>(registration),
      ),
    );
    return touched.flatMap((name) => {
      // The policy check covers the name of a choice that the claim sets.
      const choices = policy?.parameters?.[keys.registration]?.[name]?.allowed;
      if (Object.hasOwn(now, name) && choices?.length) return [];
      return errors
        .filter((e) => e.path === name || e.path.startsWith(`${name}.`))
        .map((e) => ({ path: at(name), message: e.message }));
    });
  });
}

// requestsOf returns the parameters of the request of each selection, as
// resolve gives them. Without the snapshot of the claim, they are the
// parameters of the selections.
function requestsOf(
  selections: PluginSelection[],
  ctx: ClusterClaimContext,
  policy: ResolvedPolicy | undefined,
): Record<string, unknown>[] {
  const resolution =
    policy &&
    resolve({
      recorded: policy,
      environment: generated<Environment | undefined>(ctx.environment),
      registrations: generated<PluginRegistration[]>(ctx.registrations),
      selections,
    });
  return selections.map((s, i) =>
    resolution && "value" in resolution
      ? (resolution.value.requests?.[i]?.parameters ?? {})
      : asObject(s.parameters),
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

// generated gives an object of the cache the generated type that the policy
// library takes. The two differ only in their timestamps, string against
// Date, and the policy library reads no timestamp.
function generated<T>(value: unknown): T {
  return value as T;
}

// selectionsOf returns the selections of claim that name a section and a
// registration. A change can set spec.plugins to any value, and the dry run
// reports one of the wrong type.
function selectionsOf(claim: Claim): PluginSelection[] {
  const plugins: unknown = claim.spec?.plugins;
  if (!Array.isArray(plugins)) return [];
  return plugins.filter(
    (p): p is PluginSelection =>
      typeof p === "object" &&
      p !== null &&
      typeof p.section === "string" &&
      typeof p.registration === "string",
  );
}

function ownValue(object: Record<string, unknown>, name: string): unknown {
  return Object.hasOwn(object, name) ? object[name] : undefined;
}

// The CRD does not give the parameters of a registration a shape.
function asObject(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}
