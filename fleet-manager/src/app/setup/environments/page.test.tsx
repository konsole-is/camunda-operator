// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { list } from "@/lib/k8s/read";
import EnvironmentsPage, { dynamic } from "./page";

const failing = vi.hoisted(() => ({ kind: "" }));
vi.mock("@/lib/k8s/read", () => ({
  list: vi.fn(async (kind: string) => {
    if (kind === failing.kind) throw new Error(`${kind} is forbidden`);
    return [];
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

describe("EnvironmentsPage", () => {
  it("reads each kind of the sources from the cache", async () => {
    render(await EnvironmentsPage());
    expect(
      vi
        .mocked(list)
        .mock.calls.map(([kind]) => kind)
        .sort(),
    ).toEqual([
      "CamundaCluster",
      "CamundaClusterClaim",
      "CamundaClusterPreset",
      "CamundaManagementCluster",
      "CamundaManagementClusterClaim",
      "CamundaPlatformConfig",
      "CamundaRelease",
      "DatabaseServerPreset",
      "ElasticsearchClusterPreset",
      "Environment",
      "Namespace",
      "PluginRegistration",
      "PluginRequest",
      "PluginSection",
    ]);
    expect(dynamic).toBe("force-dynamic");
  });

  it("renders the list when a read fails", async () => {
    failing.kind = "Environment";
    render(await EnvironmentsPage());
    expect(
      screen.getByRole("heading", { level: 1, name: "Environments" }),
    ).toBeTruthy();
  });
});
