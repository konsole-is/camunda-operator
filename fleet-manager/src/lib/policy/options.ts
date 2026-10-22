import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { OpenAPISchema } from "@/lib/schema";
import { byCodeUnit, equalJSON, isObject, own } from "./json";
import { parameterPolicies, sectionPolicy } from "./resolve";
import type {
  ClaimKind,
  OptionView,
  ParameterRule,
  SectionView,
} from "./types";

/**
 * claimEditable returns one rule for each parameter of registration: the
 * properties of its parameter schema in schema order, then the other keys of
 * its policy in code-unit order. label is the title of the property schema,
 * else the name. required follows the required list of the schema. A
 * claim-editable parameter carries its policy value and the names and labels
 * of its choices. A value of a choice, or a list of such values, is given as
 * the names of the choices. A locked parameter carries lockedSummary of its value and
 * no value. A parameterValues that is not an object counts as no values.
 */
export function claimEditable(
  registration: PluginRegistration,
  environment: Environment | undefined,
): ParameterRule[] {
  let result = parameterPolicies(registration, environment);
  if (!("value" in result)) {
    result = parameterPolicies(
      {
        ...registration,
        spec: { ...registration.spec, parameterValues: undefined },
      },
      environment,
    );
  }
  const policies = "value" in result ? result.value : {};

  const schema = isObject(registration.spec.parameters)
    ? (registration.spec.parameters as OpenAPISchema)
    : {};
  const properties = isObject(schema.properties) ? schema.properties : {};
  const required = Array.isArray(schema.required) ? schema.required : [];
  const names = [
    ...Object.keys(properties),
    ...Object.keys(policies)
      .filter((k) => !Object.hasOwn(properties, k))
      .sort(byCodeUnit),
  ];

  return names.map((name): ParameterRule => {
    const property = isObject(own(properties, name))
      ? (properties[name] as OpenAPISchema)
      : undefined;
    const base = {
      name,
      label:
        typeof property?.title === "string" && property.title !== ""
          ? property.title
          : name,
      ...(property && { schema: property }),
      ...(required.includes(name) && { required: true }),
    };
    const policy = own(policies, name) ?? {};
    if (!policy.claimEditable) {
      return {
        ...base,
        claimEditable: false,
        summary: lockedSummary(policy.value),
      };
    }
    return {
      ...base,
      claimEditable: true,
      ...("value" in policy && {
        value: namesOf(policy.value, policy.allowed ?? []),
      }),
      ...(policy.allowed && {
        allowed: policy.allowed.map((c) => ({ name: c.name, label: c.label })),
      }),
    };
  });
}

/**
 * lockedSummary says what a locked value holds without showing it: "Not set"
 * for no value or null, the text of a string, number, or boolean, "No items",
 * "1 item", or "<n> items" for a list, and "No fields", "1 field", or
 * "<n> fields" for an object.
 */
export function lockedSummary(value: unknown): string {
  if (value === undefined || value === null) return "Not set";
  if (Array.isArray(value)) return count(value.length, "item");
  if (typeof value === "object")
    return count(Object.keys(value).length, "field");
  return String(value);
}

/**
 * sectionOptions returns section as a claim author of claimKind sees it
 * under environment. A section that does not apply to claimKind, or that the
 * Environment does not list, has no options and is not required. Without an
 * Environment, the options are the registrations that serve the section, by
 * name, and the Default is the one registration with mode Default. With one,
 * they are the allowed registrations in the order of allowed, and the Default
 * is its default. A registration that is missing, does not serve the section,
 * or is not Accepted=True and Ready=True, is left out. An option has no choices when its registration
 * lists none, or when its choice parameter ("preset" if the schema has it,
 * else "name") is claim-editable and the Environment gives it an allowed
 * list. That list is then in parameters. disabledReason stays unset: the
 * caller sets it from missingInputs.
 */
export function sectionOptions(
  section: PluginSection,
  registrations: readonly PluginRegistration[],
  environment: Environment | undefined,
  claimKind: ClaimKind,
): SectionView {
  const name = section.metadata.name ?? "";
  const { spec } = section;
  const view: SectionView = {
    name,
    label: spec.label,
    ...(spec.description && { description: spec.description }),
    selection: spec.selection,
    ...(spec.emptyOptionLabel && { emptyOptionLabel: spec.emptyOptionLabel }),
    required: false,
    options: [],
  };

  const policy = spec.appliesTo.includes(claimKind)
    ? sectionPolicy(name, environment, registrations)
    : undefined;
  if (!policy) return view;

  view.required = policy.required ?? false;
  for (const allowed of policy.allowed ?? []) {
    const registration = registrations.find((r) => r.metadata.name === allowed);
    if (
      !registration ||
      !registration.spec.sections.includes(name) ||
      !ready(registration)
    ) {
      continue;
    }
    view.options.push(
      optionOf(registration, environment, allowed === policy.default),
    );
  }
  return view;
}

function optionOf(
  registration: PluginRegistration,
  environment: Environment | undefined,
  isDefault: boolean,
): OptionView {
  const { spec } = registration;
  const schema = isObject(spec.parameters) ? spec.parameters : {};
  const properties = isObject(schema.properties) ? schema.properties : {};
  const parameters = claimEditable(registration, environment);
  const parameter = Object.hasOwn(properties, "preset") ? "preset" : "name";
  const rule = parameters.find((r) => r.name === parameter);
  const aliased = rule?.claimEditable && rule.allowed;
  return {
    registration: registration.metadata.name ?? "",
    label: spec.label,
    ...(spec.description && { description: spec.description }),
    ...(spec.docsUrl && { docsUrl: spec.docsUrl }),
    isDefault,
    parameters,
    ...(isObject(spec.parameters) && {
      schema: spec.parameters as OpenAPISchema,
    }),
    ...(spec.choices?.length &&
      !aliased && { choices: { parameter, items: spec.choices } }),
  };
}

// Contract 3.6: the app offers a registration only while it is accepted and
// ready.
function ready(registration: PluginRegistration): boolean {
  const conditions = registration.status?.conditions ?? [];
  return ["Accepted", "Ready"].every((type) =>
    conditions.some((c) => c.type === type && c.status === "True"),
  );
}

// The values of the choices stay with the claim controller (CL40), and the
// cards pick choices by name.
function namesOf(
  value: unknown,
  choices: readonly { name: string; value: unknown }[],
): unknown {
  const nameOfValue = (v: unknown) =>
    choices.find((c) => equalJSON(c.value, v))?.name;
  const name = nameOfValue(value);
  if (name !== undefined || !Array.isArray(value)) return name ?? value;
  const names = value.map(nameOfValue);
  return names.every((n) => n !== undefined) ? names : value;
}

function count(n: number, noun: string): string {
  if (n === 0) return `No ${noun}s`;
  return n === 1 ? `1 ${noun}` : `${n} ${noun}s`;
}
