import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { DESCRIPTION_ANNOTATION } from "@/lib/labels";
import { refKey } from "@/lib/write/refs";
import { presetFixtures } from "./fixtures";
import {
  rotationChange,
  rotationTargets,
  rotationToken,
  rotationUnread,
} from "./rotation";
import type { PresetSources } from "./sources";

const fixtures = presetFixtures();
const medium = fixtures.clusterPresets[0];
const now = new Date("2026-11-15T10:42:59.900Z");

function cluster(
  name: string,
  spec: Partial<Resource<"CamundaCluster">["spec"]>,
): Resource<"CamundaCluster"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaCluster",
    metadata: { namespace: "team-a", name },
    spec: { presetRef: "medium", platformConfigRef: "shared", ...spec },
  } as Resource<"CamundaCluster">;
}

function config(
  name: string,
  spec: Resource<"CamundaPlatformConfig">["spec"],
): Resource<"CamundaPlatformConfig"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaPlatformConfig",
    metadata: { name },
    spec,
  } as Resource<"CamundaPlatformConfig">;
}

const sources: PresetSources = {
  ...fixtures,
  clusters: [
    cluster("basic", {}),
    cluster("explicit-basic", { platformConfigRef: "basic" }),
    cluster("oidc", { platformConfigRef: "oidc" }),
    cluster("own-basic", { auth: { basic: { adminEmail: "a@example.com" } } }),
    cluster("no-config", { platformConfigRef: "missing" }),
    cluster("other-preset", { presetRef: "spare" }),
  ],
  platformConfigs: [
    config("shared", {}),
    config("basic", { auth: { method: "basic" } }),
    config("oidc", {
      auth: { method: "oidc", oidc: { issuerUrl: "https://id.example.com" } },
    } as Resource<"CamundaPlatformConfig">["spec"]),
  ],
};

describe("rotationToken", () => {
  it("writes the UTC date and time to the minute", () => {
    expect(rotationToken(now)).toBe("2026-11-15T10:42Z");
  });
});

describe("rotationTargets", () => {
  const names = rotationTargets(medium, sources).map((u) => u.name);

  it("counts the clusters with basic auth and without their own auth.basic", () => {
    expect(names).toEqual(["basic", "explicit-basic"]);
  });

  it("counts a cluster whose platform config sets no method as basic", () => {
    expect(names).toContain("basic");
  });

  it("does not count an OIDC cluster or a cluster with its own auth.basic", () => {
    expect(names).not.toContain("oidc");
    expect(names).not.toContain("own-basic");
  });

  it("does not count a cluster whose platform config is missing", () => {
    expect(names).not.toContain("no-config");
  });
});

describe("rotationUnread", () => {
  it("names the clusters and the platform configs that are not read or not current", () => {
    expect(
      rotationUnread(
        [{ kind: "CamundaPlatformConfig", message: "forbidden" }],
        ["CamundaCluster", "Environment"],
      ),
    ).toEqual(["CamundaCluster", "CamundaPlatformConfig"]);
    expect(rotationUnread([], [])).toEqual([]);
  });
});

describe("rotationChange", () => {
  it("sets passwordRotation and keeps every other field of the preset", () => {
    expect(rotationChange(medium, now).apply[0].spec).toEqual({
      cluster: {
        ...medium.spec.cluster,
        auth: {
          basic: {
            adminEmail: "admin@example.com",
            passwordRotation: "2026-11-15T10:42Z",
          },
        },
      },
    });
  });

  it("keeps the adminEmail of the preset", () => {
    const spec = rotationChange(medium, now).apply[0].spec as {
      cluster: { auth: { basic: { adminEmail: string } } };
    };
    expect(spec.cluster.auth.basic.adminEmail).toBe("admin@example.com");
  });

  it("writes from the live preset with its precondition, force, and description", () => {
    const change = rotationChange(medium, now);
    expect(change).toMatchObject({
      intent: "Rotate the admin password of the clusters of the preset medium",
      preconditions: {
        [refKey({ kind: "CamundaClusterPreset", name: "medium" })]: "10",
      },
      force: true,
    });
    expect(change.apply[0].metadata.annotations).toEqual({
      [DESCRIPTION_ANNOTATION]:
        "For production workloads with three brokers and replication.",
    });
  });
});
