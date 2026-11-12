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
import {
  type OpenAPISchema,
  type ValidationError,
  validate,
} from "@/lib/schema";
import { templateErrors } from "./preview";

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
  /** allowedBySection is false for an option that only spec.parameters names. */
  allowedBySection: boolean;
  limits: { label: string; value: string; description?: string }[];
  parameters: ParameterSetting[];
};

/**
 * optionSettings returns one entry for each registration that a section of
 * spec allows or that spec.parameters names, by name. A name that no
 * registration has gives no entry.
 * parameters are the properties of the schema in schema order, then the
 * keys of the environment policy and of the values of the registration that
 * are not properties in code-unit order, without the parameters that the
 * built-in table hides.
 */
export function optionSettings(
  registrations: readonly PluginRegistration[],
  spec: Spec,
): OptionSettings[] {
  const allowed = new Set(
    Object.values(spec.sections ?? {}).flatMap((s) => s.allowed ?? []),
  );
  const named = new Set(Object.keys(spec.parameters ?? {}));
  return registrations
    .filter((r) => {
      const name = r.metadata.name ?? "";
      return allowed.has(name) || named.has(name);
    })
    .sort((a, b) => byCodeUnit(a.metadata.name ?? "", b.metadata.name ?? ""))
    .map((r) => ({
      ...settingsOf(r, spec),
      allowedBySection: allowed.has(r.metadata.name ?? ""),
    }));
}

function settingsOf(
  registration: PluginRegistration,
  spec: Spec,
): Omit<OptionSettings, "allowedBySection"> {
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
    ...[...new Set([...Object.keys(policies), ...Object.keys(values)])]
      .filter((k) => !Object.hasOwn(properties, k))
      .sort(byCodeUnit),
  ].filter((p) => !entry?.hidden.includes(p));
  return {
    registration: name,
    label: registration.spec.label,
    limits: registration.spec.limits ?? [],
    parameters: names.map((parameter): ParameterSetting => {
      const property = own(properties, parameter);
      const propertySchema = isObject(property)
        ? (property as OpenAPISchema)
        : undefined;
      const parameterSchema = propertySchema ?? others;
      const policy: Policy = own(policies, parameter) ?? {};
      const presetKind = entry && own(entry.lists, parameter);
      return {
        name: parameter,
        label:
          typeof propertySchema?.title === "string" &&
          propertySchema.title !== ""
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
  const next: Spec = { ...spec, parameters };
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
 * refuses, that is not a valid template parameter in a template
 * parameter, or that names a preset that spec does not list in a preset
 * parameter: the environment value at
 * spec.parameters.<registration>.<parameter>.value, and each allowed value
 * at .allowed.<row>.value. An allowed value of an array parameter that is
 * not a list is one item, so its items schema checks it.
 */
export function settingErrors(
  settings: readonly OptionSettings[],
  spec: Spec,
): ValidationError[] {
  const errors: ValidationError[] = [];
  for (const option of settings) {
    for (const p of option.parameters) {
      const path = `spec.parameters.${option.registration}.${p.name}`;
      const presets = p.presetKind && presetsOf(spec, p.presetKind);
      const check = (
        schema: OpenAPISchema | undefined,
        value: unknown,
        at: string,
      ) => {
        const result = schema ? validate(schema, value) : undefined;
        const found = [
          ...(result && !result.ok ? result.errors : []),
          ...(p.template ? templateErrors(value) : []),
          ...(presets && typeof value === "string" && !presets.includes(value)
            ? [{ path: "", message: presetMessage(value) }]
            : []),
        ];
        if (found.length > 0)
          errors.push({
            path: at,
            message: found
              .map((e) => (e.path ? `${e.path}: ${e.message}` : e.message))
              .join(" "),
          });
      };
      if (p.value !== undefined) check(p.schema, p.value, `${path}.value`);
      const items =
        p.schema?.type === "array" && isObject(p.schema.items)
          ? (p.schema.items as OpenAPISchema)
          : p.schema;
      p.allowed.forEach((choice, row) =>
        check(
          Array.isArray(choice.value) ? p.schema : items,
          choice.value,
          `${path}.allowed.${row}.value`,
        ),
      );
    }
  }
  return errors;
}

/** presetsOf returns the presets of kind that spec lists. */
export function presetsOf(
  spec: Spec,
  kind: NonNullable<ParameterSetting["presetKind"]>,
): string[] {
  const list =
    kind === "ElasticsearchClusterPreset"
      ? spec.presets?.elasticsearchCluster
      : spec.presets?.databaseServer;
  return list ?? [];
}

/** presetMessage is the error of a preset parameter that names the preset name, which the environment does not list. */
export function presetMessage(name: string): string {
  return `The environment does not allow the preset ${name}. Check it under Presets, or pick another.`;
}
