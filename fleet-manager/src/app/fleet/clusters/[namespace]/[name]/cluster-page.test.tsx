// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen, within } from "@testing-library/react";
import { useEffect } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ClusterPageData } from "@/components/cluster/cluster-view";
import { GitPushesProvider, useGitPushes } from "@/components/fleet/git-pushes";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import type { RefKey, WriteMode, WriteResult } from "@/lib/write/types";
import { ClusterPage } from "./cluster-page";

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(window.location.search),
}));
vi.mock("@/lib/observe/use-observe", () => ({
  usePodUsage: () => undefined,
  usePartitions: () => undefined,
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (_kind: string, _opts: unknown, initial: unknown[]) => ({
    items: initial,
  }),
  useResource: (_kind: string, _ref: unknown, initial?: unknown) => ({
    item: initial,
  }),
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
}));
vi.mock(
  "@/lib/write/writer",
  () => import("../../../../../../test/fakes/writer"),
);

const now = "2026-11-10T12:00:00Z";

beforeEach(() => {
  window.history.replaceState(null, "", "/fleet/clusters/team-a/orders");
});
afterEach(cleanup);

// data is the page of the managed cluster orders with its claim. managed
// false leaves out the claim label.
function data(managed = true): ClusterPageData {
  return {
    cluster: {
      kind: "CamundaCluster",
      metadata: {
        namespace: "team-a",
        name: "orders",
        labels: managed ? { [CLAIM_LABEL]: "orders" } : {},
      },
      spec: {},
    } as unknown as Resource<"CamundaCluster">,
    environments: [],
    claims: [
      {
        kind: "CamundaClusterClaim",
        metadata: {
          namespace: "team-a",
          name: "orders",
          resourceVersion: "41",
        },
        spec: {},
      } as unknown as Resource<"CamundaClusterClaim">,
    ],
    presets: [],
    releases: [],
    storage: [],
    accessRequests: [],
    requests: [],
    pods: [],
    podSelector: "camunda.io/cluster=orders",
    backups: { elasticsearch: [], rdbms: [] },
    restores: { elasticsearch: [], rdbms: [], pointInTime: [] },
    backupStorage: {
      objectStorage: [],
      databaseConfigs: [],
      databaseServerConfigs: [],
      databaseServers: [],
      databaseServerPresets: [],
      schedules: [],
    },
    optimizes: [],
    sections: [],
    registrations: [],
    platformConfigs: [],
    managementAuthConfigs: [],
    fieldDocs: {},
  };
}

function gitResultFor(key: RefKey): WriteResult {
  return {
    ok: true,
    mode: "git",
    commit: "a".repeat(40),
    revisions: { [key]: "bbb" },
    paths: { [key]: `${key.toLowerCase()}.yaml` },
    deleted: {},
  };
}

// Pusher records one Git push, as a save of another flow of the tab does.
function Pusher({ result }: { result: WriteResult }) {
  const { record } = useGitPushes();
  useEffect(() => record(result, now), [record, result]);
  return null;
}

function page(
  writeMode: WriteMode | undefined,
  pushed?: WriteResult,
  initial = data(),
) {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <GitPushesProvider>
        {pushed && <Pusher result={pushed} />}
        <ClusterPage initial={initial} now={now} writeMode={writeMode} />
      </GitPushesProvider>
    </QueryClientProvider>,
  );
}

const waitingLine = () =>
  screen
    .queryAllByRole("status")
    .map((el) => el.textContent)
    .find((text) => text?.includes("waiting for GitOps"));

describe("ClusterPage", () => {
  it("shows the waiting line of the pushed claim in Git mode", async () => {
    await act(async () => {
      page("git", gitResultFor("CamundaClusterClaim/team-a/orders"));
    });
    expect(waitingLine()).toContain("1 change waiting for GitOps");
    expect(waitingLine()).toContain("camundaclusterclaim/team-a/orders.yaml");
  });

  it("shows no waiting line for a push of another object", async () => {
    await act(async () => {
      page("git", gitResultFor("CamundaClusterClaim/team-a/payments"));
    });
    expect(waitingLine()).toBeUndefined();
  });

  it("mounts the edit host for a managed cluster", () => {
    const { unmount } = page("direct");
    expect(screen.queryByText(/^Changes are off/)).toBeNull();
    unmount();
    page(undefined);
    expect(
      screen.getByText(
        "Changes are off: the fleet manager could not read its settings.",
      ),
    ).toBeTruthy();
  });

  it("a managed cluster offers Change topology in the Topology section", () => {
    page("direct");
    const topology = screen.getByRole("region", { name: "Topology" });
    expect(
      within(topology).getByRole("button", { name: "Change topology" }),
    ).toBeTruthy();
  });

  it("an unmanaged cluster offers no Change topology", () => {
    page("direct", undefined, data(false));
    expect(screen.getByRole("region", { name: "Topology" })).toBeTruthy();
    expect(
      screen.queryByRole("button", { name: "Change topology" }),
    ).toBeNull();
  });
});
