import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { DESCRIPTION_ANNOTATION, LAST_APPLIED_ANNOTATION } from "@/lib/labels";
import { refKey } from "@/lib/write/refs";
import { presetChangeCount, presetDraftOf, presetUpdateChange } from "./draft";
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
