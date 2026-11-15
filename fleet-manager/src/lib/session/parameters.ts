import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import { requestSchemaErrors, type ControllerParameters } from "@/lib/drafts";
import { pathFrom, type Step } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import {
  equalJSON,
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
// counts only at a parameter that the change sets, changes, or removes, so
// that a claim that breaks a rule already does not block every later change.
// added gives the parameters that the claim controller adds to a request.
export function changedParameterErrors(input: {
  claim: ClaimWithPlugins;
  after: ClaimWithPlugins;
  environment?: Resource<"Environment">;
  registrations: readonly Resource<"PluginRegistration">[];
  policy?: ResolvedPolicy;
  added: ControllerParameters;
}): FieldError[] {
  const { claim, after, policy, added } = input;
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
    const before = asObject(valueAt(claim, steps));
    const now = asObject(selection.parameters);
    const touched = [
      ...new Set([...Object.keys(before), ...Object.keys(now)]),
    ].filter((name) => !equalJSON(ownValue(now, name), ownValue(before, name)));
    const registration = input.registrations.find(
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
      : asObject(s.parameters),
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

// The CRD does not give the parameters of a registration a shape.
function asObject(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}
