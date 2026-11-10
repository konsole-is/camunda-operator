import type { Environment } from "@/generated/crds/environment";
import {
  type ClaimKind,
  equalJSON,
  type PluginSelection,
  releaseAllowed,
  resolve,
  type ResolvedPolicy,
} from "@/lib/policy";
import { specOf, type EnvironmentDraft } from "./draft";
import { labelledNamespaces } from "./rows";
import type { EnvironmentSources } from "./sources";

/** ClaimRef names one claim of either kind. */
export type ClaimRef = { kind: ClaimKind; namespace: string; name: string };

/**
 * ReferenceWarning is a claim that an edit touches although it keeps its
 * recorded policy. removed: the edit no longer allows the release or the
 * preset of the claim. followsDefault: the claim names no release or no
 * platform config, so it moves to the new default, name. cleared: the
 * claim names none, and the edit removes the default.
 */
export type ReferenceWarning =
  | {
      claim: ClaimRef;
      kind: "CamundaRelease" | "CamundaClusterPreset";
      name: string;
      removed: true;
    }
  | {
      claim: ClaimRef;
      kind: "CamundaRelease" | "CamundaPlatformConfig";
      name: string;
      followsDefault: true;
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
};

type Claim = {
  kind: ClaimKind;
  metadata: { namespace?: string; name?: string };
  spec: {
    plugins?: PluginSelection[];
    platformConfigRef?: string;
    releaseRef?: string;
    presetRef?: string;
  };
  status?: { resolvedPolicy?: ResolvedPolicy };
};

/**
 * environmentImpact compares the edited spec with live for the claims in
 * the namespaces of the environment. A claim without a recorded policy is
 * not counted. A new environment has no claims.
 */
export function environmentImpact(
  live: Environment | undefined,
  draft: EnvironmentDraft,
  s: EnvironmentSources,
): EnvironmentImpact {
  const impact: EnvironmentImpact = { clusters: 0, planes: 0, warnings: [] };
  if (!live) return impact;
  const edited: Environment = { ...live, spec: specOf(draft) };
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
    impact.warnings.push(...warningsOf(ref, claim, live, edited));
  }
  return impact;
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
    const release = environment.spec.releases?.default;
    const config = environment.spec.platformConfigRef;
    return [
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

function presetAllowed(preset: string, environment: Environment): boolean {
  return (environment.spec.presets?.camundaCluster ?? []).includes(preset);
}
