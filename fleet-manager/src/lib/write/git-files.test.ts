import { describe, expect, it } from "vitest";
import { parse } from "yaml";
import { KindMap } from "@/lib/k8s/kinds";
import { REVISION_ANNOTATION } from "@/lib/labels";
import { APP_FIELD_MANAGER } from "@/lib/merge";
import { gitBase, gitOpsManager, gitPath, renderObject } from "./git-files";
import type { KubeObject } from "./types";

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

describe("renderObject", () => {
  const claim = {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: {
      name: "orders",
      namespace: "team-a",
      uid: "u",
      resourceVersion: "7",
      generation: 2,
      creationTimestamp: "2026-10-21T00:00:00Z",
      managedFields: [{}],
    },
    spec: { presetRef: "small" },
    status: { conditions: [] },
  };

  it("leaves out every server field", () => {
    const parsed = parse(renderObject(claim).text);
    expect(parsed.status).toBeUndefined();
    expect(Object.keys(parsed.metadata).sort()).toEqual([
      "annotations",
      "name",
      "namespace",
    ]);
  });

  it("adds the revision annotation with the SHA-256 of the content", () => {
    const { text, revision } = renderObject(claim);
    expect(revision).toMatch(/^[0-9a-f]{64}$/);
    expect(parse(text).metadata.annotations["fleet.camunda.io/revision"]).toBe(
      revision,
    );
  });

  it("keeps the other annotations of the object", () => {
    const parsed = parse(
      renderObject({
        ...claim,
        metadata: { ...claim.metadata, annotations: { note: "x" } },
      }).text,
    );
    expect(parsed.metadata.annotations.note).toBe("x");
  });

  it("gives the same revision for the same content in another key order or with an old revision", () => {
    const reordered = {
      spec: claim.spec,
      metadata: {
        namespace: "team-a",
        name: "orders",
        annotations: { "fleet.camunda.io/revision": "old" },
      },
      kind: claim.kind,
      apiVersion: claim.apiVersion,
    };
    expect(renderObject(reordered).revision).toBe(renderObject(claim).revision);
  });

  it("gives another revision when the content changes", () => {
    expect(
      renderObject({ ...claim, spec: { presetRef: "large" } }).revision,
    ).not.toBe(renderObject(claim).revision);
  });
});

describe("gitOpsManager", () => {
  const revised = (
    ...managedFields: { manager: string; operation: string; time?: string }[]
  ): KubeObject => ({
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: {
      name: "prod",
      annotations: { [REVISION_ANNOTATION]: "abc" },
      managedFields: managedFields.map((entry) => ({
        ...entry,
        fieldsV1: {
          "f:metadata": {
            "f:annotations": { [`f:${REVISION_ANNOTATION}`]: {} },
          },
          "f:spec": { "f:displayName": {} },
        },
      })),
    },
  });

  it("returns the manager that applied the revision annotation with server-side apply", () => {
    expect(
      gitOpsManager(
        revised({ manager: "kustomize-controller", operation: "Apply" }),
      ),
    ).toBe("kustomize-controller");
  });

  it("returns undefined when only an Update entry owns the annotation", () => {
    expect(
      gitOpsManager(
        revised({ manager: "argocd-controller", operation: "Update" }),
      ),
    ).toBeUndefined();
  });

  it("returns undefined when no entry owns the annotation", () => {
    expect(
      gitOpsManager({
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "Environment",
        metadata: {
          name: "prod",
          managedFields: [
            {
              manager: "fleet-manager",
              operation: "Apply",
              fieldsV1: { "f:spec": { "f:displayName": {} } },
            },
          ],
        },
      }),
    ).toBeUndefined();
  });

  it("returns undefined for an object that does not exist", () => {
    expect(gitOpsManager(undefined)).toBeUndefined();
  });

  it("skips the app, which applies the annotation when a page saves a live object directly", () => {
    expect(
      gitOpsManager(
        revised(
          {
            manager: "gitops",
            operation: "Apply",
            time: "2026-10-20T10:00:00Z",
          },
          {
            manager: APP_FIELD_MANAGER,
            operation: "Apply",
            time: "2026-10-21T10:00:00Z",
          },
        ),
      ),
    ).toBe("gitops");
  });
});
