// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { list } from "@/lib/k8s/read";
import PlanesPage, { dynamic } from "./page";

const data: Record<string, unknown[]> = {
  CamundaManagementCluster: [
    {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaManagementCluster",
      metadata: {
        namespace: "platform",
        name: "plane-a",
        labels: { "fleet.camunda.io/claim": "plane-a" },
      },
      spec: { identityProvider: { keycloak: {} } },
    },
  ],
  CamundaManagementClusterClaim: [
    {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "CamundaManagementClusterClaim",
      metadata: { namespace: "platform", name: "plane-a" },
      spec: {
        environments: ["production"],
        management: { identityProvider: { keycloak: {} } },
      },
    },
  ],
  Namespace: [],
};

const failing = vi.hoisted(() => ({ kind: "" }));
vi.mock("@/lib/k8s/read", () => ({
  list: vi.fn(async (kind: string) => {
    if (kind === failing.kind) throw new Error(`${kind} is forbidden`);
    return data[kind];
  }),
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (_kind: string, _opts: unknown, initial: unknown[]) => ({
    items: initial,
  }),
}));

afterEach(() => {
  cleanup();
  failing.kind = "";
});

describe("PlanesPage", () => {
  it("renders the planes of the cache on each request", async () => {
    render(await PlanesPage());
    expect(
      vi
        .mocked(list)
        .mock.calls.map(([kind]) => kind)
        .sort(),
    ).toEqual([
      "CamundaManagementCluster",
      "CamundaManagementClusterClaim",
      "Namespace",
    ]);
    expect(screen.getByRole("row", { name: /plane-a/ })).toBeTruthy();
    expect(dynamic).toBe("force-dynamic");
  });

  it("a failed first read of one kind still shows the page", async () => {
    failing.kind = "Namespace";
    render(await PlanesPage());
    expect(screen.getByRole("row", { name: /plane-a/ })).toBeTruthy();
  });
});
