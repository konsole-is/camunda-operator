import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { clusterOptimize } from "./optimize";

const optimize = (name: string, cluster: string, created: string) =>
  ({
    kind: "CamundaOptimize",
    metadata: { name, namespace: "team-a", creationTimestamp: created },
    spec: { clusterRef: { name: cluster } },
  }) as unknown as Resource<"CamundaOptimize">;

describe("clusterOptimize", () => {
  it("returns the Optimize of the cluster and leaves out those of other clusters", () => {
    expect(
      clusterOptimize("orders", [
        optimize("other", "payments", "2026-10-01T00:00:00Z"),
        optimize("orders-optimize", "orders", "2026-10-02T00:00:00Z"),
      ])?.metadata?.name,
    ).toBe("orders-optimize");
  });

  it("returns the oldest when a second Optimize names the same cluster", () => {
    expect(
      clusterOptimize("orders", [
        optimize("b-new", "orders", "2026-10-03T00:00:00Z"),
        optimize("a-old", "orders", "2026-10-01T00:00:00Z"),
      ])?.metadata?.name,
    ).toBe("a-old");
  });

  it("orders two Optimize objects of the same age by name", () => {
    expect(
      clusterOptimize("orders", [
        optimize("b", "orders", "2026-10-01T00:00:00Z"),
        optimize("a", "orders", "2026-10-01T00:00:00Z"),
      ])?.metadata?.name,
    ).toBe("a");
  });

  it("returns undefined for a cluster without Optimize", () => {
    expect(clusterOptimize("orders", [])).toBeUndefined();
  });
});
