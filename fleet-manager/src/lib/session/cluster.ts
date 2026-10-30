import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
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
  checkSlotConflicts,
  equalJSON,
  releaseMove,
  resolve,
  type BackupLike,
  type MovingCluster,
  type PluginSelection,
  type PolicyResult,
  type ResolvedPolicy,
} from "@/lib/policy";
import { schemaError, validate, type OpenAPISchema } from "@/lib/schema";
import { claimAfter, type Session } from "./session";
import { valueAt } from "./steps";

/** FieldError is one error of a session. Without a path, it shows above the fields. */
export type FieldError = { path?: FieldPath; message: string };

/**
 * ClusterClaimContext is what validateClusterClaim reads beside the session.
 * environment is the Environment of the namespace, cluster the live
 * CamundaCluster, and backups the backups of its namespace.
 */
export type ClusterClaimContext = {
  environment?: Resource<"Environment">;
  sections: readonly Resource<"PluginSection">[];
  registrations: readonly Resource<"PluginRegistration">[];
  releases: readonly Resource<"CamundaRelease">[];
  cluster: MovingCluster;
  backups: readonly BackupLike[];
};

type Claim = Resource<"CamundaClusterClaim">;

/**
 * validateClusterClaim returns the errors of the claim after the change of
 * s, before review: each refused field in spec.cluster, the first value that
 * the resolved policy does not allow, the first field that a selected slot
 * fills, each section parameter that fails the schema of its registration,
 * and a release move that skips a minor, downgrades without
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

// changedParameterErrors checks each section parameter that after sets to
// another value than claim.
function changedParameterErrors(
  claim: Claim,
  after: Claim,
  ctx: ClusterClaimContext,
  policy: ResolvedPolicy | undefined,
): FieldError[] {
  const errors: FieldError[] = [];
  for (const selection of selectionsOf(after)) {
    const keys = {
      section: selection.section,
      registration: selection.registration,
    };
    const steps: Step[] = [
      { field: "spec" },
      { field: "plugins" },
      { item: keys },
    ];
    const before = asObject(
      valueAt(claim, [...steps, { field: "parameters" }]),
    );
    for (const [name, value] of Object.entries(
      asObject(selection.parameters),
    )) {
      if (
        equalJSON(value, Object.hasOwn(before, name) ? before[name] : undefined)
      )
        continue;
      const path = pathFrom("", [
        ...steps,
        { field: "parameters" },
        { field: name },
      ]);
      errors.push(
        ...parameterErrors(after, keys, name, value, path, ctx, policy),
      );
    }
  }
  return errors;
}

function parameterErrors(
  after: Claim,
  keys: { section: string; registration: string },
  name: string,
  value: unknown,
  path: FieldPath,
  ctx: ClusterClaimContext,
  policy: ResolvedPolicy | undefined,
): FieldError[] {
  // A parameter with choices holds the name of a choice, which the policy
  // check covers. The schema describes the value of the choice.
  const choices = policy?.parameters?.[keys.registration]?.[name]?.allowed;
  if (choices?.length) return [];
  const registration = ctx.registrations.find(
    (r) => r.metadata?.name === keys.registration,
  );
  const root = asObject(registration?.spec.parameters) as OpenAPISchema;
  if (Object.keys(root).length === 0) return [];
  const broken = schemaError(root);
  if (broken !== undefined)
    return [{ path, message: `The schema is not valid: ${broken}` }];
  // The parameters of the request are checked, so that references and root
  // rules see the values that the plugin gets.
  const result = validate(root, {
    ...requestOf(after, keys, ctx, policy),
    [name]: value,
  });
  if (result.ok) return [];
  return result.errors
    .filter((e) => e.path === name || e.path.startsWith(`${name}.`))
    .map((e) => ({ path, message: e.message }));
}

// requestOf returns the parameters of the request of the selection with
// keys, as resolve gives them. Without the snapshot of the claim, it returns
// the parameters of the claim.
function requestOf(
  after: Claim,
  keys: Record<string, unknown>,
  ctx: ClusterClaimContext,
  policy: ResolvedPolicy | undefined,
): Record<string, unknown> {
  const selections = selectionsOf(after);
  const i = selections.findIndex((s) =>
    Object.entries(keys).every(([k, v]) => s[k as keyof PluginSelection] === v),
  );
  const resolution =
    policy &&
    resolve({
      recorded: policy,
      environment: generated<Environment | undefined>(ctx.environment),
      registrations: generated<PluginRegistration[]>(ctx.registrations),
      selections,
    });
  if (resolution && "value" in resolution)
    return resolution.value.requests?.[i]?.parameters ?? {};
  return asObject(selections[i]?.parameters);
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

// The CRD does not give the parameters of a registration a shape.
function asObject(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}
