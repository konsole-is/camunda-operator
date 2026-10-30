import { describe, expect, it } from "vitest";
import { KindMap } from "@/lib/k8s/kinds";
import { gitBase, gitPath } from ".";

describe("gitPath", () => {
  it("puts a namespaced object under its namespace and kind plural", () => {
    expect(
      gitPath("clusters/prod", KindMap.CamundaClusterClaim, {
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "CamundaClusterClaim",
        namespace: "team-a",
        name: "orders",
      }),
    ).toBe("clusters/prod/team-a/camundaclusterclaims/orders.yaml");
  });

  it("puts a cluster-scoped object under _cluster", () => {
    expect(
      gitPath("", KindMap.Namespace, {
        apiVersion: "v1",
        kind: "Namespace",
        name: "team-a",
      }),
    ).toBe("_cluster/namespaces/team-a.yaml");
  });
});

describe("gitBase", () => {
  it.each([
    ["", ""],
    ["/", ""],
    ["/fleet/", "fleet"],
    ["a/b", "a/b"],
  ])("%s is the folder %s", (input, out) => expect(gitBase(input)).toBe(out));

  it.each(["../x", "a/../b", "a/./b", "a\\b", "a//b"])("refuses %s", (input) =>
    expect(gitBase(input)).toBeUndefined(),
  );
});
