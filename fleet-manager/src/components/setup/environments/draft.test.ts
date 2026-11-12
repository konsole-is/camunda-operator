import type { V1Namespace } from "@kubernetes/client-node";
import { describe, expect, it, vi } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import {
  createChange,
  deleteChange,
  draftErrors,
  draftOf,
  type EnvironmentDraft,
  namespaceLabelChange,
  specOf,
  updateChange,
} from "./draft";

vi.mock("@/lib/write/writer", () => ({ getWriter: vi.fn() }));
const { namespaceLabelChangeSchema } = await import("@/lib/write/run-change");

const NEW = { isNew: true, taken: [] };

function draft(
  spec: Environment["spec"] = {},
  name = "staging",
): EnvironmentDraft {
  return { name, spec: { deletionPolicy: "Delete", ...spec } };
}

const live: Environment = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: {
    name: "staging",
    resourceVersion: "12",
    uid: "u-1",
    labels: { team: "platform" },
    managedFields: [{ manager: "kubectl-client-side-apply" }],
  },
  spec: { displayName: "Staging", releases: { allowed: ["8.9.1"] } },
  status: { namespaces: 2 },
};

describe("draftOf", () => {
  it("a new draft has the default deletion policy Delete", () => {
    expect(draftOf(undefined)).toEqual({
      name: "",
      spec: { deletionPolicy: "Delete" },
    });
  });

  it("a draft of an environment holds its name and a copy of its spec", () => {
    const d = draftOf(live);
    expect(d).toEqual({ name: "staging", spec: live.spec });
    d.spec.releases!.allowed!.push("8.9.2");
    expect(live.spec.releases?.allowed).toEqual(["8.9.1"]);
  });
});

describe("specOf", () => {
  it("drops empty lists, maps, and strings", () => {
    expect(
      specOf(
        draft({
          displayName: "",
          color: "",
          presets: { camundaCluster: [] },
          releases: { allowed: [], default: "" },
          platformConfigRef: "",
          namespaceCreation: {
            allowed: false,
            labels: {},
            requiredLabelKeys: [],
          },
        }),
      ),
    ).toEqual({ deletionPolicy: "Delete" });
  });

  it("keeps the lists that hold names, and the sections and parameters as they are", () => {
    const sections = { access: { required: true, allowed: ["gateway"] } };
    const parameters = { gateway: { host: { claimEditable: true } } };
    expect(
      specOf(
        draft({
          presets: { camundaCluster: ["small"], databaseServer: [] },
          releases: { allowed: ["8.9.1"], default: "" },
          namespaceCreation: { allowed: false, labels: { team: "a" } },
          sections,
          parameters,
        }),
      ),
    ).toEqual({
      deletionPolicy: "Delete",
      presets: { camundaCluster: ["small"] },
      releases: { allowed: ["8.9.1"] },
      namespaceCreation: { labels: { team: "a" } },
      sections,
      parameters,
    });
  });
});

