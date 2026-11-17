// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { cloneElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { list } from "@/lib/k8s/read";
import PlatformPage, { dynamic } from "./page";

const failing = vi.hoisted(() => ({ kind: "" }));
const watch = vi.hoisted(() => ({ answered: false }));
vi.mock("@/lib/k8s/read", () => ({
  list: vi.fn(async (kind: string) => {
    if (kind === failing.kind) throw new Error(`${kind} is forbidden`);
    return kind === "CamundaPlatformConfig"
      ? [
          {
            apiVersion: "core.camunda.io/v1",
            kind,
            metadata: { name: "basic-lab" },
            spec: {},
          },
        ]
      : [];
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

describe("PlatformPage", () => {
  it("reads each kind of the sources from the cache", async () => {
    render(await PlatformPage());
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
      "DatabaseServer",
      "DatabaseServerPreset",
      "Environment",
      "Namespace",
    ]);
    expect(dynamic).toBe("force-dynamic");
  });

  it("renders the list when a read fails", async () => {
    failing.kind = "CamundaRelease";
    render(await PlatformPage());
    expect(
      screen.getByRole("heading", { level: 1, name: "Platform" }),
    ).toBeTruthy();
  });

  it("shows the error of a failed server read until the watch of its kind answers", async () => {
    failing.kind = "Environment";
    const page = await PlatformPage();
    const { rerender } = render(page);
    const readErrors = () =>
      screen
        .queryAllByRole("alert")
        .map((alert) => alert.textContent)
        .filter((text) => text?.startsWith("Could not read"));
    expect(readErrors()).toEqual([
      "Could not read Environment: Environment is forbidden",
    ]);
    watch.answered = true;
    rerender(cloneElement(page));
    expect(readErrors()).toEqual([]);
  });

  it("shows no user count and names the kind whose server read failed until its watch answers", async () => {
    failing.kind = "CamundaCluster";
    render(await PlatformPage());
    const row = screen.getByRole("row", { name: /basic-lab/ });
    expect(within(row).getAllByRole("cell")[2].textContent).toBe("Not known");
    expect(
      screen.getByText(
        "The users are not known while the fleet manager cannot read CamundaCluster.",
      ),
    ).toBeTruthy();
  });
});
