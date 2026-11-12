import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { OpenAPISchema } from "@/lib/schema";
import { missingInputs } from "./inputs";
import { byCodeUnit, equalJSON, isObject, own } from "./json";
import { parameterPolicies, sectionPolicy } from "./resolve";
import type {
  ClaimKind,
  OptionView,
  ParameterRule,
  PluginSelection,
  SectionView,
} from "./types";

/**
 * claimEditable returns one rule for each parameter of registration: the
 * properties of its parameter schema in schema order, then the other keys of
 * its policy in code-unit order. label is the title of the property schema,
 * else the name. A key that is not a property has the additionalProperties
 * schema, when it is one. required follows the required list of the schema. A
 * claim-editable parameter carries its policy value and the names and labels
 * of its choices, and allowedLists when every choice is a list. A value of a
 * choice, or a list of such values, is given as the names of the choices. A
 * locked parameter carries lockedSummary of its value and choices, and no value. A
 * parameterValues that is not an object counts as no values.
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
  const otherValues = isObject(schema.additionalProperties)
    ? (schema.additionalProperties as OpenAPISchema)
    : undefined;
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
      ...((property ?? otherValues) && { schema: property ?? otherValues }),
      ...(required.includes(name) && { required: true }),
    };
    const policy = own(policies, name) ?? {};
    if (!policy.claimEditable) {
      return {
        ...base,
        claimEditable: false,
        summary: lockedSummary(policy.value, policy.allowed),
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
      ...(policy.allowed?.length &&
        policy.allowed.every((c) => Array.isArray(c.value)) && {
          allowedLists: true,
        }),
    };
  });
}

/**
 * lockedSummary says what a locked value holds: "Not set" for no value or
 * null, the text of a string, number, or boolean, "No items", "1 item", or
 * "<n> items" for a list, and "No fields", "1 field", or "<n> fields" for an
 * object. A value other than null that one of choices holds shows as the
 * label of that choice. Any other string that starts with "arn:" shows as
 * "Set by your admin".
 */
export function lockedSummary(
  value: unknown,
  choices: readonly { name: string; label: string; value?: unknown }[] = [],
): string {
  if (value === undefined || value === null) return "Not set";
  const choice = choices.find((c) => equalJSON(c.value, value));
  if (choice) return choice.label;
  if (typeof value === "string" && value.startsWith("arn:")) {
    return "Set by your admin";
  }
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
 * or is not Accepted=True and Ready=True, is left out: offeredSection shows
 * the accepted ones that serve the section. An option has no choices when its
 * registration lists none, or when its choice parameter ("preset" if the
 * schema has it, else "name") is claim-editable and the Environment gives it
 * an allowed list. That list is then in parameters. disabledReason stays
 * unset: withMissingInputs sets it.
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

/**
 * offeredSection returns sectionOptions, plus each allowed registration that
 * serves the section and is Accepted=True but not Ready=True, at its place
 * in the allowed order, disabled with "Unavailable right now: <Ready
 * message>" ("Unavailable right now" without a message). Such an option
 * keeps isDefault, so it shows the Default label, and SectionCards does not
 * preselect it. What a claim can select does not change.
 */
export function offeredSection(
  section: PluginSection,
  registrations: readonly PluginRegistration[],
  environment: Environment | undefined,
  claimKind: ClaimKind,
): SectionView {
  const view = sectionOptions(section, registrations, environment, claimKind);
  const name = view.name;
  const policy = section.spec.appliesTo.includes(claimKind)
    ? sectionPolicy(name, environment, registrations)
    : undefined;
  if (!policy) return view;

  const options = (policy.allowed ?? []).flatMap((allowed): OptionView[] => {
    const offered = view.options.find((o) => o.registration === allowed);
    if (offered) return [offered];
    const registration = registrations.find((r) => r.metadata.name === allowed);
    if (
      !registration?.spec.sections.includes(name) ||
      !conditionTrue(registration, "Accepted")
    ) {
      return [];
    }
    const message = conditionOf(registration, "Ready")?.message;
    return [
      {
        ...optionOf(registration, environment, allowed === policy.default),
        disabledReason: message
          ? `Unavailable right now: ${message}`
          : "Unavailable right now",
      },
    ];
  });
  return { ...view, options };
}

/**
 * withMissingInputs disables each option of view that has a required input
 * whose section has no selection in selections (missingInputs), with
 * "Needs <a|an> <section label in lower case>", one phrase for each such
 * section, joined by " and ". A section that no PluginSection of sections
 * has is named by its name. An option that is disabled already keeps its
 * reason.
 */
export function withMissingInputs(
  view: SectionView,
  selections: readonly PluginSelection[],
  registrations: readonly PluginRegistration[],
  sections: readonly PluginSection[],
): SectionView {
  return {
    ...view,
    options: view.options.map((option) => {
      if (option.disabledReason) return option;
      const missing = missingInputs(
        { section: view.name, registration: option.registration },
        selections,
        registrations,
      );
      if (missing.length === 0) return option;
      const phrases = [...new Set(missing.map((input) => input.section))].map(
        (name) => {
          const label =
            sections.find((s) => s.metadata.name === name)?.spec.label ?? name;
          const noun = label.charAt(0).toLowerCase() + label.slice(1);
          return `${/^[aeiou]/.test(noun) ? "an" : "a"} ${noun}`;
        },
      );
      return { ...option, disabledReason: `Needs ${phrases.join(" and ")}` };
    }),
  };
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
  return ["Accepted", "Ready"].every((type) =>
    conditionTrue(registration, type),
  );
}

/** conditionTrue says whether object has a condition of type with status True. */
export function conditionTrue(
  object: { status?: { conditions?: { type: string; status: string }[] } },
  type: string,
): boolean {
  return (
    object.status?.conditions?.some(
      (c) => c.type === type && c.status === "True",
    ) ?? false
  );
}

function conditionOf(registration: PluginRegistration, type: string) {
  return registration.status?.conditions?.find((c) => c.type === type);
}

// The values of the choices stay with the claim controller, and the
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
