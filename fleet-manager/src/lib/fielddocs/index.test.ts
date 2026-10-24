import { describe, expect, it } from "vitest";
import { fieldDocs } from "@/generated/fielddocs";
import type { ClusterField } from "@/lib/fields";
import {
  apiReferenceUrl,
  fieldDocFor,
  fieldDocsOf,
  nearestFieldDoc,
} from "./index";

describe("fieldDocFor", () => {
  it("returns the doc of a known path", () => {
    expect(
      fieldDocFor("core.camunda.io", "CamundaCluster", "spec.zeebe.partitions"),
    ).toBe(
      fieldDocs["core.camunda.io"]?.CamundaCluster?.["spec.zeebe.partitions"],
    );
  });

  it.each([
    ["core.camunda.io", "CamundaCluster", "spec.nope"],
    ["core.camunda.io", "Nope", "spec"],
    ["nope.camunda.io", "CamundaCluster", "spec"],
    ["core.camunda.io", "CamundaCluster", "constructor"],
    ["core.camunda.io", "constructor", "spec"],
    ["constructor", "CamundaCluster", "spec"],
  ])("returns nothing for %s %s %s", (group, kind, path) => {
    expect(fieldDocFor(group, kind, path)).toBeUndefined();
  });
});

describe("apiReferenceUrl", () => {
  it.each([
    [
      "0.2.3",
      "https://konsole-is.github.io/camunda-operator/0.2/crds/api-reference/#zeebespec",
    ],
    [
      "v1.4.0",
      "https://konsole-is.github.io/camunda-operator/1.4/crds/api-reference/#zeebespec",
    ],
    [
      "0.0.0-dev",
      "https://konsole-is.github.io/camunda-operator/dev/crds/api-reference/#zeebespec",
    ],
    [
      "0.3.0-rc.1",
      "https://konsole-is.github.io/camunda-operator/dev/crds/api-reference/#zeebespec",
    ],
  ])("links version %s to %s", (version, url) => {
    expect(apiReferenceUrl(version, "#zeebespec")).toBe(url);
  });
});

describe("nearestFieldDoc", () => {
  it("falls back to the map field for a map key", () => {
    expect(
      nearestFieldDoc("core.camunda.io", "CamundaCluster", [
        "spec",
        "podLabels",
        "team",
      ]),
    ).toBe(fieldDocFor("core.camunda.io", "CamundaCluster", "spec.podLabels"));
  });

  it("returns nothing when no prefix has a doc", () => {
    expect(
      nearestFieldDoc("core.camunda.io", "Nope", ["spec", "zeebe"]),
    ).toBeUndefined();
  });
});

describe("fieldDocsOf", () => {
  it("keys each doc by field key and skips a parameter", () => {
    const field = (over: Partial<ClusterField>): ClusterField => ({
      key: "",
      target: { kind: "CamundaCluster" },
      group: "zeebe",
      path: "",
      label: "",
      value: undefined,
      state: { kind: "preset" },
      ...over,
    });
    const docs = fieldDocsOf([
      field({
        key: "CamundaCluster:spec.zeebe.partitions",
        docNames: ["spec", "zeebe", "partitions"],
      }),
      field({
        key: "secondary-storage/elasticsearch:spec.parameters.spec",
        target: {
          kind: "PluginRequest",
          section: "secondary-storage",
          registration: "elasticsearch",
        },
        group: "parameters",
      }),
    ]);
    expect(docs).toEqual({
      "CamundaCluster:spec.zeebe.partitions": fieldDocFor(
        "core.camunda.io",
        "CamundaCluster",
        "spec.zeebe.partitions",
      ),
    });
  });
});
