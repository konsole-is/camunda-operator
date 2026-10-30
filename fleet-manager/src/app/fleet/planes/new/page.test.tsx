// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { list } from "@/lib/k8s/read";
import NewPlanePage, { dynamic } from "./page";

const data: Record<string, unknown[]> = {
  Environment: [
    {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "Environment",
      metadata: { name: "production" },
      spec: { platformConfigRef: "default-platform" },
    },
  ],
  Namespace: [],
  CamundaCluster: [],
  CamundaPlatformConfig: [],
  CamundaManagementCluster: [],
  CamundaManagementClusterClaim: [],
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
  vi.mocked(list).mockClear();
  failing.kind = "";
});

describe("NewPlanePage", () => {
  it("reads the first snapshot of each kind on each request", async () => {
    render(await NewPlanePage());
    expect(
      vi
        .mocked(list)
        .mock.calls.map(([kind]) => kind)
        .sort(),
    ).toEqual([
      "CamundaCluster",
      "CamundaManagementCluster",
      "CamundaManagementClusterClaim",
      "CamundaPlatformConfig",
      "Environment",
      "Namespace",
    ]);
    expect(
      screen.getByRole("heading", { name: "New management plane" }),
    ).toBeTruthy();
    expect(screen.getByRole("radio", { name: /production/ })).toBeTruthy();
    expect(dynamic).toBe("force-dynamic");
  });

  it("a failed read of one kind still shows the flow", async () => {
    failing.kind = "CamundaCluster";
    render(await NewPlanePage());
    expect(screen.getByRole("radio", { name: /production/ })).toBeTruthy();
  });
});
