import type { Environment } from "@/generated/crds/environment";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { BUILTIN_OPTIONS } from "./builtins";
import {
  byCodeUnit,
  decodeObject,
  equalJSON,
  listOfNames,
  nameOf,
  NOT_AN_OBJECT,
  own,
} from "./json";
import {
  hasValue,
  requestParameters,
  type ParameterPolicies,
  type ParameterPolicy,
  type ResolvedPolicy,
} from "./resolve";
import type {
  ClaimKind,
  PluginSelection,
  PolicyResult,
  Refusal,
} from "./types";

/**
 * AllowedInput holds what checkAllowed reads. environment is the current
 * Environment of the namespace. kind is the kind of the claim. A section of
 * the policy that is not in sections applies to every claim kind. policy is
 * the snapshot of the claim. presetRef and releaseRef are absent for a plane
 * claim. requests are the live PluginRequests of the claim.
 */
export type AllowedInput = (
  | { kind?: undefined; sections?: undefined }
  | { kind: ClaimKind; sections: readonly PluginSection[] }
) & {
  environment?: Environment;
  policy: ResolvedPolicy;
  presetRef?: string;
  releaseRef?: string;
  selections?: PluginSelection[];
  requests?: readonly PluginRequest[];
};

// The Environment list of each preset kind of a built-in registration.
const PRESET_LISTS = {
  ElasticsearchClusterPreset: "elasticsearchCluster",
  DatabaseServerPreset: "databaseServer",
} as const;

/**
 * checkAllowed refuses with NotAllowed the first value of a claim that its
 * policy does not allow, by the rule of CheckAllowed of pkg/claimpolicy, in
 * this order: the preset, the preset of each built-in storage selection in
 * claim order, the release, each selection in claim order in a
 * section whose appliesTo does not list kind, each registration in claim
 * order, each required section that applies to kind and has no selection in
 * name order, and each parameter.
 * The preset of a built-in storage selection is the preset parameter of its
 * request, and it must be in spec.presets.elasticsearchCluster or
 * spec.presets.databaseServer, by the kind of the preset. A preset that the
 * request in requests of the same section and registration already carries
 * passes. An empty list allows no preset.
 * Without an Environment, every preset and release passes. The registrations
 * and the parameters are checked against the snapshot. A key that is not
 * claim-editable passes only with the value of the snapshot, and a null
 * value of the snapshot is no value. A claim-editable key with choices takes
 * only a name of a choice or a list of names. parameters of a selection that
 * is not a JSON object is an error. The value is an empty object when the
 * claim passes.
 */
export function checkAllowed(
  input: AllowedInput,
): PolicyResult<Record<string, never>> {
  const { environment, kind, policy, selections = [], sections = [] } = input;
  if (environment) {
    const env = environment.metadata.name ?? "";
    if (
      input.presetRef &&
      !(environment.spec.presets?.camundaCluster ?? []).includes(
        input.presetRef,
      )
    ) {
      return notAllowed(
        `CamundaClusterPreset "${input.presetRef}" is not in spec.presets.camundaCluster of Environment "${env}"`,
      );
    }
    const refused = checkStoragePresets(
      environment,
      policy,
      selections,
      input.requests ?? [],
    );
    if (refused) return refused;
    if (input.releaseRef && !releaseAllowed(input.releaseRef, environment)) {
      return notAllowed(
        `CamundaRelease "${input.releaseRef}" is not in spec.releases.allowed of Environment "${env}"`,
      );
    }
  }

  for (const s of selections) {
    if (!appliesTo(s.section, kind, sections)) {
      return notAllowed(`section "${s.section}" does not apply to a ${kind}`);
    }
  }

  const selected = new Set<string>();
  for (const s of selections) {
    if (
      !(own(policy.sections ?? {}, s.section)?.allowed ?? []).includes(
        s.registration,
      )
    ) {
      return notAllowed(
        `registration "${s.registration}" is not allowed in section "${s.section}"`,
      );
    }
    selected.add(s.section);
  }

  for (const section of Object.keys(policy.sections ?? {}).sort(byCodeUnit)) {
    if (
      policy.sections?.[section].required &&
      !selected.has(section) &&
      appliesTo(section, kind, sections)
    ) {
      return notAllowed(
        `section "${section}" is required, and the claim selects nothing in it`,
      );
    }
  }

  for (const s of selections) {
    const refused = checkParameters(
      s,
      own(policy.parameters ?? {}, s.registration) ?? {},
    );
    if (refused) return refused;
  }

  return { value: {} };
}

