import type { CamundaClusterClaimStatus } from "@/generated/crds/camundaclusterclaim";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import {
  byCodeUnit,
  decodeObject,
  listOfNames,
  nameOf,
  NOT_AN_OBJECT,
  own,
  setOwn,
} from "./json";
import type { PluginSelection, PolicyResult } from "./types";

export type ResolvedPolicy = NonNullable<
  CamundaClusterClaimStatus["resolvedPolicy"]
>;
export type SectionPolicy = NonNullable<ResolvedPolicy["sections"]>[string];
export type ParameterPolicy = NonNullable<
  ResolvedPolicy["parameters"]
>[string][string];
export type ParameterPolicies = Record<string, ParameterPolicy>;
type Choice = NonNullable<ParameterPolicy["allowed"]>[number];

/** ResolveInput holds what resolve reads. recorded is status.resolvedPolicy of the claim, absent at creation. */
export type ResolveInput = {
  recorded?: ResolvedPolicy;
  environment?: Environment;
  registrations?: PluginRegistration[];
  selections?: PluginSelection[];
};

/** RequestParameters are the parameters of the request of one selection. parameters is absent when the request has none. */
export type RequestParameters = {
  section: string;
  registration: string;
  parameters?: Record<string, unknown>;
};

/** Resolution is the snapshot of a claim and the parameters of its requests, one for each selection in claim order. */
export type Resolution = {
  policy: ResolvedPolicy;
  requests?: RequestParameters[];
};

/**
 * resolve returns the policy snapshot of a claim and the parameters of each
 * of its requests, by the rule of Resolve of pkg/claimpolicy. With recorded,
 * the policy is a copy of it, so a later change of the Environment or of a
 * registration does not change it. A claim-editable key of a selection
 * replaces the policy value, and a name of a choice, or a list of names,
 * becomes the value of the choice. A name that is not a choice stays as it
 * is, because checkAllowed refuses it.
 *
 * It refuses with InvalidReference a selected registration that does not
 * exist. parameterValues of a registration or parameters of a selection that
 * is not a JSON object is an error.
 */
export function resolve(input: ResolveInput): PolicyResult<Resolution> {
  const policy = snapshot(input);
  if (!("value" in policy)) return policy;

  const requests: RequestParameters[] = [];
  for (const s of input.selections ?? []) {
    const parameters = requestParameters(
      s,
      policy.value.parameters?.[s.registration] ?? {},
    );
    if (!("value" in parameters)) return parameters;
    requests.push({
      section: s.section,
      registration: s.registration,
      ...(parameters.value && { parameters: parameters.value }),
    });
  }

  return {
    value: {
      policy: policy.value,
      ...(requests.length > 0 && { requests }),
    },
  };
}

/**
 * sectionPolicy returns the policy of section. With an Environment it is the
 * entry of spec.sections, or undefined when it has none. Without one, allowed
 * is the names of the registrations that serve the section, sorted, and
 * default is the one with mode Default when exactly one has it.
 */
export function sectionPolicy(
  section: string,
  environment: Environment | undefined,
  registrations: readonly PluginRegistration[],
): SectionPolicy | undefined {
  if (environment) {
    const entry = environment.spec.sections?.[section];
    return entry && compactSection(entry);
  }

  const serving = registrations.filter((r) =>
    r.spec.sections.includes(section),
  );
  const defaults = serving.filter((r) => r.spec.mode === "Default");
  return compactSection({
    allowed: serving.map((r) => r.metadata.name ?? "").sort(byCodeUnit),
    default: defaults.length === 1 ? defaults[0].metadata.name : undefined,
  });
}

/**
 * parameterPolicies returns the policy of each parameter of registration.
 * Each key of parameterValues gets its value. With an Environment, its entry
 * in spec.parameters sets claimEditable and allowed, replaces the value when
 * it has one that is not null, and adds its own keys. Without one, the keys
 * of claimEditable of the registration are claim-editable. parameterValues
 * that is not a JSON object is an error.
 */
