import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { DESCRIPTION_ANNOTATION, LAST_APPLIED_ANNOTATION } from "@/lib/labels";
import { foreignFields } from "@/lib/write/ownership";
import type { KubeObject } from "@/lib/write/types";
import { refKey } from "@/lib/write/refs";
import {
  createChange,
  nameErrors,
  type PresetDraft,
  presetChangeCount,
  presetDraftOf,
  presetUpdateChange,
} from "./draft";
import { presetFixtures } from "./fixtures";

const sources = presetFixtures();
const medium = sources.clusterPresets[0];
const standardPg = sources.databasePresets[1];

describe("presetDraftOf", () => {
  it("holds the body and the description of the preset", () => {
    expect(presetDraftOf(standardPg)).toEqual({
      body: standardPg.spec.server,
      description: "",
    });
    expect(presetDraftOf(medium).description).toBe(
      "For production workloads with three brokers and replication.",
    );
  });

  it("gives an empty draft without a preset", () => {
    expect(presetDraftOf(undefined)).toEqual({ body: {}, description: "" });
  });
});

describe("presetUpdateChange", () => {
  it("applies the edited body and description with the precondition and no force", () => {
    const draft = {
      body: { ...standardPg.spec.server, instances: 3 },
      description: "  Two replicas for the plane  ",
    };
    expect(presetUpdateChange(standardPg, draft)).toEqual({
      intent: "Change the preset standard",
      apply: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "DatabaseServerPreset",
          metadata: {
            name: "standard",
            annotations: {
              [DESCRIPTION_ANNOTATION]: "Two replicas for the plane",
            },
          },
          spec: { server: { ...standardPg.spec.server, instances: 3 } },
        },
      ],
      preconditions: {
        [refKey({ kind: "DatabaseServerPreset", name: "standard" })]: "14",
      },
    });
  });

  it("writes the body of a cluster preset under spec.cluster", () => {
    const draft = presetDraftOf(medium);
    expect(presetUpdateChange(medium, draft).apply[0].spec).toEqual(
      medium.spec,
    );
  });

  it("removes the description when it is blank", () => {
    const change = presetUpdateChange(medium, {
      ...presetDraftOf(medium),
      description: "   ",
    });
    expect(change.apply[0].metadata.annotations).toBeUndefined();
  });

  it("leaves out the last applied file of kubectl and keeps the labels and other annotations", () => {
    const applied = {
      ...medium,
      metadata: {
        ...medium.metadata,
        labels: { team: "platform" },
        annotations: {
          ...medium.metadata.annotations,
          [LAST_APPLIED_ANNOTATION]: "{}",
          "example.com/owner": "platform",
        },
      },
    } as Resource<"CamundaClusterPreset">;
    const { metadata } = presetUpdateChange(applied, presetDraftOf(applied))
      .apply[0];
    expect(metadata.labels).toEqual({ team: "platform" });
    expect(metadata.annotations).toEqual({
      [DESCRIPTION_ANNOTATION]:
        "For production workloads with three brokers and replication.",
      "example.com/owner": "platform",
    });
  });
});

describe("presetChangeCount", () => {
  it("counts no change for the live preset", () => {
    expect(presetChangeCount(medium, presetDraftOf(medium))).toBe(0);
  });

  it("counts each top-level field of the body and the description that change", () => {
    const draft = presetDraftOf(medium);
    expect(
      presetChangeCount(medium, {
        body: { ...draft.body, zeebe: { replicas: 5 }, externalUrl: "x" },
        description: "Other",
      }),
    ).toBe(3);
  });

  it("counts a description that differs only in spaces as no change", () => {
    const draft = presetDraftOf(medium);
    expect(
      presetChangeCount(medium, {
        ...draft,
        description: ` ${draft.description} `,
      }),
    ).toBe(0);
  });
});

