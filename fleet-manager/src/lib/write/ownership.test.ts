import { describe, expect, it } from "vitest";
import { LAST_APPLIED_ANNOTATION } from "@/lib/labels";
import type { ManagedFieldsEntry } from "@/lib/merge";
import {
  fieldValue,
  foreignFields,
  offered,
  takenFields,
  withTakeOver,
  withValuesOf,
  withoutFields,
} from "./ownership";
import type {
  Change,
  ForeignField,
  KubeObject,
  ObjectOwnership,
} from "./types";

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
              known: true,
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
        managers: [
          { name: "kubectl", operation: "Apply", kind: "other", known: true },
        ],
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
    const kubectl = [
      { name: "kubectl", operation: "Apply", kind: "other", known: true },
    ];
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
        managers: [
          { name: "kubectl", operation: "Apply", kind: "other", known: true },
        ],
      },
    ]);
  });

  it("leaves out a cleared field at or under a kept path", () => {
    const live = environment(
      {
        recovery: { time: "2026-11-01T00:00:00Z" },
        pitr: { enabled: true, lastRecovery: { at: "now" } },
      },
      [
        entry("kubectl", "Apply", {
          "f:spec": {
            "f:recovery": { "f:time": {} },
            "f:pitr": { "f:enabled": {}, "f:lastRecovery": { "f:at": {} } },
          },
        }),
      ],
    );
    expect(
      foreignFields(live, environment({ pitr: { enabled: true } }), [
        "spec.recovery",
        "spec.pitr.lastRecovery",
      ]),
    ).toEqual({ changed: [], cleared: [] });
  });

  it("names the other fields of a cleared field that holds a kept path", () => {
    const live = environment({ pitr: { enabled: true, lastRecovery: {} } }, [
      entry("kubectl", "Apply", {
        "f:spec": {
          "f:pitr": { ".": {}, "f:enabled": {}, "f:lastRecovery": {} },
        },
      }),
    ]);
    const { cleared } = foreignFields(live, environment({}), [
      "spec.pitr.lastRecovery",
    ]);
    expect(cleared.map((f) => f.path)).toEqual(["spec.pitr.enabled"]);
  });

  it("leaves out a changed field at a kept path and one that holds a kept path", () => {
    const live = environment(
      { displayName: "Stage", pitr: { enabled: true, lastRecovery: {} } },
      [
        entry("kubectl", "Apply", {
          "f:spec": { "f:displayName": {}, "f:pitr": {} },
        }),
      ],
    );
    const { changed } = foreignFields(
      live,
      environment({ displayName: "Prod", pitr: { enabled: false } }),
      ["spec.displayName", "spec.pitr.lastRecovery"],
    );
    expect(changed).toEqual([]);
  });

  it("leaves out every field of a kept root", () => {
    const live = environment({ displayName: "Stage" }, [
      entry("kubectl", "Apply", displayName),
    ]);
    expect(foreignFields(live, environment({}), ["spec"])).toEqual({
      changed: [],
      cleared: [],
    });
  });

  it("a kept annotation does not hide a sibling whose key starts with its key", () => {
    const live = environment(
      {},
      [
        entry("kubectl", "Apply", {
          "f:metadata": {
            "f:annotations": { "f:example": {}, "f:example.com/team": {} },
          },
        }),
      ],
      { annotations: { example: "a", "example.com/team": "b" } },
    );
    const { cleared } = foreignFields(live, environment({}), [
      "metadata.annotations.example",
    ]);
    expect(cleared.map((f) => f.path)).toEqual([
      "metadata.annotations.example.com/team",
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
          {
            name: "fleet-manager",
            operation: "Update",
            kind: "other",
            known: true,
          },
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

  it("marks a tool that the fleet manager does not know", () => {
    const live = environment({ displayName: "Stage", color: "red" }, [
      entry("my-script", "Apply", displayName),
      entry("kubectl-edit", "Update", { "f:spec": { "f:color": {} } }),
    ]);
    const { changed } = foreignFields(
      live,
      environment({ displayName: "Prod", color: "blue" }),
    );
    expect(changed.flatMap((f) => f.managers)).toEqual([
      { name: "kubectl-edit", operation: "Update", kind: "other", known: true },
      { name: "my-script", operation: "Apply", kind: "other", known: false },
    ]);
    expect(changed.every(offered)).toBe(true);
  });

  it("leaves out the fields of a subresource entry", () => {
    const live = environment({ displayName: "Stage" }, [
      { ...entry("kubectl", "Update", displayName), subresource: "scale" },
    ]);
    expect(foreignFields(live, environment({ displayName: "Prod" }))).toEqual({
      changed: [],
      cleared: [],
    });
  });
});

describe("withTakeOver", () => {
  const field = (path: string, name: string) => ({
    path,
    steps: [],
    managers: [
      { name, operation: "Apply", kind: "other" as const, known: true },
    ],
  });
  const operatorField = (path: string) => ({
    path,
    steps: [],
    managers: [
      {
        name: `CamundaClusterClaim/cluster/${uid}`,
        operation: "Apply",
        kind: "operator" as const,
        known: true,
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

  it("lists a path once when two fields have it", () => {
    const live = environment({ parameters: { "a.b": 1, a: { b: 2 } } }, [
      entry("kubectl", "Apply", {
        "f:spec": { "f:parameters": { "f:a.b": {}, "f:a": { "f:b": {} } } },
      }),
    ]);
    const fields = foreignFields(
      live,
      environment({ parameters: { "a.b": 3, a: { b: 4 } } }),
    );
    expect(fields.changed.map((f) => f.path)).toEqual([
      "spec.parameters.a.b",
      "spec.parameters.a.b",
    ]);
    expect(withTakeOver(change, [{ ref, ...fields }]).takeOver).toEqual({
      "Environment//staging": ["spec.parameters.a.b"],
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
            known: true,
          },
        ],
      },
    ]);
  });

  it("an entry of the same manager with another operation does not keep the field", () => {
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

describe("takenFields of a subresource entry", () => {
  it("names a field that a subresource entry lost", () => {
    const replicas = { "f:spec": { "f:replicas": {} } };
    const before = environment({ replicas: 3 }, [
      { ...entry("kubectl", "Update", replicas), subresource: "scale" },
    ]);
    const after = environment({ replicas: 5 }, [
      entry("fleet-manager", "Apply", replicas),
    ]);
    expect(takenFields(before, after)).toEqual([
      expect.objectContaining({
        path: "spec.replicas",
        managers: [expect.objectContaining({ name: "kubectl" })],
      }),
    ]);
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

  it("removes two items by index from the list as it was", () => {
    const at = (index: number) => ({
      path: `spec.items[${index}]`,
      steps: [{ field: "spec" }, { field: "items" }, { index }],
      managers: [],
    });
    expect(
      withoutFields({ spec: { items: ["a", "b", "c", "d", "e"] } }, [
        at(1),
        at(3),
      ]),
    ).toEqual({ spec: { items: ["a", "c", "e"] } });
  });
});

// fieldAt gives a field with steps, as the review names it.
const fieldAt = (...steps: ForeignField["steps"]): ForeignField => ({
  path: "",
  steps,
  managers: [],
});

describe("fieldValue", () => {
  const object = JSON.parse(
    `{"spec":{"replicas":3,"images":{"zeebe":"8.9"},"zones":["a","b"],
      "allowed":[{"name":"x","label":"X"}],"labels":{"app.kubernetes.io/name":"c"},
      "__proto__":{"own":true}}}`,
  ) as object;

  it("gives a scalar, an object, and a list", () => {
    expect(
      fieldValue(object, fieldAt({ field: "spec" }, { field: "replicas" })),
    ).toBe(3);
    expect(
      fieldValue(object, fieldAt({ field: "spec" }, { field: "images" })),
    ).toEqual({ zeebe: "8.9" });
    expect(
      fieldValue(object, fieldAt({ field: "spec" }, { field: "zones" })),
    ).toEqual(["a", "b"]);
  });

  it("gives a list item and a field under it", () => {
    const allowed = [{ field: "spec" }, { field: "allowed" }] as const;
    expect(
      fieldValue(object, fieldAt(...allowed, { item: { name: "x" } })),
    ).toEqual({ name: "x", label: "X" });
    expect(
      fieldValue(
        object,
        fieldAt(...allowed, { item: { name: "x" } }, { field: "label" }),
      ),
    ).toBe("X");
    expect(
      fieldValue(
        object,
        fieldAt({ field: "spec" }, { field: "zones" }, { value: "b" }),
      ),
    ).toBe("b");
    expect(
      fieldValue(
        object,
        fieldAt({ field: "spec" }, { field: "zones" }, { index: 0 }),
      ),
    ).toBe("a");
  });

  it("gives undefined for a missing field, a missing parent, and a missing item", () => {
    expect(
      fieldValue(object, fieldAt({ field: "spec" }, { field: "gone" })),
    ).toBeUndefined();
    expect(
      fieldValue(
        object,
        fieldAt({ field: "spec" }, { field: "gone" }, { field: "leaf" }),
      ),
    ).toBeUndefined();
    expect(
      fieldValue(
        object,
        fieldAt({ field: "spec" }, { field: "zones" }, { index: 2 }),
      ),
    ).toBeUndefined();
    expect(
      fieldValue(
        object,
        fieldAt({ field: "spec" }, { field: "replicas" }, { field: "leaf" }),
      ),
    ).toBeUndefined();
  });

  it("reads __proto__ as a key, and never the prototype", () => {
    expect(
      fieldValue(object, fieldAt({ field: "spec" }, { field: "__proto__" })),
    ).toEqual({ own: true });
    expect(
      fieldValue(
        { spec: {} },
        fieldAt({ field: "spec" }, { field: "__proto__" }),
      ),
    ).toBeUndefined();
    expect(
      fieldValue(
        { spec: {} },
        fieldAt({ field: "spec" }, { field: "toString" }),
      ),
    ).toBeUndefined();
  });

  it("reads a map key with dots as one key", () => {
    expect(
      fieldValue(
        object,
        fieldAt(
          { field: "spec" },
          { field: "labels" },
          { field: "app.kubernetes.io/name" },
        ),
      ),
    ).toBe("c");
  });
});

describe("withValuesOf", () => {
  const from = JSON.parse(
    `{"spec":{"replicas":3,"images":{"zeebe":"8.9"},"zones":["a","b"],
      "allowed":[{"name":"x","label":"X"}],"labels":{"app.kubernetes.io/name":"c"},
      "__proto__":{"own":true}}}`,
  ) as object;

  it("sets a scalar, an object, and a list from from, and changes neither input", () => {
    const object = { spec: { replicas: 1 } };
    const fromCopy = structuredClone(from);
    const result = withValuesOf(
      object,
      [
        fieldAt({ field: "spec" }, { field: "replicas" }),
        fieldAt({ field: "spec" }, { field: "images" }),
        fieldAt({ field: "spec" }, { field: "zones" }),
      ],
      from,
    );
    expect(result).toEqual({
      spec: { replicas: 3, images: { zeebe: "8.9" }, zones: ["a", "b"] },
    });
    expect(object).toEqual({ spec: { replicas: 1 } });
    (result.spec as unknown as { zones: string[] }).zones.push("c");
    expect(from).toEqual(fromCopy);
  });

  it("does not set a field whose last step is a list item", () => {
    const object = { spec: { allowed: [] as unknown[], zones: ["a"] } };
    expect(
      withValuesOf(
        object,
        [
          fieldAt(
            { field: "spec" },
            { field: "allowed" },
            { item: { name: "x" } },
          ),
          fieldAt({ field: "spec" }, { field: "zones" }, { value: "b" }),
          fieldAt({ field: "spec" }, { field: "zones" }, { index: 1 }),
        ],
        from,
      ),
    ).toEqual(object);
  });

  it("sets a field under a list item that object holds", () => {
    const object = { spec: { allowed: [{ name: "x" }] } };
    expect(
      withValuesOf(
        object,
        [
          fieldAt(
            { field: "spec" },
            { field: "allowed" },
            { item: { name: "x" } },
            { field: "label" },
          ),
        ],
        from,
      ),
    ).toEqual({ spec: { allowed: [{ name: "x", label: "X" }] } });
  });

  it("does not set a field whose parent object does not hold, or that from does not hold", () => {
    const object = { spec: {} };
    expect(
      withValuesOf(
        object,
        [
          fieldAt({ field: "spec" }, { field: "images" }, { field: "zeebe" }),
          fieldAt({ field: "spec" }, { field: "gone" }),
        ],
        from,
      ),
    ).toEqual({ spec: {} });
  });

  it("sets __proto__ as a key and leaves the prototype alone", () => {
    const result = withValuesOf(
      { spec: {} },
      [fieldAt({ field: "spec" }, { field: "__proto__" })],
      from,
    ) as { spec: object };
    expect(Object.keys(result.spec)).toEqual(["__proto__"]);
    expect(Object.getPrototypeOf(result.spec)).toBe(Object.prototype);
    expect(
      Object.getOwnPropertyDescriptor(result.spec, "__proto__")?.value,
    ).toEqual({ own: true });
  });

  it("sets a map key with dots as one key", () => {
    expect(
      withValuesOf(
        { spec: { labels: {} } },
        [
          fieldAt(
            { field: "spec" },
            { field: "labels" },
            { field: "app.kubernetes.io/name" },
          ),
        ],
        from,
      ),
    ).toEqual({ spec: { labels: { "app.kubernetes.io/name": "c" } } });
  });
});