describe("draftErrors", () => {
  it.each([
    ["", "Give the environment a name."],
    ["Bad_Name", "The name must be a DNS name of at most 63 characters."],
    ["a".repeat(64), "The name must be a DNS name of at most 63 characters."],
  ])("refuses the name %j of a new environment", (name, message) => {
    expect(draftErrors(draft({}, name), NEW)).toEqual([
      { path: "name", message },
    ]);
  });

  it("accepts the name new, because the create page has a path of its own", () => {
    expect(draftErrors(draft({}, "new"), NEW)).toEqual([]);
  });

  it("refuses the name of an environment that exists", () => {
    expect(
      draftErrors(draft({}, "staging"), { isNew: true, taken: ["staging"] }),
    ).toEqual([
      { path: "name", message: "An environment with this name exists." },
    ]);
  });

  it("checks no name of an existing environment", () => {
    expect(
      draftErrors(draft({}, "staging"), { isNew: false, taken: ["staging"] }),
    ).toEqual([]);
  });

  it("refuses a display name of more than 64 characters", () => {
    expect(draftErrors(draft({ displayName: "x".repeat(65) }), NEW)).toEqual([
      {
        path: "spec.displayName",
        message: "The display name has at most 64 characters.",
      },
    ]);
    expect(draftErrors(draft({ displayName: "x".repeat(64) }), NEW)).toEqual(
      [],
    );
  });

  it("counts the characters of a display name as the API server does", () => {
    expect(draftErrors(draft({ displayName: "🚀".repeat(64) }), NEW)).toEqual(
      [],
    );
    expect(
      draftErrors(draft({ displayName: "🚀".repeat(65) }), NEW),
    ).toHaveLength(1);
  });

  it("refuses a color that is not a hex color of six digits", () => {
    expect(draftErrors(draft({ color: "#abc" }), NEW)).toEqual([
      {
        path: "spec.color",
        message: "The color is a hex color such as #2E7D32.",
      },
    ]);
  });

  it("refuses a default release that is not allowed", () => {
    expect(
      draftErrors(
        draft({ releases: { allowed: ["8.9.1"], default: "8.9.2" } }),
        NEW,
      ),
    ).toEqual([
      {
        path: "spec.releases.default",
        message: "The default release must be one of the allowed releases.",
      },
    ]);
  });

  it("refuses a section default that is not an allowed option", () => {
    expect(
      draftErrors(
        draft({
          sections: {
            access: { allowed: ["gateway"], default: "gateway" },
            "backup-storage": { allowed: ["s3"], default: "rustfs" },
          },
        }),
        NEW,
      ),
    ).toEqual([
      {
        path: "spec.sections.backup-storage.default",
        message: "The default option must be one of the allowed options.",
      },
    ]);
  });

  it("refuses a required section without an allowed option", () => {
    expect(
      draftErrors(
        draft({ sections: { "backup-storage": { required: true } } }),
        NEW,
      ),
    ).toEqual([
      {
        path: "spec.sections.backup-storage.allowed",
        message: "A required section needs at least one allowed option.",
      },
    ]);
  });

  it("refuses a duplicate name in the allowed values of a parameter", () => {
    expect(
      draftErrors(
        draft({
          parameters: {
            "aws-connectors-role": {
              extraPolicyArns: {
                claimEditable: true,
                allowed: [
                  { name: "sqs", label: "SQS access", value: "a" },
                  { name: "sqs", label: "SQS read", value: "b" },
                ],
              },
            },
          },
        }),
        NEW,
      ),
    ).toEqual([
      {
        path: "spec.parameters.aws-connectors-role.extraPolicyArns.allowed.1.name",
        message: "Each name is used once.",
      },
    ]);
  });

  it("refuses the environment label in the fixed labels and in the required label keys", () => {
    expect(
      draftErrors(
        draft({
          namespaceCreation: {
            labels: { "fleet.camunda.io/environment": "staging" },
            requiredLabelKeys: ["fleet.camunda.io/environment"],
          },
        }),
        NEW,
      ),
    ).toEqual([
      {
        path: "spec.namespaceCreation.labels",
        message: "The fleet manager sets fleet.camunda.io/environment itself.",
      },
      {
        path: "spec.namespaceCreation.requiredLabelKeys",
        message: "The fleet manager sets fleet.camunda.io/environment itself.",
      },
    ]);
  });

  it("refuses a fixed label that is not a valid label", () => {
    expect(
      draftErrors(
        draft({ namespaceCreation: { labels: { "bad key": "x" } } }),
        NEW,
      ),
    ).toEqual([
      {
        path: "spec.namespaceCreation.labels",
        message: "Each fixed label is a valid label.",
      },
    ]);
  });

  it("refuses a required label key that is not a valid label key", () => {
    expect(
      draftErrors(
        draft({ namespaceCreation: { requiredLabelKeys: ["bad key"] } }),
        NEW,
      ),
    ).toEqual([
      {
        path: "spec.namespaceCreation.requiredLabelKeys",
        message: "A required label key is a valid label key.",
      },
    ]);
  });

  it("refuses a required label key that is set twice", () => {
    expect(
      draftErrors(
        draft({ namespaceCreation: { requiredLabelKeys: ["team", "team"] } }),
        NEW,
      ),
    ).toEqual([
      {
        path: "spec.namespaceCreation.requiredLabelKeys",
        message: "Each required label key is used once.",
      },
    ]);
  });

  it("refuses more than 32 fixed labels", () => {
    const labels = Object.fromEntries(
      Array.from({ length: 33 }, (_, i) => [`key-${i}`, "x"]),
    );
    expect(draftErrors(draft({ namespaceCreation: { labels } }), NEW)).toEqual([
      {
        path: "spec.namespaceCreation.labels",
        message: "At most 32 fixed labels.",
      },
    ]);
  });

  it("refuses more than 64 allowed releases", () => {
    const allowed = Array.from({ length: 65 }, (_, i) => `r-${i}`);
    expect(draftErrors(draft({ releases: { allowed } }), NEW)).toEqual([
      {
        path: "spec.releases.allowed",
        message: "At most 64 allowed releases.",
      },
    ]);
  });

  it("refuses more than 16 required label keys", () => {
    const keys = Array.from({ length: 17 }, (_, i) => `key-${i}`);
    expect(
      draftErrors(
        draft({ namespaceCreation: { requiredLabelKeys: keys } }),
        NEW,
      ),
    ).toEqual([
      {
        path: "spec.namespaceCreation.requiredLabelKeys",
        message: "At most 16 required label keys.",
      },
    ]);
  });
});

