import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { Environment } from "@/generated/crds/environment";
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
  releaseMove,
  type BackupLike,
  type MovingCluster,
  type PluginSelection,
  type PolicyResult,
} from "@/lib/policy";
import { validate, type OpenAPISchema } from "@/lib/schema";
import { claimAfter, type PendingChange, type Session } from "./session";
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
 * validateClusterClaim returns every error of the claim after the change of
 * s, before review: a refused field in spec.cluster, a value that the
 * resolved policy does not allow, a field that a selected slot fills, a
 * section parameter that fails the schema of its registration, and a
 * release move that skips a minor, downgrades without allowVersionDowngrade
 * set to its version, or names no release. It checks the preset and the
 * release against the Environment only when s changes them. Types and
 * ranges of the CRD are left to the dry run.
 */
export function validateClusterClaim(
  s: Session<Claim>,
  ctx: ClusterClaimContext,
): FieldError[] {
  const after = claimAfter(s);
  const spec = after.spec;
  const selections = (spec.plugins ?? []) as PluginSelection[];
  const errors: FieldError[] = [];

  for (const row of REFUSED_CLUSTER_FIELDS) {
    const steps = refusedSteps(row);
    if (isSet(valueAt(after, steps)))
      errors.push({ path: pathFrom("", steps), message: refusedMessage(row) });
  }

  const policy = after.status?.resolvedPolicy;
  if (policy)
    errors.push(
      ...messagesOf(
        checkAllowed({
          kind: "CamundaClusterClaim",
          sections: generated<PluginSection[]>(ctx.sections),
          environment: generated<Environment | undefined>(ctx.environment),
          policy,
          selections,
          ...(changed(s, "presetRef") && { presetRef: spec.presetRef }),
          ...(changed(s, "releaseRef") && { releaseRef: spec.releaseRef }),
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

  for (const entry of s.pending.values())
    errors.push(...parameterErrors(entry, ctx));

  if (changed(s, "releaseRef") && spec.releaseRef)
    errors.push(...releaseErrors(spec.releaseRef, spec, ctx));

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

function changed(s: Session<Claim>, field: string): boolean {
  const entry = [...s.pending.values()].find(
    (e) =>
      e.steps.length === 2 &&
      "field" in e.steps[1] &&
      e.steps[1].field === field,
  );
  return entry !== undefined && !("reset" in entry);
}

function messagesOf(result: PolicyResult<unknown>): FieldError[] {
  if ("refusal" in result) return [{ message: result.refusal.message }];
  if ("error" in result) return [{ message: result.error }];
  return [];
}

// A section parameter has the steps spec, plugins, the selection,
// parameters, and its name.
function parameterErrors(
  entry: PendingChange,
  ctx: ClusterClaimContext,
): FieldError[] {
  if ("reset" in entry || entry.steps.length !== 5) return [];
  const [, plugins, selection, parameters, name] = entry.steps as [
    Step,
    Step,
    Step,
    Step,
    Step,
  ];
  if (
    !("field" in plugins && plugins.field === "plugins") ||
    !("item" in selection) ||
    !("field" in parameters && parameters.field === "parameters") ||
    !("field" in name)
  )
    return [];
  const registration = ctx.registrations.find(
    (r) => r.metadata?.name === selection.item.registration,
  );
  const schema = asObject(
    asObject(asObject(registration?.spec.parameters).properties)[name.field],
  );
  if (Object.keys(schema).length === 0) return [];
  const result = validate(schema as OpenAPISchema, entry.value);
  return result.ok
    ? []
    : result.errors.map((e) => ({ path: entry.path, message: e.message }));
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

// The CRD does not give the parameters of a registration a shape.
function asObject(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}
