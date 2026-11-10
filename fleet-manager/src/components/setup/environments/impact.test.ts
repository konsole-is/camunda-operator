import type { V1Namespace } from "@kubernetes/client-node";
import { describe, expect, it } from "vitest";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { CamundaManagementClusterClaim } from "@/generated/crds/camundamanagementclusterclaim";
import type { Environment } from "@/generated/crds/environment";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import type { EnvironmentDraft } from "./draft";
import { environmentImpact } from "./impact";
import type { EnvironmentSources } from "./sources";

type Recorded = NonNullable<
  NonNullable<CamundaClusterClaim["status"]>["resolvedPolicy"]
>;

const DELETE: Recorded = { deletionPolicy: "Delete" };

function environment(spec: Environment["spec"]): Environment {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name: "staging", resourceVersion: "3" },
    spec,
  };
}

function namespace(name: string, env?: string): V1Namespace {
  return {
    metadata: {
      name,
      ...(env && { labels: { [ENVIRONMENT_LABEL]: env } }),
    },
  };
}

function clusterClaim(
  name: string,
  spec: Partial<CamundaClusterClaim["spec"]> = {},
  resolvedPolicy?: Recorded,
  ns = "team-a",
): CamundaClusterClaim {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { namespace: ns, name },
    spec: { presetRef: "small", ...spec },
    ...(resolvedPolicy && { status: { resolvedPolicy } }),
  };
}

function planeClaim(
  name: string,
  spec: Partial<CamundaManagementClusterClaim["spec"]> = {},
  resolvedPolicy?: Recorded,
): CamundaManagementClusterClaim {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: { namespace: "team-a", name },
    spec: { environments: ["staging"], ...spec },
    ...(resolvedPolicy && { status: { resolvedPolicy } }),
  } as CamundaManagementClusterClaim;
}

function sources(s: Partial<EnvironmentSources>): EnvironmentSources {
  return {
    environments: [],
    namespaces: [namespace("team-a", "staging"), namespace("team-b", "other")],
    clusterClaims: [],
    planeClaims: [],
    clusters: [],
    planes: [],
    sections: [],
    registrations: [],
    requests: [],
    clusterPresets: [],
    elasticsearchPresets: [],
    databasePresets: [],
    releases: [],
    platformConfigs: [],
    ...s,
  };
}

function edit(spec: Environment["spec"]): EnvironmentDraft {
  return { name: "staging", spec };
}

