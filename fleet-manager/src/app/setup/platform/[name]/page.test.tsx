// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import PlatformConfigPage, { dynamic } from "./page";

const config = {
  apiVersion: "core.camunda.io/v1",
  kind: "CamundaPlatformConfig",
  metadata: { name: "basic-lab", resourceVersion: "4", uid: "u-1" },
  spec: {},
};
const failing = vi.hoisted(() => ({ kind: "", bundled: false }));
const seen = vi.hoisted(() => ({ imagesTable: undefined as unknown }));

vi.mock("next/navigation", () => ({
  notFound: vi.fn(() => {
    throw new Error("NEXT_NOT_FOUND");
  }),
  useRouter: () => ({ push: vi.fn() }),
}));
vi.mock("@/lib/k8s/read", () => ({
  list: vi.fn(async (kind: string) => {
    if (kind === failing.kind) throw new Error(`${kind} is forbidden`);
    return kind === "CamundaPlatformConfig" ? [config] : [];
  }),
  get: vi.fn(async (_kind: string, _ns: unknown, name: string) =>
    name === config.metadata.name ? config : undefined,
  ),
}));
// The watch of a kind whose server read failed has not answered yet.
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (_kind: string, _opts: unknown, initial: unknown[]) => ({
    items: initial,
  }),
  useWatchesFresh: (lists: { kind: string }[]) =>
    lists.map(({ kind }) => kind !== failing.kind),
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
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
vi.mock("@/app/setup/platform/actions", () => ({
  previewPlatformConfigChange: vi.fn(),
  applyPlatformConfigChange: vi.fn(),
}));
vi.mock(
  "@/components/setup/platform/platform-detail",
  async (importOriginal) => {
    const actual =
      await importOriginal<
        typeof import("@/components/setup/platform/platform-detail")
      >();
    return {
      ...actual,
      PlatformDetailLive: (
        props: Parameters<typeof actual.PlatformDetailLive>[0],
      ) => {
        seen.imagesTable = props.imagesTable;
        return <actual.PlatformDetailLive {...props} />;
      },
    };
  },
);

afterEach(() => {
  cleanup();
  failing.kind = "";
  failing.bundled = false;
});

const params = (name: string) => ({ params: Promise.resolve({ name }) });
const show = async (name: string) =>
  render(await PlatformConfigPage(params(name)), {
    wrapper: GitPushesProvider,
  });

describe("PlatformConfigPage", () => {
  it("renders the detail of a config that exists", async () => {
    await show("basic-lab");
    expect(
      screen.getByRole("heading", { level: 1, name: "basic-lab" }),
    ).toBeTruthy();
    expect(
      screen.getByRole("button", { name: "Review the delete" }),
    ).toBeTruthy();
    expect(seen.imagesTable).toEqual({
      schemaVersion: 1,
      minors: { "8.9": {} },
    });
    expect(dynamic).toBe("force-dynamic");
  });

  it("answers not found for a config that does not exist", async () => {
    await expect(PlatformConfigPage(params("gone"))).rejects.toThrow(
      "NEXT_NOT_FOUND",
    );
  });

  it("passes an empty images table when the table cannot be read", async () => {
    failing.bundled = true;
    await show("basic-lab");
    expect(seen.imagesTable).toEqual({ schemaVersion: 1, minors: {} });
  });

  it("offers no delete and names the kind whose server read failed until its watch answers", async () => {
    failing.kind = "Environment";
    await show("basic-lab");
    expect(
      screen.getByText(
        "Delete is possible when the fleet manager can read Environment.",
      ),
    ).toBeTruthy();
    expect(
      screen.queryByRole("button", { name: "Review the delete" }),
    ).toBeNull();
    expect(
      screen.getByText(
        "The users are not known while the fleet manager cannot read Environment.",
      ),
    ).toBeTruthy();
  });
});
