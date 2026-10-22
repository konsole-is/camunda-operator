// @vitest-environment jsdom
import { act, cleanup, render, screen } from "@testing-library/react";
import { useEffect } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useGitSync } from "@/lib/k8s/hooks";
import type { WriteResult } from "@/lib/write/types";
import { GitBanner } from "./git-banner";
import { GitPushesProvider, useGitPushes } from "./git-pushes";

vi.mock("@/lib/k8s/hooks", () => ({ useGitSync: vi.fn() }));

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
afterEach(cleanup);

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
});
