import { describe, expect, it } from "vitest";
import { LAST_APPLIED_ANNOTATION } from "@/lib/labels";
import type { ManagedFieldsEntry } from "@/lib/merge";
import {
  foreignFields,
  offered,
  takenFields,
  withTakeOver,
  withoutFields,
} from "./ownership";
import type { Change, KubeObject, ObjectOwnership } from "./types";

const uid = "0b5c7a2e-1f0e-4f43-9d0a-6a0c2f1e9b11";
const ref = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  name: "staging",
};

const environment = (
  spec: object,
  managedFields: ManagedFieldsEntry[] = [],
  metadata: object = {},
): KubeObject => ({
  apiVersion: ref.apiVersion,
  kind: ref.kind,
  metadata: { name: ref.name, managedFields, ...metadata },
  spec,
});

const entry = (
  manager: string,
  operation: string,
  fieldsV1: object,
): ManagedFieldsEntry => ({ manager, operation, fieldsV1 });

const displayName = { "f:spec": { "f:displayName": {} } };

describe("foreignFields", () => {
  it("names a field that kubectl set and the apply changes", () => {
    const live = environment({ displayName: "Stage" }, [
      entry("kubectl-client-side-apply", "Update", displayName),
    ]);
    expect(foreignFields(live, environment({ displayName: "Prod" }))).toEqual({
      changed: [
        {
          path: "spec.displayName",
          steps: [{ field: "spec" }, { field: "displayName" }],
          managers: [
            {
              name: "kubectl-client-side-apply",
              operation: "Update",
              kind: "other",
            },
          ],
        },
      ],
      cleared: [],
    });
  });

  it("a field that the apply sends with the same value is not changed", () => {
    const live = environment({ displayName: "Stage" }, [
      entry("kubectl-client-side-apply", "Update", displayName),
    ]);
    expect(foreignFields(live, environment({ displayName: "Stage" }))).toEqual({
      changed: [],
      cleared: [],
    });
  });

  it("a field that the fleet manager shares with kubectl is changed", () => {
    const live = environment({ displayName: "Stage" }, [
      entry("fleet-manager", "Apply", displayName),
      entry("kubectl", "Apply", displayName),
    ]);
    const { changed } = foreignFields(
      live,
      environment({ displayName: "Prod" }),
    );
    expect(changed).toEqual([
      expect.objectContaining({
        path: "spec.displayName",
        managers: [{ name: "kubectl", operation: "Apply", kind: "other" }],
      }),
    ]);
  });

  it("names a cleared field, a map entry, and a set member once each", () => {
    const live = environment(
      {
        displayName: "Stage",
        sections: { gone: { required: true }, kept: { required: false } },
        presets: { camundaCluster: ["a", "b"] },
      },
      [
        entry("kubectl", "Apply", {
          "f:spec": {
            "f:displayName": {},
            "f:sections": { "f:gone": { ".": {}, "f:required": {} } },
            "f:presets": { "f:camundaCluster": { 'v:"b"': {} } },
          },
        }),
        entry("fleet-manager", "Apply", {
          "f:spec": {
            "f:sections": { "f:kept": { ".": {}, "f:required": {} } },
            "f:presets": { "f:camundaCluster": { 'v:"a"': {} } },
          },
        }),
      ],
    );
    const applied = environment({
      sections: { kept: { required: false } },
      presets: { camundaCluster: ["a"] },
    });
    const kubectl = [{ name: "kubectl", operation: "Apply", kind: "other" }];
    expect(foreignFields(live, applied)).toEqual({
      changed: [],
      cleared: [
        {
          path: "spec.displayName",
          steps: [{ field: "spec" }, { field: "displayName" }],
          managers: kubectl,
        },
        {
          path: 'spec.presets.camundaCluster[="b"]',
          steps: [
            { field: "spec" },
            { field: "presets" },
            { field: "camundaCluster" },
            { value: "b" },
          ],
          managers: kubectl,
        },
        {
          path: "spec.sections.gone",
          steps: [{ field: "spec" }, { field: "sections" }, { field: "gone" }],
          managers: kubectl,
        },
      ],
    });
  });

  it("names a leaf under a kept keyed item", () => {
    const allowed = (item: object) => ({
      parameters: { "aws-s3": { tier: { allowed: [item] } } },
    });
    const live = environment(allowed({ name: "x", label: "X" }), [
      entry("kubectl", "Apply", {
        "f:spec": {
          "f:parameters": {
            "f:aws-s3": {
              "f:tier": {
                "f:allowed": {
                  'k:{"name":"x"}': { ".": {}, "f:name": {}, "f:label": {} },
                },
              },
            },
          },
        },
      }),
    ]);
    const { cleared } = foreignFields(
      live,
      environment(allowed({ name: "x" })),
    );
    expect(cleared).toEqual([
      {
        path: "spec.parameters.aws-s3.tier.allowed[name=x].label",
        steps: [
          { field: "spec" },
          { field: "parameters" },
          { field: "aws-s3" },
          { field: "tier" },
          { field: "allowed" },
          { item: { name: "x" } },
          { field: "label" },
        ],
        managers: [{ name: "kubectl", operation: "Apply", kind: "other" }],
      },
    ]);
  });

  it("a cleared field that only the fleet manager owns is not named", () => {
    const live = environment({ displayName: "Stage" }, [
      entry("fleet-manager", "Apply", displayName),
    ]);
    expect(foreignFields(live, environment({}))).toEqual({
      changed: [],
      cleared: [],
    });
  });

  it("an Update entry of the fleet manager is another manager", () => {
    const live = environment({ displayName: "Stage" }, [
      entry("fleet-manager", "Update", displayName),
    ]);
    expect(foreignFields(live, environment({})).cleared).toEqual([
      expect.objectContaining({
        path: "spec.displayName",
        managers: [
          { name: "fleet-manager", operation: "Update", kind: "other" },
        ],
      }),
    ]);
  });

  it("skips the last applied annotation and status", () => {
    const live = {
      ...environment(
        {},
        [
          entry("kubectl-client-side-apply", "Update", {
            "f:metadata": {
              "f:annotations": {
                ".": {},
                [`f:${LAST_APPLIED_ANNOTATION}`]: {},
              },
            },
            "f:status": { "f:phase": {} },
          }),
          {
            ...entry("camunda-operator/environment", "Update", {
              "f:status": { "f:ready": {} },
            }),
            subresource: "status",
          },
        ],
        { annotations: { [LAST_APPLIED_ANNOTATION]: "{}" } },
      ),
      status: { phase: "Ready", ready: true },
    };
    expect(foreignFields(live, environment({}))).toEqual({
      changed: [],
      cleared: [],
    });
  });

  it("names a label and an annotation that kubectl set", () => {
    const live = environment(
      {},
      [
        entry("kubectl", "Apply", {
          "f:metadata": {
            "f:labels": { "f:team": {} },
            "f:annotations": { "f:note": {} },
          },
        }),
      ],
      { labels: { team: "a" }, annotations: { note: "n" } },
    );
    const applied = environment({}, [], { labels: { team: "b" } });
    const { changed, cleared } = foreignFields(live, applied);
    expect(changed.map((f) => f.path)).toEqual(["metadata.labels.team"]);
    expect(cleared.map((f) => f.path)).toEqual(["metadata.annotations.note"]);
  });

  it("marks an operator owner", () => {
    const live = environment({ displayName: "Stage" }, [
      entry(`CamundaClusterClaim/cluster/${uid}`, "Apply", displayName),
    ]);
    const { changed } = foreignFields(
      live,
      environment({ displayName: "Prod" }),
    );
    expect(changed[0].managers[0].kind).toBe("operator");
    expect(offered(changed[0])).toBe(false);
  });

  it("marks every manager of an object that Argo CD tracks as a GitOps tool", () => {
    const live = environment(
      { displayName: "Stage" },
      [entry("kubectl-client-side-apply", "Update", displayName)],
      { annotations: { "argocd.argoproj.io/tracking-id": "a:b/c" } },
    );
    const { changed } = foreignFields(
      live,
      environment({ displayName: "Prod" }, [], {
        annotations: { "argocd.argoproj.io/tracking-id": "a:b/c" },
      }),
    );
    expect(changed[0].managers[0].kind).toBe("gitops");
    expect(offered(changed[0])).toBe(true);
  });
});

