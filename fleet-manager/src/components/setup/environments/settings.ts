import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import { HOST_PATTERN_PARAMETERS } from "@/lib/claims/hosts";
import { ACCESS_SECTION } from "@/lib/labels";
import {
  builtinEntry,
  byCodeUnit,
  decodeObject,
  isObject,
  lockedSummary,
  own,
  setOwn,
} from "@/lib/policy";
import { type OpenAPISchema, type ValidationError, validate } from "@/lib/schema";

type Spec = Environment["spec"];
type Policy = NonNullable<Spec["parameters"]>[string][string];

/** NamedValue is one allowed choice of a claim-editable parameter: what the claim stores, what the pages show, and what the request gets. */
export type NamedValue = NonNullable<Policy["allowed"]>[number];

/** ParameterSetting is one parameter of a registration with the policy of the environment. */
export type ParameterSetting = {
  name: string;
  label: string;
  schema?: OpenAPISchema;
  /** registrationSummary is lockedSummary of the value that the registration gives the parameter. */
  registrationSummary: string;
  value?: unknown;
  claimEditable: boolean;
  allowed: NamedValue[];
  /** template is set for a parameter whose schema has x-fleet-template: true. */
  template: boolean;
  /** hostPattern is set for a host pattern of an access registration. */
  hostPattern: boolean;
  /** presetKind is set for the parameter that lists presets of that kind. */
  presetKind?: "ElasticsearchClusterPreset" | "DatabaseServerPreset";
};

/** OptionSettings is one registration with the settings of its parameters and its limits. */
export type OptionSettings = {
  registration: string;
  label: string;
  limits: { label: string; value: string; description?: string }[];
  parameters: ParameterSetting[];
};

/**
 * optionSettings returns one entry for each registration that a section of
 * spec allows, by name. A name that no registration has gives no entry.
 * parameters are the properties of the schema in schema order, then the
 * keys of the environment policy that are not properties in code-unit
 * order, without the parameters that the built-in table hides.
 */
export function optionSettings(
  registrations: readonly PluginRegistration[],
  spec: Spec,
): OptionSettings[] {
  const allowed = new Set(
    Object.values(spec.sections ?? {}).flatMap((s) => s.allowed ?? []),
  );
  return registrations
    .filter((r) => allowed.has(r.metadata.name ?? ""))
    .sort((a, b) => byCodeUnit(a.metadata.name ?? "", b.metadata.name ?? ""))
    .map((r) => settingsOf(r, spec));
}

function settingsOf(
  registration: PluginRegistration,
  spec: Spec,
): OptionSettings {
  const name = registration.metadata.name ?? "";
  const schema = isObject(registration.spec.parameters)
    ? (registration.spec.parameters as OpenAPISchema)
    : {};
  const properties = isObject(schema.properties)
    ? (schema.properties as Record<string, unknown>)
    : {};
  const others = isObject(schema.additionalProperties)
    ? (schema.additionalProperties as OpenAPISchema)
    : undefined;
  const policies = own(spec.parameters ?? {}, name) ?? {};
  const values = decodeObject(registration.spec.parameterValues) ?? {};
  const entry = builtinEntry(registration);
  const access = registration.spec.sections.includes(ACCESS_SECTION);
  const names = [
    ...Object.keys(properties),
    ...Object.keys(policies)
      .filter((k) => !Object.hasOwn(properties, k))
      .sort(byCodeUnit),
  ].filter((p) => !entry?.hidden.includes(p));
  return {
    registration: name,
    label: registration.spec.label,
    limits: registration.spec.limits ?? [],
    parameters: names.map((parameter): ParameterSetting => {
      const property = own(properties, parameter);
      const propertySchema = isObject(property) ? (property as OpenAPISchema) : undefined;
      const parameterSchema = propertySchema ?? others;
      const policy: Policy = own(policies, parameter) ?? {};
      const presetKind = entry && own(entry.lists, parameter);
      return {
        name: parameter,
        label:
          typeof propertySchema?.title === "string" && propertySchema.title !== ""
            ? propertySchema.title
            : parameter,
        ...(parameterSchema && { schema: parameterSchema }),
        registrationSummary: lockedSummary(own(values, parameter)),
        ...(policy.value !== undefined && { value: policy.value }),
        claimEditable: policy.claimEditable ?? false,
        allowed: policy.allowed ?? [],
        template: parameterSchema?.["x-fleet-template"] === true,
        hostPattern:
          access &&
          (HOST_PATTERN_PARAMETERS as readonly string[]).includes(parameter),
        ...((presetKind === "ElasticsearchClusterPreset" ||
          presetKind === "DatabaseServerPreset") && { presetKind }),
      };
    }),
  };
}