describe("environmentImpact", () => {
  it("counts the claims whose recorded policy differs from the edit", () => {
    const live = environment({ deletionPolicy: "Delete" });
    expect(
      environmentImpact(
        live,
        edit({ deletionPolicy: "Orphan" }),
        sources({
          clusterClaims: [
            clusterClaim("orders", { releaseRef: "8.9.1" }, DELETE),
            clusterClaim("billing", { releaseRef: "8.9.1" }, DELETE),
            clusterClaim("other", { releaseRef: "8.9.1" }, DELETE, "team-b"),
          ],
        }),
      ),
    ).toEqual({ clusters: 2, planes: 0, warnings: [] });
  });

  it("counts a plane claim as a plane", () => {
    expect(
      environmentImpact(
        environment({}),
        edit({ deletionPolicy: "Orphan" }),
        sources({ planeClaims: [planeClaim("plane", {}, DELETE)] }),
      ),
    ).toMatchObject({ clusters: 0, planes: 1 });
  });

  it("does not count a claim whose recorded policy equals the edit, nor a claim without a recorded policy", () => {
    expect(
      environmentImpact(
        environment({ deletionPolicy: "Delete", displayName: "Staging" }),
        edit({ deletionPolicy: "Delete", displayName: "Stage" }),
        sources({
          clusterClaims: [
            clusterClaim("orders", { releaseRef: "8.9.1" }, DELETE),
            clusterClaim("fresh", { releaseRef: "8.9.1" }),
          ],
        }),
      ),
    ).toEqual({ clusters: 0, planes: 0, warnings: [] });
  });

  it("lists a claim whose release the edit removes, with removed true", () => {
    expect(
      environmentImpact(
        environment({ releases: { allowed: ["8.9.1", "8.9.2"] } }),
        edit({ releases: { allowed: ["8.9.2"] } }),
        sources({
          clusterClaims: [clusterClaim("orders", { releaseRef: "8.9.1" })],
        }),
      ).warnings,
    ).toEqual([
      {
        claim: {
          kind: "CamundaClusterClaim",
          namespace: "team-a",
          name: "orders",
        },
        kind: "CamundaRelease",
        name: "8.9.1",
        removed: true,
      },
    ]);
  });

  it("lists a claim whose CamundaClusterPreset the edit removes", () => {
    expect(
      environmentImpact(
        environment({
          presets: { camundaCluster: ["small", "large"] },
          releases: { allowed: ["8.9.1"] },
        }),
        edit({
          presets: { camundaCluster: ["large"] },
          releases: { allowed: ["8.9.1"] },
        }),
        sources({
          clusterClaims: [
            clusterClaim("orders", { presetRef: "small", releaseRef: "8.9.1" }),
          ],
        }),
      ).warnings,
    ).toEqual([
      {
        claim: {
          kind: "CamundaClusterClaim",
          namespace: "team-a",
          name: "orders",
        },
        kind: "CamundaClusterPreset",
        name: "small",
        removed: true,
      },
    ]);
  });

  it("does not list a claim whose release the live environment already did not allow", () => {
    expect(
      environmentImpact(
        environment({ releases: { allowed: ["8.9.2"] } }),
        edit({ releases: { allowed: ["8.9.3"] } }),
        sources({
          clusterClaims: [clusterClaim("orders", { releaseRef: "8.9.1" })],
        }),
      ).warnings,
    ).toEqual([]);
  });

  it("lists a claim without a releaseRef when the default release changes", () => {
    expect(
      environmentImpact(
        environment({
          releases: { allowed: ["8.9.1", "8.9.2"], default: "8.9.1" },
        }),
        edit({ releases: { allowed: ["8.9.1", "8.9.2"], default: "8.9.2" } }),
        sources({ clusterClaims: [clusterClaim("orders")] }),
      ).warnings,
    ).toEqual([
      {
        claim: {
          kind: "CamundaClusterClaim",
          namespace: "team-a",
          name: "orders",
        },
        kind: "CamundaRelease",
        name: "8.9.2",
        followsDefault: true,
      },
    ]);
  });

  it("lists a claim without a platformConfigRef when the default platform config changes", () => {
    expect(
      environmentImpact(
        environment({ platformConfigRef: "shared" }),
        edit({ platformConfigRef: "strict" }),
        sources({
          clusterClaims: [
            clusterClaim("orders"),
            clusterClaim("pinned", { platformConfigRef: "own" }),
          ],
          planeClaims: [planeClaim("plane")],
        }),
      ).warnings,
    ).toEqual([
      {
        claim: {
          kind: "CamundaClusterClaim",
          namespace: "team-a",
          name: "orders",
        },
        kind: "CamundaPlatformConfig",
        name: "strict",
        followsDefault: true,
      },
      {
        claim: {
          kind: "CamundaManagementClusterClaim",
          namespace: "team-a",
          name: "plane",
        },
        kind: "CamundaPlatformConfig",
        name: "strict",
        followsDefault: true,
      },
    ]);
  });

  it("has no claims and no warnings for a new environment", () => {
    expect(
      environmentImpact(
        undefined,
        edit({ releases: { allowed: ["8.9.2"], default: "8.9.2" } }),
        sources({
          namespaces: [],
          clusterClaims: [clusterClaim("orders", {}, DELETE)],
        }),
      ),
    ).toEqual({ clusters: 0, planes: 0, warnings: [] });
  });
});
