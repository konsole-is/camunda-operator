import { isManaged } from "@/lib/labels";
import { child } from "@/lib/merge";
import { resolve, type PluginSelection } from "@/lib/policy";
import type { ClusterFieldsInput } from "./cluster";
import type { ClusterField, FieldState } from "./types";
import { pathFrom, type Step } from "./walk";

/**
 * parameterFields returns one field for each parameter of each selection of
 * the claim, in selection order, then by parameter name. The names are those
 * of the registration schema, of the resolved policy, and of the claim. A
 * parameter that the claim cannot change is Set by your admin, with the
 * policy value, also when the claim holds a value for it. A claim-editable
 * one is Changed in the claim when the claim sets it, else From environment.
 * When the policy resolves for the claim, the value is the one that the
 * request gets, so a choice with a value that the claim picks by name has
 * that value.
 * A cluster without a claim, or an unmanaged one, has none.
 */
export function parameterFields(input: ClusterFieldsInput): ClusterField[] {
  if (!isManaged(input.cluster) || input.claim === undefined) return [];
  const selections = (input.claim.spec.plugins ?? []) as PluginSelection[];
  const resolved = input.policy
    ? resolve({ recorded: input.policy, selections })
    : undefined;
  const requests =
    resolved && "value" in resolved ? resolved.value.requests : undefined;
  const fields: ClusterField[] = [];
  for (const [i, s] of selections.entries()) {
    const request = requests?.[i];
    const policies = input.policy?.parameters?.[s.registration] ?? {};
    const claimed = asObject(s.parameters);
    const registration = input.registrations.find(
      (r) => r.metadata?.name === s.registration,
    );
    const schema = properties(registration?.spec.parameters);
    const names = [
      ...new Set([
        ...Object.keys(schema),
        ...Object.keys(policies),
        ...Object.keys(claimed),
      ]),
    ].sort();
    const selection: Step[] = [
      { field: "spec" },
      { field: "plugins" },
      { item: { section: s.section, registration: s.registration } },
    ];

    for (const name of names) {
      const policy = Object.hasOwn(policies, name) ? policies[name] : undefined;
      const editable = policy?.claimEditable === true;
      const set = editable && Object.hasOwn(claimed, name);
      const state: FieldState = !editable
        ? { kind: "admin" }
        : set
          ? { kind: "claim" }
          : { kind: "environment" };
      const path = child(child("spec", "parameters"), name);
      const claimSteps: Step[] = [
        ...selection,
        { field: "parameters" },
        { field: name },
      ];
      const description = Object.hasOwn(schema, name)
        ? describe(schema[name])
        : undefined;
      fields.push({
        key: `${s.section}/${s.registration}:${path}`,
        target: {
          kind: "PluginRequest",
          section: s.section,
          registration: s.registration,
        },
        group: "parameters",
        path,
        label: name,
        value: request
          ? request.parameters?.[name]
          : set
            ? claimed[name]
            : policy?.value,
        state,
        ...(!editable && { lock: { reason: "admin" as const } }),
        claimPath: pathFrom("", claimSteps),
        claimSteps,
        ...(description !== undefined && { description }),
      });
    }
  }
  return fields;
}

// The CRD does not give parameters or their schema a shape, so these read
// them defensively.
function asObject(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}

function properties(schema: unknown): Record<string, unknown> {
  return asObject(asObject(schema).properties);
}

function describe(property: unknown): string | undefined {
  const text = asObject(property).description;
  return typeof text === "string" && text !== "" ? text : undefined;
}
