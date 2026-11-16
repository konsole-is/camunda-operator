import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import { fieldSchemaFromOpenAPI } from "@/lib/fields/editor";
import { builtinEntry } from "@/lib/policy/builtins";
import { byCodeUnit, own } from "@/lib/policy/json";
import {
  type ParameterPolicies,
  hasValue,
  parameterPolicies,
} from "@/lib/policy/resolve";
import type { PolicyResult } from "@/lib/policy/types";
import { isRecord, isText } from "./values";

/**
 * ParameterRow is one parameter of a registration. error says why its
 * schema cannot be read. environmentOnly is true for a parameter that only
 * an Environment names.
 */
export type ParameterRow = {
  name: string;
  environmentOnly?: true;
  error?: string;
  description?: string;
  /** type is the type of the property schema, as written there. */
  type?: string;
  minimum?: number;
  maximum?: number;
  values?: readonly string[];
  /** fallback is the value without an Environment: parameterValues of the registration. */
  fallback?: unknown;
  claimEditable: boolean;
  /** environments is unset while Environment is not read. */
  environments?: {
    /** name is the name of the Environment, and environment its display name, else its name. */
    name: string;
    environment: string;
    value?: unknown;
    claimEditable: boolean;
  }[];
};

/**
 * parameterRows gives one row for each parameter of registration, by name,
 * without the parameters that the claim controller sets. Each of
 * environments adds its value and claimEditable, in the order of
 * environments, while Environment is not in unread. An error of
 * parameterPolicies, and a schema or properties that are not an object, is
 * the result.
 */
export function parameterRows(
  registration: PluginRegistration,
  environments: readonly Environment[],
  unread: readonly string[],
): PolicyResult<ParameterRow[]> {
  const schema = registration.spec.parameters;
  if (schema !== undefined && !isRecord(schema))
    return { error: "The parameter schema is not an object." };
  const properties = schema?.properties;
  if (properties !== undefined && !isRecord(properties))
    return {
      error: "The properties of the parameter schema are not an object.",
    };
  const claimEditable: unknown = registration.spec.claimEditable;
  if (claimEditable !== undefined && !Array.isArray(claimEditable))
    return { error: "The claimEditable of the registration is not a list." };
  const fallback = parameterPolicies(registration, undefined);
  if (!("value" in fallback)) return fallback;
  const byEnvironment: {
    name: string;
    environment: string;
    policies: ParameterPolicies;
  }[] = [];
  if (!unread.includes("Environment")) {
    for (const environment of environments) {
      if (!validEntries(environment, registration.metadata.name ?? ""))
        return {
          error: `The parameters of the Environment ${environment.metadata.name ?? ""} for this option are not valid.`,
        };
      const policies = parameterPolicies(registration, environment);
      if (!("value" in policies)) return policies;
      byEnvironment.push({
        name: environment.metadata.name ?? "",
        environment:
          environment.spec.displayName || (environment.metadata.name ?? ""),
        policies: policies.value,
      });
    }
  }
  const hidden = builtinEntry(registration)?.hidden ?? [];
  const names = [
    ...new Set([
      ...Object.keys(properties ?? {}),
      ...Object.keys(fallback.value),
      ...byEnvironment.flatMap(({ policies }) => Object.keys(policies)),
    ]),
  ]
    .filter((name) => !hidden.includes(name))
    .sort(byCodeUnit);
  return {
    value: names.map((name) => {
      const policy = own(fallback.value, name);
      const property = properties ? own(properties, name) : undefined;
      return {
        name,
        ...(property === undefined &&
          policy === undefined && { environmentOnly: true as const }),
        ...schemaFacts(property),
        ...(policy && hasValue(policy) && { fallback: policy.value }),
        claimEditable: policy?.claimEditable === true,
        ...(!unread.includes("Environment") && {
          environments: byEnvironment.map(
            ({ name: id, environment, policies }) => {
              const entry = own(policies, name);
              return {
                name: id,
                environment,
                value: entry && hasValue(entry) ? entry.value : undefined,
                claimEditable: entry?.claimEditable === true,
              };
            },
          ),
        }),
      };
    }),
  };
}

type SchemaFacts = Pick<
  ParameterRow,
  "error" | "description" | "type" | "minimum" | "maximum" | "values"
>;

function schemaFacts(property: unknown): SchemaFacts {
  if (property === undefined) return {};
  const field = fieldSchemaFromOpenAPI(property);
  if (!field || !isRecord(property))
    return { error: "The schema of this parameter is not an object." };
  if (
    !isText(property.description, true) ||
    !isText(property.type, true) ||
    !isNumber(property.minimum) ||
    !isNumber(property.maximum) ||
    (property.enum !== undefined && !Array.isArray(property.enum))
  )
    return {
      error: "The schema of this parameter has a field of the wrong type.",
    };
  return {
    ...(typeof property.description === "string" && {
      description: property.description,
    }),
    ...(typeof property.type === "string" && { type: property.type }),
    ...(field.minimum !== undefined && { minimum: field.minimum }),
    ...(field.maximum !== undefined && { maximum: field.maximum }),
    ...(Array.isArray(property.enum) && {
      values: property.enum.map((v) =>
        typeof v === "string" ? v : JSON.stringify(v),
      ),
    }),
  };
}

function isNumber(value: unknown): boolean {
  return value === undefined || typeof value === "number";
}

// parameterPolicies throws on an Environment entry that is not an object
// of policy objects, or whose allowed is not a list of objects.
function validEntries(environment: Environment, name: string): boolean {
  const parameters: unknown = environment.spec.parameters;
  if (parameters === undefined) return true;
  if (!isRecord(parameters)) return false;
  const entries = own(parameters, name);
  return (
    entries === undefined ||
    (isRecord(entries) &&
      Object.values(entries).every(
        (e) =>
          isRecord(e) &&
          (e.allowed === undefined ||
            (Array.isArray(e.allowed) && e.allowed.every(isRecord))),
      ))
  );
}
