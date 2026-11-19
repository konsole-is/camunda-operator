import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import { requestSchemaErrors, type ControllerParameters } from "@/lib/drafts";
import { pathFrom, type Step } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import {
  type DeletionPolicy,
  deletionPolicyOf,
  equalJSON,
  objectOrEmpty,
  resolve,
  type PluginSelection,
  type PolicyResult,
  type ResolvedPolicy,
} from "@/lib/policy";
import { schemaError, type OpenAPISchema } from "@/lib/schema";
import type { FieldError } from "./cluster";
import { valueAt } from "./steps";

/** ClaimWithPlugins is a claim of any kind that can select sections. */
export type ClaimWithPlugins = { spec?: { plugins?: unknown } };

// changedParameterErrors checks the request of each selection of after
// against the schema of its registration, as the create flows do. An error
// counts only at a parameter that the change sets, changes, or removes, or
// that the claim controller adds and the change moves, so that a claim that
// breaks a rule already does not block every later change. added gives the
// parameters that the claim controller adds to a request after the change,
// and addedBefore those before it. An error at a parameter that the
// controller adds and the change moves has no path.
export function changedParameterErrors(input: {
  claim: ClaimWithPlugins;
  after: ClaimWithPlugins;
  environment?: Resource<"Environment">;
  registrations: readonly Resource<"PluginRegistration">[];
  policy?: ResolvedPolicy;
  added: ControllerParameters;
  addedBefore?: ControllerParameters;
}): FieldError[] {
  const { claim, after, policy, added, addedBefore = added } = input;
  const selections = selectionsOf(after);
  const requests = requestsOf(selections, input);
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
    // The CRD does not give the parameters of a registration a shape.
    const before = objectOrEmpty(valueAt(claim, steps));
    const now = objectOrEmpty(selection.parameters);
    const touched = [
      ...new Set([...Object.keys(before), ...Object.keys(now)]),
    ].filter((name) => !equalJSON(ownValue(now, name), ownValue(before, name)));
    const registration = input.registrations.find(
      (r) => r.metadata?.name === keys.registration,
    );
    if (!registration) return [];
    const request = { ...keys, parameters: requests[i] };
    const own = generated<PluginRegistration>(registration);
    const nowAdded = added(request, own);
    const beforeAdded = addedBefore(request, own);
    const moved = Object.keys(nowAdded).filter(
      (name) => !equalJSON(nowAdded[name], beforeAdded[name]),
    );
    if (touched.length === 0 && moved.length === 0) return [];
    const root = objectOrEmpty(registration.spec.parameters) as OpenAPISchema;
    if (Object.keys(root).length === 0) return [];
    const at = (name: string) => pathFrom("", [...steps, { field: name }]);
    const broken = schemaError(root);
    if (broken !== undefined)
      return touched.map((name) => ({
        path: at(name),
        message: `The schema is not valid: ${broken}`,
      }));
    const { errors } = requestSchemaErrors(own, requests[i], nowAdded);
    const of = (name: string) =>
      errors.filter((e) => e.path === name || e.path.startsWith(`${name}.`));
    const controller = moved.flatMap((name) =>
      of(name).map((e) => ({
        message: `Parameter ${e.path} that the claim controller sets on section "${keys.section}" ${e.message}.`,
      })),
    );
    return [
      ...controller,
      ...touched.flatMap((name) => {
        // The policy check covers the name of a choice that the claim sets.
        const choices =
          policy?.parameters?.[keys.registration]?.[name]?.allowed;
        if (Object.hasOwn(now, name) && choices?.length) return [];
        return of(name).map((e) => ({ path: at(name), message: e.message }));
      }),
    ];
  });
}

// requestsOf returns the parameters of the request of each selection, as
// resolve gives them. Without the snapshot of the claim, they are the
// parameters of the selections.
export function requestsOf(
  selections: PluginSelection[],
  input: {
    environment?: Resource<"Environment">;
    registrations: readonly Resource<"PluginRegistration">[];
    policy?: ResolvedPolicy;
  },
): Record<string, unknown>[] {
  const resolution =
    input.policy &&
    resolve({
      recorded: input.policy,
      environment: generated<Environment | undefined>(input.environment),
      registrations: generated<PluginRegistration[]>(input.registrations),
      selections,
    });
  return selections.map((s, i) =>
    resolution && "value" in resolution
      ? (resolution.value.requests?.[i]?.parameters ?? {})
      : objectOrEmpty(s.parameters),
  );
}

// selectionsOf returns the selections of claim that name a section and a
// registration. A change can set spec.plugins to any value, and the dry run
// reports one of the wrong type.
export function selectionsOf(claim: ClaimWithPlugins): PluginSelection[] {
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

// toOrphan says that a save moves the deletion policy that a claim with the
// resolved policy gets to Orphan from another policy. before and after are
// spec.deletionPolicy of the live claim and of the save.
export function toOrphan(
  before: DeletionPolicy | "" | undefined,
  after: DeletionPolicy | "" | undefined,
  resolved: ResolvedPolicy | undefined,
): boolean {
  return (
    deletionPolicyOf(after, resolved) === "Orphan" &&
    deletionPolicyOf(before, resolved) !== "Orphan"
  );
}

// messagesOf turns a refusal or an error of the policy library into one
// error above the fields.
export function messagesOf(result: PolicyResult<unknown>): FieldError[] {
  if ("refusal" in result) return [{ message: result.refusal.message }];
  if ("error" in result) return [{ message: result.error }];
  return [];
}

// generated gives an object of the cache the generated type that the policy
// library takes. The two differ only in their timestamps, string against
// Date, and the policy library reads no timestamp.
export function generated<T>(value: unknown): T {
  return value as T;
}

function ownValue(object: Record<string, unknown>, name: string): unknown {
  return Object.hasOwn(object, name) ? object[name] : undefined;
}
