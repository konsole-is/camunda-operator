import { describe, expect, it } from "vitest";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaManagementCluster } from "@/generated/crds/camundamanagementcluster";
import type { CamundaRelease } from "@/generated/crds/camundarelease";
import { platformFixtures } from "./fixtures";
import { platformRows, userMinors, usersOf } from "./model";
import type { PlatformSources } from "./sources";

function cluster(name: string, spec: object, status?: object) {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaCluster",
    metadata: { namespace: "team-x", name },
    spec: { platformConfigRef: "lab", ...spec },
    ...(status && { status }),
  } as unknown as CamundaCluster;
}

function plane(name: string, spec: object) {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaManagementCluster",
    metadata: { namespace: "platform", name },
    spec: { platformConfigRef: "lab", ...spec },
  } as unknown as CamundaManagementCluster;
}

function minorsOf(sources: PlatformSources, name = "lab") {
  return userMinors(usersOf(name, sources), sources);
}

describe("platformRows", () => {
  it("gives the name, the auth method, the user count, and Ready of each config, sorted by name", () => {
    const rows = platformRows(platformFixtures());

    expect(
      rows.map((r) => [r.name, r.method, r.users, r.ready?.reason]),
    ).toEqual([
      ["basic-lab", "basic", 0, "Healthy"],
      ["broken", "basic", 1, "MissingSecret"],
      ["oidc-prod", "oidc", 5, "Healthy"],
    ]);
  });
});

describe("userMinors", () => {
  it("takes the reported version of a cluster", () => {
    expect(minorsOf(platformFixtures(), "oidc-prod")).toEqual(["8.9"]);
  });

  it("takes the merged version of a cluster that reports none", () => {
    const release = {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaRelease",
      metadata: { name: "next" },
      spec: { version: "8.10.0" },
    } as unknown as CamundaRelease;
    const sources: PlatformSources = {
      ...platformFixtures(),
      clusters: [cluster("fresh", { releaseRef: "next" })],
      releases: [release],
    };

    expect(minorsOf(sources)).toEqual(["8.10"]);
  });

  it("takes the Identity, Console, and Web Modeler minors of a management plane", () => {
    const sources: PlatformSources = {
      ...platformFixtures(),
      planes: [
        plane("main", {
          identity: { version: "8.9.14" },
          console: { version: "8.10.133" },
          webModeler: { version: "8.11.15" },
        }),
      ],
    };

    expect(minorsOf(sources)).toEqual(["8.9", "8.10", "8.11"]);
  });

  it("dedupes the minors and sorts them by number", () => {
    const sources: PlatformSources = {
      ...platformFixtures(),
      clusters: [
        cluster("a", {}, { management: { version: "8.10.1" } }),
        cluster("b", {}, { management: { version: "8.9.3" } }),
        cluster("c", {}, { management: { version: "8.10.4" } }),
      ],
    };

    expect(minorsOf(sources)).toEqual(["8.9", "8.10"]);
  });

  it("userMinors leaves database servers out", () => {
    const sources = platformFixtures();
    const users = usersOf("oidc-prod", sources);

    expect(users.databaseServers).toHaveLength(1);
    expect(userMinors({ ...users, clusters: [], planes: [] }, sources)).toEqual(
      [],
    );
  });
});
