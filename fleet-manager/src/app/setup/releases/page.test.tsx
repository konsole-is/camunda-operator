// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import { list } from "@/lib/k8s/read";
import { fakeWriter } from "../../../../test/fakes/writer";
import ReleasesPage, { dynamic } from "./page";

vi.mock("@/lib/write/writer", () => import("../../../../test/fakes/writer"));
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
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
}));

const seen = vi.hoisted(() => ({ mode: undefined as unknown }));
vi.mock("@/components/setup/releases/release-list", async (importOriginal) => {
  const actual =
    await importOriginal<
      typeof import("@/components/setup/releases/release-list")
    >();
  return {
    ...actual,
    ReleaseListLive: (props: Parameters<typeof actual.ReleaseListLive>[0]) => {
      seen.mode = props.mode;
      return <actual.ReleaseListLive {...props} />;
    },
  };
});

const show = async () =>
  render(await ReleasesPage(), { wrapper: GitPushesProvider });

afterEach(() => {
  cleanup();
  vi.mocked(list).mockClear();
  failing.kind = "";
  fakeWriter.reset();
});

describe("ReleasesPage", () => {
  it("reads each kind of the sources from the cache", async () => {
    await show();
    expect(
      vi
        .mocked(list)
        .mock.calls.map(([kind]) => kind)
        .sort(),
    ).toEqual([
      "CamundaCluster",
      "CamundaClusterClaim",
      "CamundaRelease",
      "DatabaseServer",
      "ElasticsearchCluster",
      "Environment",
      "Namespace",
    ]);
    expect(dynamic).toBe("force-dynamic");
  });

  it("passes the write mode", async () => {
    fakeWriter.reset("git");
    await show();
    expect(seen.mode).toBe("git");
  });

  it("renders the list when a read fails", async () => {
    failing.kind = "CamundaRelease";
    await show();
    expect(
      screen.getByRole("heading", { level: 1, name: "Releases" }),
    ).toBeTruthy();
  });
});
