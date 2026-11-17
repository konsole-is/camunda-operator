// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { cloneElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import { list } from "@/lib/k8s/read";
import { fakeWriter } from "../../../../test/fakes/writer";
import ReleasesPage, { dynamic } from "./page";

vi.mock("@/lib/write/writer", () => import("../../../../test/fakes/writer"));
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
  watch.answered = false;
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

  it("shows the error of a failed server read until the watch of its kind answers", async () => {
    failing.kind = "Environment";
    const page = await ReleasesPage();
    const { rerender } = render(page, { wrapper: GitPushesProvider });
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
});
