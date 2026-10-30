// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import { type ReactNode, useEffect } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useGitSync, useResource } from "@/lib/k8s/hooks";
import type { WriteResult } from "@/lib/write/types";
import { GitBanner } from "./git-banner";
import { GitPushesProvider, useGitPushes } from "./git-pushes";

import { FakeEventSource } from "../../../test/fakes/event-source";

vi.mock("@/lib/k8s/hooks", async (importOriginal) => {
  const hooks = await importOriginal<typeof import("@/lib/k8s/hooks")>();
  return { ...hooks, useGitSync: vi.fn() };
});

const result: WriteResult = {
  ok: true,
  mode: "git",
  commit: "c0ffee",
  revisions: {
    "CamundaClusterClaim/team-a/orders": "abc",
    "Environment//prod": "def",
  },
  paths: {
    "CamundaClusterClaim/team-a/orders":
      "fleet/team-a/camundaclusterclaims/orders.yaml",
    "Environment//prod": "fleet/_cluster/environments/prod.yaml",
  },
  deleted: {},
};

// Pushed records result once, as a page does after runChange.
function Pushed({ result }: { result?: WriteResult }) {
  const { record } = useGitPushes();
  useEffect(() => {
    if (result) record(result, new Date().toISOString());
  }, [record, result]);
  return null;
}

const show = (pushed?: WriteResult) =>
  render(
    <GitPushesProvider>
      <Pushed result={pushed} />
      <GitBanner />
    </GitPushesProvider>,
  );

beforeEach(() => {
  vi.mocked(useGitSync).mockReturnValue("waiting");
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("GitBanner", () => {
  it("shows Waiting for GitOps to apply with one line for each pending push", () => {
    show(result);

    expect(screen.getByText("Waiting for GitOps to apply")).toBeTruthy();
    expect(
      screen.getByText("fleet/team-a/camundaclusterclaims/orders.yaml"),
    ).toBeTruthy();
    expect(
      screen.getByText("fleet/_cluster/environments/prod.yaml"),
    ).toBeTruthy();
    expect(useGitSync).toHaveBeenCalledWith(
      { kind: "CamundaClusterClaim", namespace: "team-a", name: "orders" },
      "abc",
    );
  });

  it("renders nothing when nothing waits", () => {
    const { container } = show();
    expect(container.innerHTML).toBe("");
  });

  it("a push that turns applied leaves the banner", async () => {
    vi.mocked(useGitSync).mockReturnValue("applied");

    const { container } = show(result);
    await act(async () => {});

    expect(screen.queryByText("Waiting for GitOps to apply")).toBeNull();
    expect(container.innerHTML).toBe("");
  });

  it("never names a GitOps tool", () => {
    const { container } = show(result);
    expect(container.textContent).not.toMatch(/argo|flux/i);
  });

  it("does not settle a save that equals an old cached copy until fresh data arrives", async () => {
    const real =
      await vi.importActual<typeof import("@/lib/k8s/hooks")>(
        "@/lib/k8s/hooks",
      );
    vi.mocked(useGitSync).mockImplementation(real.useGitSync);
    FakeEventSource.instances = [];
    vi.stubGlobal("EventSource", FakeEventSource);
    const ref = { kind: "Environment" as const, name: "prod" };
    const prod = {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "Environment",
      metadata: {
        name: "prod",
        annotations: { "fleet.camunda.io/revision": "def" },
      },
    };
    const saved: WriteResult = {
      ok: true,
      mode: "git",
      commit: "c0ffee",
      revisions: { "Environment//prod": "def" },
      paths: { "Environment//prod": "fleet/_cluster/environments/prod.yaml" },
      deleted: {},
    };
    function Viewer() {
      useResource("Environment", ref);
      return null;
    }
    const client = new QueryClient();
    const page = (children: ReactNode) => (
      <QueryClientProvider client={client}>
        <GitPushesProvider>{children}</GitPushesProvider>
      </QueryClientProvider>
    );
    const { rerender } = render(page(<Viewer />));
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1));
    const old = FakeEventSource.instances[0];
    old.emit("snapshot", {
      watch: old.ids()[0],
      kind: "Environment",
      objects: [prod],
    });

    rerender(page(null));
    await act(async () => {});
    rerender(
      page(
        <>
          <Pushed result={saved} />
          <GitBanner />
        </>,
      ),
    );
    await act(async () => {});
    expect(screen.getByText("Waiting for GitOps to apply")).toBeTruthy();
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(2));

    const live = FakeEventSource.instances[1];
    live.emit("snapshot", {
      watch: live.ids()[0],
      kind: "Environment",
      objects: [prod],
    });
    await waitFor(() =>
      expect(screen.queryByText("Waiting for GitOps to apply")).toBeNull(),
    );
  });
});