/**
 * withParameterPolicy returns spec with the policy of one parameter. An
 * undefined value, claimEditable false, and an empty allowed list are left
 * out, and a policy without claimEditable has no allowed. An empty policy
 * removes the entry, a registration without entries leaves
 * spec.parameters, and spec.parameters without registrations goes too.
 * spec does not change.
 */
export function withParameterPolicy(
  spec: Spec,
  registration: string,
  parameter: string,
  policy: { value?: unknown; claimEditable?: boolean; allowed?: NamedValue[] },
): Spec {
  const entry: Policy = {
    ...(policy.value !== undefined && { value: policy.value }),
    ...(policy.claimEditable && { claimEditable: true }),
    ...(policy.claimEditable &&
      policy.allowed?.length && { allowed: policy.allowed }),
  };
  const parameters = structuredClone(spec.parameters ?? {});
  const policies = own(parameters, registration) ?? {};
  delete policies[parameter];
  if (Object.keys(entry).length > 0) setOwn(policies, parameter, entry);
  delete parameters[registration];
  if (Object.keys(policies).length > 0)
    setOwn(parameters, registration, policies);
  const next = { ...spec, parameters };
  if (Object.keys(parameters).length === 0) delete next.parameters;
  return next;
}

/**
 * allowedErrors returns an error for each empty name or label and for each
 * name or label that an earlier row has, by path "<row>.name" or
 * "<row>.label".
 */
export function allowedErrors(
  allowed: readonly NamedValue[],
): ValidationError[] {
  const errors: ValidationError[] = [];
  allowed.forEach((choice, row) => {
    const earlier = allowed.slice(0, row);
    if (choice.name.trim() === "")
      errors.push({ path: `${row}.name`, message: "Give the choice a name." });
    else if (earlier.some((c) => c.name === choice.name))
      errors.push({ path: `${row}.name`, message: "Each name is used once." });
    if (choice.label.trim() === "")
      errors.push({
        path: `${row}.label`,
        message: "Give the choice a label.",
      });
    else if (earlier.some((c) => c.label === choice.label))
      errors.push({
        path: `${row}.label`,
        message: "Each label is used once.",
      });
  });
  return errors;
}

/**
 * settingErrors returns an error for each value of settings that its schema
 * refuses: the environment value at
 * spec.parameters.<registration>.<parameter>.value, and each allowed value
 * at .allowed.<row>.value. An allowed value of an array parameter that is
 * not a list is one item, so its items schema checks it.
 */
export function settingErrors(
  settings: readonly OptionSettings[],
): ValidationError[] {
  const errors: ValidationError[] = [];
  for (const option of settings) {
    for (const p of option.parameters) {
      if (!p.schema) continue;
      const path = `spec.parameters.${option.registration}.${p.name}`;
      const check = (schema: OpenAPISchema, value: unknown, at: string) => {
        const result = validate(schema, value);
        if (!result.ok)
          errors.push({
            path: at,
            message: result.errors
              .map((e) => (e.path ? `${e.path}: ${e.message}` : e.message))
              .join(" "),
          });
      };
      if (p.value !== undefined) check(p.schema, p.value, `${path}.value`);
      const items =
        p.schema.type === "array" && isObject(p.schema.items)
          ? (p.schema.items as OpenAPISchema)
          : p.schema;
      p.allowed.forEach((choice, row) =>
        check(
          Array.isArray(choice.value) ? p.schema! : items,
          choice.value,
          `${path}.allowed.${row}.value`,
        ),
      );
    }
  }
  return errors;
}
