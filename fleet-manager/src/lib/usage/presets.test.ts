import { describe, expect, it } from "vitest";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import { presetUsage } from "./presets";

function cluster(namespace: string, name: string, presetRef?: string) {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaCluster",
    metadata: { namespace, name },
    spec: { ...(presetRef && { presetRef }) },
  } as CamundaCluster;
}

describe("presetUsage", () => {
  it("lists the clusters of each preset by their presetRef, sorted", () => {
    expect(
      presetUsage([
        cluster("team-b", "billing", "small"),
        cluster("team-a", "orders", "small"),
        cluster("team-a", "search"),
      ]),
    ).toEqual(new Map([["small", ["team-a/orders", "team-b/billing"]]]));
  });

  it("gives no entry without clusters", () => {
    expect(presetUsage([])).toEqual(new Map());
  });
});
