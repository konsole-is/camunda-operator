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
 * platform config, so it moves to the new default, name.
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
      kind: "CamundaRelease";
      name: string;
      followsDefault: true;
    }
  | {
      claim: ClaimRef;
      kind: "CamundaPlatformConfig";
      name: string;
      followsDefault: true;
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
      const next = edited.spec.releases?.default;
      if (next && next !== live.spec.releases?.default)
        warnings.push({
          claim: ref,
          kind: "CamundaRelease",
          name: next,
          followsDefault: true,
        });
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
  const config = edited.spec.platformConfigRef;
  if (
    !claim.spec.platformConfigRef &&
    config &&
    config !== live.spec.platformConfigRef
  )
    warnings.push({
      claim: ref,
      kind: "CamundaPlatformConfig",
      name: config,
      followsDefault: true,
    });
  return warnings;
}

function presetAllowed(preset: string, environment: Environment): boolean {
  return (environment.spec.presets?.camundaCluster ?? []).includes(preset);
}