describe("withTakeOver", () => {
  const field = (path: string, name: string) => ({
    path,
    steps: [],
    managers: [{ name, operation: "Apply", kind: "other" as const }],
  });
  const operatorField = (path: string) => ({
    path,
    steps: [],
    managers: [
      {
        name: `CamundaClusterClaim/cluster/${uid}`,
        operation: "Apply",
        kind: "operator" as const,
      },
    ],
  });
  const change: Change = {
    intent: "Rename",
    apply: [environment({ displayName: "Prod" })],
    preconditions: { "Environment//staging": "7" },
  };

  it("lists offered fields only", () => {
    const ownership: ObjectOwnership[] = [
      {
        ref,
        changed: [field("spec.a", "kubectl"), operatorField("spec.b")],
        cleared: [field("spec.c", "kubectl")],
      },
      {
        ref: { ...ref, name: "other" },
        changed: [field("spec.a", "kubectl")],
        cleared: [],
      },
    ];
    expect(withTakeOver(change, ownership)).toEqual({
      ...change,
      takeOver: { "Environment//staging": ["spec.a"] },
      remove: { "Environment//staging": ["spec.c"] },
    });
  });

  it("returns the same change when nothing is offered", () => {
    expect(
      withTakeOver(change, [
        { ref, changed: [operatorField("spec.b")], cleared: [] },
      ]),
    ).toBe(change);
    expect(withTakeOver(change, [])).toBe(change);
  });
});