/**
 * releaseAllowed says whether environment allows the release name. Every
 * release is allowed without an Environment, and none when the Environment
 * has no spec.releases.
 */
export function releaseAllowed(
  release: string,
  environment: Environment | undefined,
): boolean {
  if (!environment) return true;
  return (environment.spec.releases?.allowed ?? []).includes(release);
}

function appliesTo(
  section: string,
  kind: ClaimKind | undefined,
  sections: readonly PluginSection[],
): boolean {
  const found = sections.find((s) => s.metadata.name === section);
  return !found || (kind !== undefined && found.spec.appliesTo.includes(kind));
}

function checkStoragePresets(
  environment: Environment,
  policy: ResolvedPolicy,
  selections: readonly PluginSelection[],
  requests: readonly PluginRequest[],
): { refusal: Refusal } | { error: string } | undefined {
  for (const s of selections) {
    const kind = own(BUILTIN_OPTIONS, s.registration)?.lists.preset;
    if (
      kind !== "ElasticsearchClusterPreset" &&
      kind !== "DatabaseServerPreset"
    )
      continue;
    const parameters = requestParameters(
      s,
      own(policy.parameters ?? {}, s.registration) ?? {},
    );
    if (!("value" in parameters)) return parameters;
    const preset = presetOf(parameters.value);
    const list = PRESET_LISTS[kind];
    if (
      preset === undefined ||
      (environment.spec.presets?.[list] ?? []).includes(preset) ||
      requests.some(
        (r) =>
          r.spec.section === s.section &&
          r.spec.registration === s.registration &&
          presetOf(r.spec.parameters) === preset,
      )
    )
      continue;
    return notAllowed(
      `${kind} "${preset}" is not in spec.presets.${list} of Environment "${environment.metadata.name ?? ""}"`,
    );
  }
  return undefined;
}

// A preset that is not a name is left to the parameter schema of the
// registration.
function presetOf(parameters: unknown): string | undefined {
  return nameOf(decodeObject(parameters)?.preset);
}

function checkParameters(
  selection: PluginSelection,
  policies: ParameterPolicies,
): { refusal: Refusal } | { error: string } | undefined {
  const claimed = decodeObject(selection.parameters);
  if (!claimed) {
    return {
      error: `parameters of the selection of section "${selection.section}": ${NOT_AN_OBJECT}`,
    };
  }

  for (const key of Object.keys(claimed).sort(byCodeUnit)) {
    const policy: ParameterPolicy = own(policies, key) ?? {};
    const value = claimed[key];
    if (!policy.claimEditable) {
      if (!hasValue(policy) || !equalJSON(value, policy.value)) {
        return notAllowed(
          `parameter "${key}" of registration "${selection.registration}" is not claim-editable`,
        );
      }
      continue;
    }

    const known = (policy.allowed ?? []).map((c) => c.name);
    if (known.length === 0) continue;
    const name = nameOf(value);
    const names = name !== undefined ? [name] : listOfNames(value);
    if (!names) {
      return notAllowed(
        `parameter "${key}" of registration "${selection.registration}" takes a name of its choices or a list of names`,
      );
    }
    const unknown = names.find((n) => !known.includes(n));
    if (unknown !== undefined) {
      return notAllowed(
        `parameter "${key}" of registration "${selection.registration}" has the name "${unknown}", which is not one of ${known.join(", ")}`,
      );
    }
  }

  return undefined;
}

function notAllowed(message: string): { refusal: Refusal } {
  return { refusal: { reason: "NotAllowed", message } };
}
