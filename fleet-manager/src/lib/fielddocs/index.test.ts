import { describe, expect, it } from "vitest";
import { fieldDocs } from "@/generated/fielddocs";
import { apiReferenceUrl, fieldDocFor } from "./index";

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