describe("the changes", () => {
  it("createChange applies the environment with its name and the compact spec, and names it in create", () => {
    expect(createChange(draft({ displayName: "", color: "#2E7D32" }))).toEqual({
      intent: "Create the environment staging",
      apply: [
        {
          apiVersion: "fleet.camunda.io/v1alpha1",
          kind: "Environment",
          metadata: { name: "staging" },
          spec: { deletionPolicy: "Delete", color: "#2E7D32" },
        },
      ],
      create: ["Environment//staging"],
    });
  });

  it("updateChange applies the live object with the edited spec, the resourceVersion of live as precondition, and force", () => {
    const edited = draft({
      displayName: "Stage",
      presets: { camundaCluster: [] },
    });
    const change = updateChange(live, edited);
    expect(change.apply).toHaveLength(1);
    expect(change.apply[0].metadata.labels).toEqual(live.metadata.labels);
    expect(change.apply[0].spec).toEqual(specOf(edited));
    expect(change.apply[0].status).toBeUndefined();
    expect(change.preconditions).toEqual({ "Environment//staging": "12" });
    expect(change.force).toBe(true);
    expect(change.intent).toBe("Change the environment staging");
  });

  it("updateChange sends allowed false when the edit turns namespace creation off", () => {
    const on: Environment = {
      ...live,
      spec: { ...live.spec, namespaceCreation: { allowed: true } },
    };
    const off = draftOf(on);
    off.spec.namespaceCreation = { allowed: false };
    expect(updateChange(on, off).apply[0].spec).toMatchObject({
      namespaceCreation: { allowed: false },
    });
  });

  it("updateChange leaves out the last applied file of kubectl", () => {
    const applied: Environment = {
      ...live,
      metadata: {
        ...live.metadata,
        annotations: {
          "kubectl.kubernetes.io/last-applied-configuration": "{}",
          note: "kept",
        },
      },
    };
    expect(
      updateChange(applied, draftOf(applied)).apply[0].metadata.annotations,
    ).toEqual({ note: "kept" });
  });

  it("deleteChange deletes the environment with the resourceVersion of live", () => {
    expect(deleteChange(live)).toEqual({
      intent: "Delete the environment staging",
      apply: [],
      delete: [
        {
          apiVersion: "fleet.camunda.io/v1alpha1",
          kind: "Environment",
          name: "staging",
        },
      ],
      preconditions: { "Environment//staging": "12" },
    });
  });

  it("namespaceLabelChange sets only the environment label, with the resourceVersion of the namespace and directOnly", () => {
    const ns: V1Namespace = {
      metadata: {
        name: "team-a",
        resourceVersion: "7",
        labels: { team: "a" },
        annotations: { note: "x" },
      },
      spec: { finalizers: ["kubernetes"] },
    };
    const change = namespaceLabelChange(ns, "staging");
    expect(change).toEqual({
      intent: "Add the namespace team-a to the environment staging",
      apply: [
        {
          apiVersion: "v1",
          kind: "Namespace",
          metadata: {
            name: "team-a",
            labels: { "fleet.camunda.io/environment": "staging" },
          },
        },
      ],
      preconditions: { "Namespace//team-a": "7" },
      directOnly: true,
    });
    expect(namespaceLabelChangeSchema.safeParse(change).success).toBe(true);
  });
});

describe("namespaceLabelChange of a namespace that the app wrote", () => {
  const ns: V1Namespace = {
    metadata: {
      name: "team-a",
      resourceVersion: "9",
      labels: {
        team: "payments",
        owner: "ops",
        "fleet.camunda.io/environment": "production",
      },
      annotations: { note: "kept", other: "theirs" },
      managedFields: [
        {
          manager: "fleet-manager",
          operation: "Apply",
          fieldsV1: {
            "f:metadata": {
              "f:labels": {
                "f:team": {},
                "f:fleet.camunda.io/environment": {},
              },
              "f:annotations": { "f:note": {} },
            },
          },
        },
        {
          manager: "kubectl-label",
          operation: "Update",
          fieldsV1: {
            "f:metadata": {
              "f:labels": { "f:owner": {} },
              "f:annotations": { "f:other": {} },
            },
          },
        },
      ],
    },
  };

  it("carries the labels and annotations that the app owns, and none of another manager", () => {
    const change = namespaceLabelChange(ns, "staging");
    expect(change.apply[0].metadata).toEqual({
      name: "team-a",
      labels: { team: "payments", "fleet.camunda.io/environment": "staging" },
      annotations: { note: "kept" },
    });
    expect(change.preconditions).toEqual({ "Namespace//team-a": "9" });
    expect(namespaceLabelChangeSchema.safeParse(change).success).toBe(true);
  });
});
