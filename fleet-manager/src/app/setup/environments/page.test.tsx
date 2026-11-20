// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { cloneElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { list } from "@/lib/k8s/read";
import EnvironmentsPage, { dynamic } from "./page";

const failing = vi.hoisted(() => ({ kind: "" }));
const watch = vi.hoisted(() => ({ answered: false }));
vi.mock("@/lib/k8s/read", () => ({
  list: vi.fn(async (kind: string) => {
    if (kind === failing.kind) throw new Error(`${kind} is forbidden`);
    return [];
  }),
}));
// The watch of a kind whose server read failed has not answered yet.
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (_kind: string, _opts: unknown, initial: unknown[]) => ({
    items: initial,
  }),
  useWatchesFresh: (lists: { kind: string }[]) =>
    lists.map(({ kind }) => watch.answered || kind !== failing.kind),
}));

afterEach(() => {
  cleanup();
  vi.mocked(list).mockClear();
  failing.kind = "";
  watch.answered = false;
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

  it("shows the error of a failed server read until the watch of its kind answers", async () => {
    failing.kind = "CamundaRelease";
    const page = await EnvironmentsPage();
    const { rerender } = render(page);
    const readErrors = () =>
      screen
        .queryAllByRole("alert")
        .map((alert) => alert.textContent)
        .filter((text) => text?.startsWith("Could not read"));
    expect(readErrors()).toEqual([
      "Could not read CamundaRelease: CamundaRelease is forbidden",
    ]);
    watch.answered = true;
    rerender(cloneElement(page));
    expect(readErrors()).toEqual([]);
  });
});