describe("presetUpdateChange over the empty values of another tool", () => {
  // presetWith is the preset standard with a description and an instance
  // count that kubectl set.
  const presetWith = (description: string) => ({
    ...standardPg,
    metadata: {
      ...standardPg.metadata,
      annotations: { [DESCRIPTION_ANNOTATION]: description },
      managedFields: [
        {
          manager: "kubectl-client-side-apply",
          operation: "Update",
          fieldsV1: {
            "f:metadata": {
              "f:annotations": { [`f:${DESCRIPTION_ANNOTATION}`]: {} },
            },
            "f:spec": { "f:server": { "f:instances": {} } },
          },
        },
      ],
    },
  });
  const rows = (live: typeof standardPg, draft: PresetDraft) => {
    const change = presetUpdateChange(live, draft);
    const { changed, cleared } = foreignFields(
      live as unknown as KubeObject,
      change.apply[0],
      change.keep?.[refKey({ kind: "DatabaseServerPreset", name: "standard" })],
    );
    return {
      changed: changed.map((f) => f.path),
      cleared: cleared.map((f) => f.path),
    };
  };
  const moreInstances = (live: typeof standardPg): PresetDraft => {
    const draft = presetDraftOf(live);
    return { ...draft, body: { ...draft.body, instances: 3 } };
  };

  it("shows only the changed field", () => {
    const live = presetWith("");
    expect(rows(live, moreInstances(live))).toEqual({
      changed: ["spec.server.instances"],
      cleared: [],
    });
  });

  it("still shows a description that the edit cleared", () => {
    const live = presetWith("Two replicas");
    expect(
      rows(live, { ...moreInstances(live), description: "" }).cleared,
    ).toEqual([`metadata.annotations.${DESCRIPTION_ANNOTATION}`]);
  });
});

const NAME_RULE =
  "The name has only lowercase letters, digits, and hyphens, and at most 253 characters.";

describe("nameErrors", () => {
  it.each([
    ["", "Give the preset a name."],
    ["team.a", NAME_RULE],
    ["Small", NAME_RULE],
    ["small-", NAME_RULE],
    ["-small", NAME_RULE],
    ["a".repeat(254), NAME_RULE],
    ["standard", "A preset of this kind with this name exists."],
  ])("refuses the name %j", (name, message) => {
    expect(nameErrors(name, ["standard"])).toEqual([{ path: "name", message }]);
  });

  it("accepts a name that a claim can name", () => {
    expect(nameErrors("standard-two", ["standard"])).toEqual([]);
    expect(nameErrors("7", [])).toEqual([]);
    expect(nameErrors("a".repeat(253), [])).toEqual([]);
  });
});

describe("createChange", () => {
  it.each([
    ["CamundaClusterPreset", "cluster", { zeebe: { replicas: 1 } }],
    ["ElasticsearchClusterPreset", "cluster", { replicas: 1 }],
    ["DatabaseServerPreset", "server", { instances: 1 }],
  ] as const)(
    "creates a %s with its body under spec.%s",
    (kind, field, body) => {
      expect(createChange(kind, "lab", { body, description: "" })).toEqual({
        intent: "Create the preset lab",
        apply: [
          {
            apiVersion: "core.camunda.io/v1",
            kind,
            metadata: { name: "lab" },
            spec: { [field]: body },
          },
        ],
        create: [refKey({ kind, name: "lab" })],
      });
    },
  );

  it("creates with the name and the body only, without a precondition", () => {
    const change = createChange("CamundaClusterPreset", "large", {
      body: presetDraftOf(medium).body,
      description: "",
    });
    expect(change.apply[0].metadata).toEqual({ name: "large" });
    expect(change.apply[0].spec).toEqual(medium.spec);
    expect(change.preconditions).toBeUndefined();
  });

  it("writes the trimmed description as the annotation", () => {
    const change = createChange("DatabaseServerPreset", "lab", {
      body: {},
      description: "  For labs. ",
    });
    expect(change.apply[0].metadata.annotations).toEqual({
      [DESCRIPTION_ANNOTATION]: "For labs.",
    });
  });

  it("writes an empty body as an empty map, since the CRD requires it", () => {
    expect(
      createChange(
        "ElasticsearchClusterPreset",
        "lab",
        presetDraftOf(undefined),
      ).apply[0].spec,
    ).toEqual({ cluster: {} });
  });
});
