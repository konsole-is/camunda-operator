import type { Environment } from "@/generated/crds/environment";
import {
  ALLOW_VERSION_DOWNGRADE_ANNOTATION,
  CLAIM_LABEL,
  ENVIRONMENT_LABEL,
} from "@/lib/labels";
import {
  BUILTIN_OPTIONS,
  byCodeUnit,
  type ClaimKind,
  equalJSON,
  nameOf,
  own,
  type PluginSelection,
  releaseAllowed,
  resolve,
  type ResolvedPolicy,
} from "@/lib/policy";
import { specOf, type EnvironmentDraft } from "./draft";
import { labelledNamespaces } from "./rows";
import type { EnvironmentSources } from "./sources";

/** StoragePresetKind is the preset kind of the built-in Elasticsearch or PostgreSQL. */
export type StoragePresetKind =
  | "ElasticsearchClusterPreset"
  | "DatabaseServerPreset";

// The Environment list of each preset kind of the built-in storage.
const STORAGE_PRESET_LISTS = {
  ElasticsearchClusterPreset: "elasticsearchCluster",
  DatabaseServerPreset: "databaseServer",
} as const;

/** ClaimRef names one claim of either kind. */
export type ClaimRef = { kind: ClaimKind; namespace: string; name: string };

/**
 * ReferenceWarning is a claim that an edit touches although it keeps its
 * recorded policy. removed: the edit no longer allows the release or the
 * preset of the claim. followsDefault: the claim names no release or no
 * platform config, so it moves to the new default, name. refused is set
 * when the new default release has a version below the running version of
 * the cluster that no approval allows, or more than one minor above it in
 * the same major. It is VersionDowngradeRefused or VersionSkipRefused, and
 * it is not set when a version is missing or not x.y.z. cleared: the claim names none, and the
 * edit removes the default.
 */
export type ReferenceWarning =
  | {
      claim: ClaimRef;
      kind:
        | "CamundaRelease"
        | "CamundaClusterPreset"
        | StoragePresetKind;
      name: string;
      removed: true;
    }
  | {
      claim: ClaimRef;
      kind: "CamundaRelease" | "CamundaPlatformConfig";
      name: string;
      followsDefault: true;
      refused?: "VersionDowngradeRefused" | "VersionSkipRefused";
    }
  | {
      claim: ClaimRef;
      kind: "CamundaRelease" | "CamundaPlatformConfig";
      cleared: true;
    };

/**
 * EnvironmentImpact is what an edit does to the claims of the environment.
 * clusters and planes count the claims whose recorded policy differs from
 * the policy of the edit.
 */
export type EnvironmentImpact = {
  clusters: number;
  planes: number;
  warnings: ReferenceWarning[];
  /**
   * unassigned are the namespaces that hold claims and no environment label
   * when the edit creates the first environment, sorted.
   */
  unassigned?: string[];
};

type Claim = {
  kind: ClaimKind;
  metadata: { namespace?: string; name?: string };
  spec: {
    plugins?: PluginSelection[];
    platformConfigRef?: string;
    releaseRef?: string;
    presetRef?: string;
    allowVersionDowngrade?: string;
  };
  status?: { resolvedPolicy?: ResolvedPolicy };
};

/**
 * environmentImpact compares the edited spec with live for the claims in
 * the namespaces of the environment. A claim without a recorded policy is
 * not counted. A new environment (live undefined) covers the claims of the
 * namespaces that already carry its label.
 */
export function environmentImpact(
  live: Environment | undefined,
  draft: EnvironmentDraft,
  s: EnvironmentSources,
): EnvironmentImpact {
  const impact: EnvironmentImpact = { clusters: 0, planes: 0, warnings: [] };
  const before: Environment = live ?? {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name: draft.name },
    spec: {},
  };
  const edited: Environment = { ...before, spec: specOf(draft) };
  const namespaces = new Set(labelledNamespaces(draft.name, s.namespaces));
  // The CRD schema gives spec.plugins[].parameters no shape, and Claim
  // reads it as the policy library does.
  const claims = (
    [...s.clusterClaims, ...s.planeClaims] as unknown as Claim[]
  ).filter((c) => namespaces.has(c.metadata.namespace ?? ""));
  for (const claim of claims) {
    const ref: ClaimRef = {
      kind: claim.kind,
      namespace: claim.metadata.namespace ?? "",
      name: claim.metadata.name ?? "",
    };
    if (policyDiffers(claim, edited, s)) {
      if (claim.kind === "CamundaClusterClaim") impact.clusters++;
      else impact.planes++;
    }
    impact.warnings.push(
      ...warningsOf(ref, claim, before, edited).map((w) =>
        refusedMove(w, claim, runningVersion(claim, before, s), s),
      ),
      ...storagePresetWarnings(ref, claim, before, edited, s),
    );
  }
  if (!live && s.environments.length === 0) {
    const unlabelled = new Set(
      s.namespaces
        .filter((n) => !n.metadata?.labels?.[ENVIRONMENT_LABEL])
        .map((n) => n.metadata?.name ?? ""),
    );
    const unassigned = [
      ...new Set(
        [...s.clusterClaims, ...s.planeClaims]
          .map((c) => c.metadata.namespace ?? "")
          .filter((ns) => unlabelled.has(ns)),
      ),
    ].sort(byCodeUnit);
    if (unassigned.length > 0) impact.unassigned = unassigned;
  }
  return impact;
}

