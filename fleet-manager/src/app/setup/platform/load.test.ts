import { afterEach, describe, expect, it, vi } from "vitest";
import { list } from "@/lib/k8s/read";
import { loadImagesTable, loadPlatformSources } from "./load";

const failing = vi.hoisted(() => ({ kind: "", bundled: false }));
vi.mock("@/lib/k8s/read", () => ({
  list: vi.fn(async (kind: string) => {
    if (kind === failing.kind) throw new Error(`${kind} is forbidden`);
    return [{ kind, metadata: { name: `a-${kind}` } }];
  }),
}));
vi.mock("@/lib/bundled", () => ({
  getBundledData: vi.fn(async () => {
    if (failing.bundled) throw new Error("no table");
    return { imagesTable: { schemaVersion: 1, minors: { "8.9": {} } } };
  }),
}));

afterEach(() => {
  vi.mocked(list).mockClear();
  failing.kind = "";
  failing.bundled = false;
});

describe("loadPlatformSources", () => {
  it("reads each kind of the sources", async () => {
    const sources = await loadPlatformSources();
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
    expect(sources.planes).toEqual([
      {
        kind: "CamundaManagementCluster",
        metadata: { name: "a-CamundaManagementCluster" },
      },
    ]);
  });

  it("gives no objects and a read error for a kind whose read fails", async () => {
    failing.kind = "DatabaseServer";
    const sources = await loadPlatformSources();
    expect(sources.databaseServers).toEqual([]);
    expect(sources.clusters).toHaveLength(1);
    expect(sources.readErrors).toEqual([
      { kind: "DatabaseServer", message: "DatabaseServer is forbidden" },
    ]);
  });
});

describe("loadImagesTable", () => {
  it("gives the table of the bundled data", async () => {
    expect(await loadImagesTable()).toEqual({
      schemaVersion: 1,
      minors: { "8.9": {} },
    });
  });

  it("gives an empty table when the bundled data cannot be read", async () => {
    failing.bundled = true;
    expect(await loadImagesTable()).toEqual({ schemaVersion: 1, minors: {} });
  });
});
