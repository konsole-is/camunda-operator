// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { PlaneCreateData } from "@/components/planes/create/plane-create-flow";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import { list } from "@/lib/k8s/read";
import { fakeWriter } from "../../../../../test/fakes/writer";
import NewPlanePage, { dynamic } from "./page";

vi.mock("@/lib/write/writer", () => import("../../../../../test/fakes/writer"));

const show = async () =>
  render(await NewPlanePage(), { wrapper: GitPushesProvider });

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
  CamundaRelease: [],
  CamundaManagementCluster: [],
  CamundaManagementClusterClaim: [],
  PluginSection: [],
  PluginRegistration: [],
  PluginRequest: [],
  ManagementAuthConfig: [],
  DatabaseServerPreset: [],
  DatabaseConfig: [],
};

const table = {
  schemaVersion: 1,
  minors: { "8.9": { identity: { version: "8.9.10" } } },
};

const failing = vi.hoisted(() => ({ kind: "", bundled: false }));
const seen = vi.hoisted(() => ({ props: undefined as unknown }));
vi.mock("@/startup/bundled-data", () => ({
  loadBundledData: vi.fn(async () => {
    if (failing.bundled) throw new Error("no table");
    return { imagesTable: table };
  }),
}));
vi.mock(
  "@/components/planes/create/plane-create-flow",
  async (importOriginal) => {
    const actual =
      await importOriginal<
        typeof import("@/components/planes/create/plane-create-flow")
      >();
    return {
      ...actual,
      PlaneCreateFlow: (
        props: Parameters<typeof actual.PlaneCreateFlow>[0],
      ) => {
        seen.props = props;
        return <actual.PlaneCreateFlow {...props} />;
      },
    };
  },
);
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
  failing.bundled = false;
  fakeWriter.reset();
});

const props = () =>
  seen.props as { initial: PlaneCreateData; mode: string } | undefined;
const initial = () => props()?.initial;

describe("NewPlanePage", () => {
  it("reads the first snapshot of each kind on each request", async () => {
    await show();
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
      "CamundaRelease",
      "DatabaseConfig",
      "DatabaseServerPreset",
      "Environment",
      "ManagementAuthConfig",
      "Namespace",
      "PluginRegistration",
      "PluginRequest",
      "PluginSection",
    ]);
    expect(
      screen.getByRole("heading", { name: "New management plane" }),
    ).toBeTruthy();
    expect(screen.getByRole("radio", { name: /production/ })).toBeTruthy();
    expect(dynamic).toBe("force-dynamic");
  });

  it("reads only the access requests", async () => {
    await show();
    expect(
      vi.mocked(list).mock.calls.find(([kind]) => kind === "PluginRequest"),
    ).toEqual([
      "PluginRequest",
      { labelSelector: "plugins.camunda.io/section=access" },
    ]);
  });

  it("a failed read of one kind still shows the flow", async () => {
    failing.kind = "CamundaCluster";
    await show();
    expect(screen.getByRole("radio", { name: /production/ })).toBeTruthy();
  });

  it("passes the images table of the bundled data", async () => {
    await show();
    expect(initial()?.images).toEqual(table);
  });

  it("passes an empty images table when the table cannot be read", async () => {
    failing.bundled = true;
    await show();
    expect(initial()?.images).toEqual({ schemaVersion: 1, minors: {} });
  });

  it("passes the mode of writeMode", async () => {
    fakeWriter.reset("git");
    await show();
    expect(props()?.mode).toBe("git");
  });
});
