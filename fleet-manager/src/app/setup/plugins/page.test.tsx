// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { pluginRegistration } from "@/components/setup/plugins/fixtures";
import { list } from "@/lib/k8s/read";
import type { CatalogChart } from "@/startup/bundled-data";
import PluginsPage, { dynamic } from "./page";

const bundled = vi.hoisted(() => vi.fn());
vi.mock("@/lib/bundled", () => ({ getBundledData: bundled }));
const failing = vi.hoisted(() => ({ kind: "" }));
const served = vi.hoisted(() => ({ lists: {} as Record<string, unknown[]> }));
vi.mock("@/lib/k8s/read", () => ({
  list: vi.fn(async (kind: string) => {
    if (kind === failing.kind) throw new Error(`${kind} is forbidden`);
    return served.lists[kind] ?? [];
  }),
}));
// A watch answers with a snapshot of its kind, except the watch of the kind
// whose server read failed, which has not answered yet.
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (_kind: string, _opts: unknown, initial: unknown[]) => ({
    items: initial,
  }),
  useWatchesFresh: (lists: { kind: string }[]) =>
    lists.map(({ kind }) => kind !== failing.kind),
}));

const chart = (name: string): CatalogChart => ({
  name,
  description: `The ${name} chart.`,
  sections: ["backup-storage"],
  chart: `oci://example/${name}`,
  version: "0.1.0",
  repository: "https://example.com/charts",
  installCommand: `helm install ${name} oci://example/${name} --version 0.1.0`,
});

const catalogPanel = () =>
  screen.getByRole("region", { name: "Charts you can install" });

afterEach(() => {
  cleanup();
  vi.mocked(list).mockClear();
  bundled.mockReset();
  failing.kind = "";
  served.lists = {};
});

describe("PluginsPage", () => {
  it("reads each kind of the sources from the cache", async () => {
    bundled.mockResolvedValue({ catalog: { schemaVersion: 1, charts: [] } });
    render(await PluginsPage());
    expect(
      vi
        .mocked(list)
        .mock.calls.map(([kind]) => kind)
        .sort(),
    ).toEqual([
      "Environment",
      "Namespace",
      "PluginRegistration",
      "PluginRequest",
      "PluginSection",
    ]);
    expect(dynamic).toBe("force-dynamic");
  });

  it("says that the catalog could not be read", async () => {
    bundled.mockRejectedValueOnce(new Error("reading catalog /c.json: ENOENT"));
    render(await PluginsPage());
    expect(
      screen.getByRole("heading", { level: 1, name: "Plugins" }),
    ).toBeTruthy();
    expect(within(catalogPanel()).getByRole("alert").textContent).toBe(
      "Could not read the chart catalog: reading catalog /c.json: ENOENT",
    );
  });

  it("lists every chart of the catalog when no registration is installed", async () => {
    bundled.mockResolvedValueOnce({
      catalog: { schemaVersion: 1, charts: [chart("rustfs")] },
    });
    render(await PluginsPage());
    expect(within(catalogPanel()).getByText("rustfs")).toBeTruthy();
  });

  it("shows no list as empty and no chart as not installed after a failed read of PluginRegistration", async () => {
    failing.kind = "PluginRegistration";
    bundled.mockResolvedValueOnce({
      catalog: { schemaVersion: 1, charts: [chart("rustfs")] },
    });
    render(await PluginsPage());
    expect(screen.getByText(/^Not read: PluginRegistration\./)).toBeTruthy();
    expect(screen.getByText("Health not known")).toBeTruthy();
    expect(
      screen.queryByText("No option is installed for this section."),
    ).toBeNull();
    expect(within(catalogPanel()).queryByText("rustfs")).toBeNull();
  });

  it("calls no registration lost after a failed read of PluginSection", async () => {
    failing.kind = "PluginSection";
    served.lists = {
      PluginRegistration: [
        pluginRegistration("elasticsearch", {
          sections: ["secondary-storage"],
        }),
      ],
    };
    bundled.mockResolvedValueOnce({
      catalog: { schemaVersion: 1, charts: [] },
    });
    render(await PluginsPage());
    expect(screen.getByText(/^Not read: PluginSection\./)).toBeTruthy();
    expect(
      screen.queryByRole("region", { name: "Not in a section" }),
    ).toBeNull();
    expect(
      screen.getByRole("region", { name: "Section not known" }),
    ).toBeTruthy();
  });
});