export function parameterPolicies(
  registration: PluginRegistration,
  environment: Environment | undefined,
): PolicyResult<ParameterPolicies> {
  const name = registration.metadata.name ?? "";
  const values = decodeObject(registration.spec.parameterValues);
  if (!values) {
    return {
      error: `parameterValues of PluginRegistration "${name}": ${NOT_AN_OBJECT}`,
    };
  }

  const policies: ParameterPolicies = {};
  for (const [key, value] of Object.entries(values)) {
    setOwn(policies, key, { value: structuredClone(value) });
  }

  if (!environment) {
    for (const key of registration.spec.claimEditable ?? []) {
      setOwn(policies, key, { ...own(policies, key), claimEditable: true });
    }
    return { value: policies };
  }

  for (const [key, entry] of Object.entries(
    environment.spec.parameters?.[name] ?? {},
  )) {
    const policy: ParameterPolicy = { ...own(policies, key) };
    delete policy.claimEditable;
    delete policy.allowed;
    if (entry.claimEditable) policy.claimEditable = true;
    if (entry.allowed?.length) policy.allowed = entry.allowed.map(choiceOf);
    if (entry.value !== undefined && entry.value !== null) {
      policy.value = structuredClone(entry.value);
    }
    setOwn(policies, key, policy);
  }

  return { value: policies };
}

function snapshot(input: ResolveInput): PolicyResult<ResolvedPolicy> {
  if (input.recorded) return { value: structuredClone(input.recorded) };

  const { environment, registrations = [], selections = [] } = input;
  const policy: ResolvedPolicy = {
    deletionPolicy: environment?.spec.deletionPolicy || "Delete",
  };

  const sections = environment
    ? Object.keys(environment.spec.sections ?? {})
    : selections.map((s) => s.section);
  for (const section of sections) {
    const entry = sectionPolicy(section, environment, registrations);
    if (entry) policy.sections = { ...policy.sections, [section]: entry };
  }

  for (const s of selections) {
    const registration = registrations.find(
      (r) => r.metadata.name === s.registration,
    );
    if (!registration) {
      return {
        refusal: {
          reason: "InvalidReference",
          message: `PluginRegistration "${s.registration}" does not exist`,
        },
      };
    }

    const parameters = parameterPolicies(registration, environment);
    if (!("value" in parameters)) return parameters;
    policy.parameters = {
      ...policy.parameters,
      [s.registration]: parameters.value,
    };
  }

  return { value: policy };
}

function requestParameters(
  selection: PluginSelection,
  policies: ParameterPolicies,
): PolicyResult<Record<string, unknown> | undefined> {
  const claimed = decodeObject(selection.parameters);
  if (!claimed) {
    return {
      error: `parameters of the selection of section "${selection.section}": ${NOT_AN_OBJECT}`,
    };
  }

  const object: Record<string, unknown> = {};
  for (const [key, policy] of Object.entries(policies)) {
    if ("value" in policy) setOwn(object, key, policy.value);
  }
  for (const [key, value] of Object.entries(claimed)) {
    const policy = own(policies, key);
    if (policy?.claimEditable) {
      setOwn(object, key, valuesOfNames(value, policy.allowed ?? []));
    }
  }

  return {
    value: Object.keys(object).length > 0 ? structuredClone(object) : undefined,
  };
}

function valuesOfNames(value: unknown, choices: readonly Choice[]): unknown {
  if (choices.length === 0) return value;

  const name = nameOf(value);
  if (name !== undefined) return valueOfName(name, choices);

  return listOfNames(value)?.map((n) => valueOfName(n, choices)) ?? value;
}

function valueOfName(name: string, choices: readonly Choice[]): unknown {
  const choice = choices.find(
    (c) => c.name === name && c.value !== undefined && c.value !== null,
  );
  return choice ? choice.value : name;
}

// The operator writes a choice without a value as null.
function choiceOf(choice: Choice): Choice {
  return {
    name: choice.name,
    label: choice.label,
    value: structuredClone(choice.value ?? null),
  };
}

// The API drops a field with its empty value, so a copy does too.
function compactSection(policy: SectionPolicy): SectionPolicy {
  return {
    ...(policy.required && { required: true }),
    ...(policy.default && { default: policy.default }),
    ...(policy.allowed?.length && { allowed: [...policy.allowed] }),
  };
}