describe("takenFields", () => {
  it("names a field that lost its foreign owner and still exists", () => {
    const before = environment({ displayName: "Stage", color: "red" }, [
      entry("kubectl-client-side-apply", "Update", {
        "f:spec": { "f:displayName": {}, "f:color": {}, "f:gone": {} },
      }),
    ]);
    const after = environment({ displayName: "Prod", color: "red" }, [
      entry("kubectl-client-side-apply", "Update", {
        "f:spec": { "f:color": {} },
      }),
      entry("fleet-manager", "Apply", displayName),
    ]);
    expect(takenFields(before, after)).toEqual([
      {
        path: "spec.displayName",
        steps: [{ field: "spec" }, { field: "displayName" }],
        managers: [
          {
            name: "kubectl-client-side-apply",
            operation: "Update",
            kind: "other",
          },
        ],
      },
    ]);
  });

  it("a field that another entry of the same manager owns is not taken", () => {
    const before = environment({ displayName: "Stage" }, [
      entry("kubectl", "Apply", displayName),
    ]);
    const after = environment({ displayName: "Stage" }, [
      entry("kubectl", "Update", displayName),
    ]);
    expect(takenFields(before, after).map((f) => f.path)).toEqual([
      "spec.displayName",
    ]);
    expect(takenFields(before, before)).toEqual([]);
  });
});

describe("withoutFields", () => {
  it("removes a set member and a keyed leaf and leaves the input unchanged", () => {
    const object = {
      spec: {
        presets: { camundaCluster: ["a", "b"] },
        allowed: [
          { name: "x", label: "X", value: 1 },
          { name: "y", label: "Y" },
        ],
      },
    };
    const copy = structuredClone(object);
    const result = withoutFields(object, [
      {
        path: 'spec.presets.camundaCluster[="b"]',
        steps: [
          { field: "spec" },
          { field: "presets" },
          { field: "camundaCluster" },
          { value: "b" },
        ],
        managers: [],
      },
      {
        path: "spec.allowed[name=x].label",
        steps: [
          { field: "spec" },
          { field: "allowed" },
          { item: { name: "x" } },
          { field: "label" },
        ],
        managers: [],
      },
    ]);
    expect(result).toEqual({
      spec: {
        presets: { camundaCluster: ["a"] },
        allowed: [
          { name: "x", value: 1 },
          { name: "y", label: "Y" },
        ],
      },
    });
    expect(object).toEqual(copy);
  });

  it("keeps __proto__ as a key", () => {
    const object = JSON.parse('{"spec":{"__proto__":{"a":1},"gone":true}}') as {
      spec: object;
    };
    const result = withoutFields(object, [
      {
        path: "spec.gone",
        steps: [{ field: "spec" }, { field: "gone" }],
        managers: [],
      },
    ]);
    expect(Object.keys(result.spec)).toEqual(["__proto__"]);
    expect(Object.getPrototypeOf(result.spec)).toBe(Object.prototype);
  });

  it("removes a whole list item and leaves a missing field alone", () => {
    const object = { spec: { allowed: [{ name: "x" }, { name: "y" }] } };
    expect(
      withoutFields(object, [
        {
          path: "spec.allowed[name=x]",
          steps: [
            { field: "spec" },
            { field: "allowed" },
            { item: { name: "x" } },
          ],
          managers: [],
        },
        {
          path: "spec.missing.leaf",
          steps: [{ field: "spec" }, { field: "missing" }, { field: "leaf" }],
          managers: [],
        },
      ]),
    ).toEqual({ spec: { allowed: [{ name: "y" }] } });
  });
});
