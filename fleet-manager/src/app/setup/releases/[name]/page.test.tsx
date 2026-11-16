// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { listSecretRefsIn } from "@/lib/secrets";
import ReleasePage, { dynamic } from "./page";

const fixtures = vi.hoisted(() => ({
  releases: [
    {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaRelease",
      metadata: { name: "camunda-8-9-0" },
      spec: { version: "8.9.0" },
    },
  ],
  clusters: [
    {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaCluster",
      metadata: { namespace: "team-orders", name: "orders" },
      spec: { releaseRef: "camunda-8-9-0" },
    },
  ],
}));
const failing = vi.hoisted(() => ({ bundled: false }));
const seen = vi.hoisted(() => ({ props: undefined as unknown }));

vi.mock("next/navigation", () => ({
  notFound: vi.fn(() => {
    throw new Error("NEXT_NOT_FOUND");
  }),
}));
vi.mock("@/lib/k8s/read", () => ({
  list: vi.fn(async (kind: string) =>
    kind === "CamundaRelease"
      ? fixtures.releases
      : kind === "CamundaCluster"
        ? fixtures.clusters
        : [],
  ),
  get: vi.fn(async (_kind: string, _ns: unknown, name: string) =>
    fixtures.releases.find((r) => r.metadata.name === name),
  ),
}));
vi.mock("@/lib/write/writer", () => ({
  writeMode: vi.fn(async () => "direct"),
}));
vi.mock("@/lib/bundled", () => ({
  getBundledData: vi.fn(async () => {
    if (failing.bundled) throw new Error("no table");
    return { imagesTable: { schemaVersion: 1, minors: { "8.9": {} } } };
  }),
}));
vi.mock("@/lib/secrets", () => ({
  listSecretRefsIn: vi.fn(async () => ({ "team-orders": [] })),
}));
vi.mock("@/app/setup/releases/actions", () => ({
  previewReleaseChange: vi.fn(),
  applyReleaseChange: vi.fn(),
  previewReleaseMove: vi.fn(),
  applyReleaseMove: vi.fn(),
}));
vi.mock("@/components/setup/releases/release-detail", () => ({
  ReleaseDetail: (props: { data: { name: string }; tab: string }) => {
    seen.props = props;
    return (
      <p>
        release {props.data.name} {props.tab}
      </p>
    );
  },
}));

afterEach(() => {
  cleanup();
  failing.bundled = false;
  vi.mocked(listSecretRefsIn).mockClear();
});

const params = (name: string, tab?: string) => ({
  params: Promise.resolve({ name }),
  searchParams: Promise.resolve(tab === undefined ? {} : { tab }),
});

const data = () =>
  (seen.props as { data: { imagesTable: unknown; secrets: unknown } }).data;

describe("ReleasePage", () => {
  it("shows not found for a release that does not exist", async () => {
    await expect(ReleasePage(params("gone"))).rejects.toThrow("NEXT_NOT_FOUND");
  });

  it("reads the Secret refs of the namespaces of the users", async () => {
    render(await ReleasePage(params("camunda-8-9-0")));
    expect(listSecretRefsIn).toHaveBeenCalledWith(["team-orders"]);
    expect(data().secrets).toEqual({ "team-orders": [] });
    expect(dynamic).toBe("force-dynamic");
  });

  it("passes the images table of the bundled data", async () => {
    render(await ReleasePage(params("camunda-8-9-0")));
    expect(data().imagesTable).toEqual({
      schemaVersion: 1,
      minors: { "8.9": {} },
    });
  });

  it("passes an empty images table when the table cannot be read", async () => {
    failing.bundled = true;
    render(await ReleasePage(params("camunda-8-9-0")));
    expect(data().imagesTable).toEqual({ schemaVersion: 1, minors: {} });
  });

  it("selects the Settings tab for an unknown tab", async () => {
    render(await ReleasePage(params("camunda-8-9-0", "nope")));
    expect(screen.getByText("release camunda-8-9-0 settings")).toBeTruthy();
  });
});