// runningVersion returns the version that the cluster of claim reports, or
// the version of the default release of before when the cluster reports
// none.
function runningVersion(
  claim: Claim,
  before: Environment | undefined,
  s: EnvironmentSources,
): string | undefined {
  return (
    clusterOf(claim, s)?.status?.management?.version ||
    s.releases.find((r) => r.metadata.name === before?.spec.releases?.default)
      ?.spec.version
  );
}

/**
 * namespaceImpact returns a followsDefault warning for each claim in
 * namespace that names no release or no platform config while environment
 * has a default for it: the label of environment moves such a claim to that
 * default.
 */
export function namespaceImpact(
  namespace: string,
  environment: Environment,
  s: EnvironmentSources,
): ReferenceWarning[] {
  const claims = (
    [...s.clusterClaims, ...s.planeClaims] as unknown as Claim[]
  ).filter((c) => c.metadata.namespace === namespace);
  return claims.flatMap((claim): ReferenceWarning[] => {
    const ref: ClaimRef = {
      kind: claim.kind,
      namespace,
      name: claim.metadata.name ?? "",
    };
    const running = runningVersion(claim, undefined, s);
    const release = environment.spec.releases?.default;
    const config = environment.spec.platformConfigRef;
    const warnings: ReferenceWarning[] = [
      ...(claim.kind === "CamundaClusterClaim" &&
      !claim.spec.releaseRef &&
      release
        ? [
            {
              claim: ref,
              kind: "CamundaRelease" as const,
              name: release,
              followsDefault: true as const,
            },
          ]
        : []),
      ...(!claim.spec.platformConfigRef && config
        ? [
            {
              claim: ref,
              kind: "CamundaPlatformConfig" as const,
              name: config,
              followsDefault: true as const,
            },
          ]
        : []),
    ];
    return warnings.map((w) => refusedMove(w, claim, running, s));
  });
}

function policyDiffers(
  claim: Claim,
  edited: Environment,
  s: EnvironmentSources,
): boolean {
  const recorded = claim.status?.resolvedPolicy;
  if (!recorded) return false;
  const before = resolve({ recorded });
  const after = resolve({
    environment: edited,
    registrations: s.registrations,
    selections: claim.spec.plugins ?? [],
  });
  if (!("value" in before) || !("value" in after)) return true;
  return !equalJSON(before.value.policy, after.value.policy);
}

function warningsOf(
  ref: ClaimRef,
  claim: Claim,
  live: Environment,
  edited: Environment,
): ReferenceWarning[] {
  const warnings: ReferenceWarning[] = [];
  if (claim.kind === "CamundaClusterClaim") {
    const release = claim.spec.releaseRef;
    if (release) {
      if (releaseAllowed(release, live) && !releaseAllowed(release, edited))
        warnings.push({
          claim: ref,
          kind: "CamundaRelease",
          name: release,
          removed: true,
        });
    } else {
      warnings.push(
        ...defaultWarnings(
          ref,
          "CamundaRelease",
          live.spec.releases?.default,
          edited.spec.releases?.default,
        ),
      );
    }
    const preset = claim.spec.presetRef;
    if (preset && presetAllowed(preset, live) && !presetAllowed(preset, edited))
      warnings.push({
        claim: ref,
        kind: "CamundaClusterPreset",
        name: preset,
        removed: true,
      });
  }
  if (!claim.spec.platformConfigRef)
    warnings.push(
      ...defaultWarnings(
        ref,
        "CamundaPlatformConfig",
        live.spec.platformConfigRef,
        edited.spec.platformConfigRef,
      ),
    );
  return warnings;
}

