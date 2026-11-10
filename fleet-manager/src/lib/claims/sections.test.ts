import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import type { PluginSelection } from "@/lib/policy";
import { type SectionClaim, sectionComponents } from "./sections";

const sel = (section: string, registration = `${section}-option`) =>
  ({ section, registration }) satisfies PluginSelection;

const claimOf = (
  plugins: PluginSelection[],
  requests?: { section: string; name: string }[],
): SectionClaim => ({
  metadata: { namespace: "team-a" },
  spec: { plugins },
  ...(requests ? { status: { requests } } : {}),
});

const section = (name: string, label: string, order?: number) =>
  ({
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginSection",
    metadata: { name },
    spec: {
      appliesTo: ["CamundaClusterClaim"],
      label,
      selection: "One",
      ...(order === undefined ? {} : { order }),
    },
  }) as Resource<"PluginSection">;

const request = (
  namespace: string,
  name: string,
  ready?: { status: "True" | "False" | "Unknown"; reason: string },
) =>
  ({
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRequest",
    metadata: { namespace, name },
    spec: {
      claimRef: { kind: "CamundaClusterClaim", name: "orders" },
      deletionPolicy: "Delete",
      registration: "elasticsearch",
      section: "secondary-storage",
      target: { kind: "CamundaCluster", name: "orders" },
    },
    status: ready
      ? {
          conditions: [
            {
              type: "Ready",
              ...ready,
              message: "The grant was rejected",
              lastTransitionTime: "2026-11-10T10:00:00Z",
            },
          ],
        }
      : undefined,
  }) as Resource<"PluginRequest">;

describe("sectionComponents", () => {
  it("gives one component for each selection in the order of the sections", () => {
    const result = sectionComponents({
      claim: claimOf([sel("access"), sel("secondary-storage")]),
      sections: [
        section("secondary-storage", "Secondary storage", 50),
        section("access", "Access", 60),
      ],
      requests: [],
    });
    expect(result.map((c) => [c.id, c.label, c.order])).toEqual([
      ["section:secondary-storage", "Secondary storage", 50],
      ["section:access", "Access", 60],
    ]);
    expect(result[0].selection).toEqual(sel("secondary-storage"));
  });

  it("gives no component to a section without a selection", () => {
    const result = sectionComponents({
      claim: claimOf([sel("secondary-storage")]),
      sections: [
        section("backup-storage", "Backup storage", 40),
        section("secondary-storage", "Secondary storage", 50),
      ],
      requests: [],
    });
    expect(result.map((c) => c.section)).toEqual(["secondary-storage"]);
  });

  it("names a section that is not found by its name and sorts it last", () => {
    const result = sectionComponents({
      claim: claimOf([sel("volume-encryption"), sel("access")]),
      sections: [section("access", "Access", 60)],
      requests: [],
    });
    expect(result.map((c) => [c.label, c.order])).toEqual([
      ["Access", 60],
      ["volume-encryption", Number.MAX_SAFE_INTEGER],
    ]);
  });

  it("sorts sections of the same order by name", () => {
    const result = sectionComponents({
      claim: claimOf([sel("b"), sel("a")]),
      sections: [section("b", "B", 10), section("a", "A", 10)],
      requests: [],
    });
    expect(result.map((c) => c.section)).toEqual(["a", "b"]);
  });

  it("takes the request that the claim status names and its Ready condition", () => {
    const [component] = sectionComponents({
      claim: claimOf(
        [sel("secondary-storage")],
        [{ section: "secondary-storage", name: "orders-es" }],
      ),
      sections: [section("secondary-storage", "Secondary storage", 50)],
      requests: [
        request("team-a", "orders-es", {
          status: "False",
          reason: "GrantRejected",
        }),
      ],
    });
    expect(component.request?.metadata?.name).toBe("orders-es");
    expect(component.condition).toMatchObject({
      status: "False",
      reason: "GrantRejected",
    });
  });

  it("ignores a request of the same name in another namespace", () => {
    const [component] = sectionComponents({
      claim: claimOf(
        [sel("secondary-storage")],
        [{ section: "secondary-storage", name: "orders-es" }],
      ),
      sections: [section("secondary-storage", "Secondary storage", 50)],
      requests: [
        request("team-b", "orders-es", { status: "True", reason: "Ready" }),
      ],
    });
    expect(component.request).toBeUndefined();
    expect(component.condition).toBeUndefined();
  });

  it("has no request while the claim status names none", () => {
    const [component] = sectionComponents({
      claim: claimOf([sel("secondary-storage")]),
      sections: [section("secondary-storage", "Secondary storage", 50)],
      requests: [
        request("team-a", "orders-es", { status: "True", reason: "Ready" }),
      ],
    });
    expect(component.request).toBeUndefined();
    expect(component.condition).toBeUndefined();
  });
});
