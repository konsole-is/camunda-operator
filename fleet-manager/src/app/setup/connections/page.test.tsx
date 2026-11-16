// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CONNECTION_SOURCE_KINDS } from "@/components/setup/connections/source-kinds";
import { list } from "@/lib/k8s/read";
import ConnectionsPage, { dynamic } from "./page";

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
  useWatchesFresh: (lists: unknown[]) => lists.map(() => false),
}));
vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(),
}));

afterEach(() => {
  cleanup();
  vi.mocked(list).mockClear();
  failing.kind = "";
});

describe("ConnectionsPage", () => {
  it("reads each kind of the sources from the cache", async () => {
    render(await ConnectionsPage());
    expect(
      vi
        .mocked(list)
        .mock.calls.map(([kind]) => kind)
        .sort(),
    ).toEqual(Object.values(CONNECTION_SOURCE_KINDS).sort());
    expect(dynamic).toBe("force-dynamic");
  });

  it("shows the error of a kind that fails to read above the tabs", async () => {
    failing.kind = "ObjectStorageConfig";
    render(await ConnectionsPage());
    expect(
      screen.getByRole("heading", { level: 1, name: "Connections" }),
    ).toBeTruthy();
    expect(screen.getByRole("alert").textContent).toBe(
      "Could not read ObjectStorageConfig: ObjectStorageConfig is forbidden",
    );
  });
});