// The preset of a built-in storage selection is the preset parameter of its
// request, from the claim or from the policy that the claim recorded.
function storagePresetWarnings(
  ref: ClaimRef,
  claim: Claim,
  live: Environment,
  edited: Environment,
  s: EnvironmentSources,
): ReferenceWarning[] {
  const selections = claim.spec.plugins ?? [];
  const resolved = resolve({
    recorded: claim.status?.resolvedPolicy,
    environment: live,
    registrations: s.registrations,
    selections,
  });
  if (!("value" in resolved)) return [];
  return selections.flatMap((selection, i): ReferenceWarning[] => {
    const kind = own(BUILTIN_OPTIONS, selection.registration)?.lists.preset;
    if (
      kind !== "ElasticsearchClusterPreset" &&
      kind !== "DatabaseServerPreset"
    )
      return [];
    const name = nameOf(resolved.value.requests?.[i]?.parameters?.preset);
    const list = STORAGE_PRESET_LISTS[kind];
    const allows = (environment: Environment) =>
      name !== undefined &&
      (environment.spec.presets?.[list] ?? []).includes(name);
    return name !== undefined && allows(live) && !allows(edited)
      ? [{ claim: ref, kind, name, removed: true }]
      : [];
  });
}

function defaultWarnings(
  claim: ClaimRef,
  kind: "CamundaRelease" | "CamundaPlatformConfig",
  before: string | undefined,
  after: string | undefined,
): ReferenceWarning[] {
  if (after && after !== before)
    return [{ claim, kind, name: after, followsDefault: true }];
  if (!after && before) return [{ claim, kind, cleared: true }];
  return [];
}

// The cluster refuses an effective version below the version the brokers
// run, or more than one minor above it in the same major.
function refusedMove(
  warning: ReferenceWarning,
  claim: Claim,
  running: string | undefined,
  s: EnvironmentSources,
): ReferenceWarning {
  if (!("followsDefault" in warning) || warning.kind !== "CamundaRelease")
    return warning;
  const version = s.releases.find((r) => r.metadata.name === warning.name)?.spec
    .version;
  const have = parseVersion(running);
  const want = parseVersion(version);
  if (!have || !want) return warning;
  const approved =
    claim.spec.allowVersionDowngrade === version ||
    clusterOf(claim, s)?.metadata?.annotations?.[
      ALLOW_VERSION_DOWNGRADE_ANNOTATION
    ] === version;
  const index = want.findIndex((n, i) => n !== have[i]);
  if (index >= 0 && want[index] < have[index])
    return approved
      ? warning
      : { ...warning, refused: "VersionDowngradeRefused" };
  if (want[0] === have[0] && want[1] > have[1] + 1)
    return { ...warning, refused: "VersionSkipRefused" };
  return warning;
}

function clusterOf(claim: Claim, s: EnvironmentSources) {
  return s.clusters.find(
    (c) =>
      c.metadata?.namespace === claim.metadata.namespace &&
      c.metadata?.labels?.[CLAIM_LABEL] === claim.metadata.name,
  );
}

function parseVersion(version: string | undefined): number[] | undefined {
  const match = /^(\d+)\.(\d+)\.(\d+)$/.exec(version ?? "");
  return match ? match.slice(1).map(Number) : undefined;
}

/**
 * withoutKept returns impact without each removed warning whose release or
 * preset kept, the spec that the dry run of the save gives, still allows,
 * and without each cleared warning whose default kept still has. Another
 * field manager keeps such a member. Without kept it returns impact.
 */
export function withoutKept(
  impact: EnvironmentImpact,
  kept: Environment["spec"] | undefined,
): EnvironmentImpact {
  if (!kept) return impact;
  const still = (w: ReferenceWarning) => {
    if ("cleared" in w)
      return w.kind === "CamundaRelease"
        ? Boolean(kept.releases?.default)
        : Boolean(kept.platformConfigRef);
    if (!("removed" in w)) return false;
    const list =
      w.kind === "CamundaRelease"
        ? kept.releases?.allowed
        : w.kind === "CamundaClusterPreset"
          ? kept.presets?.camundaCluster
          : kept.presets?.[STORAGE_PRESET_LISTS[w.kind]];
    return (list ?? []).includes(w.name);
  };
  return { ...impact, warnings: impact.warnings.filter((w) => !still(w)) };
}

function presetAllowed(preset: string, environment: Environment): boolean {
  return (environment.spec.presets?.camundaCluster ?? []).includes(preset);
}
