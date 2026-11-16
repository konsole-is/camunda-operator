// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GitPushesProvider, useGitPushes } from "@/components/fleet/git-pushes";
import type { Resource } from "@/lib/k8s/kinds";
import { STALE_CLAIM_MESSAGE } from "@/lib/session";
import type { Change, WriteResult } from "@/lib/write/types";
import { PlaneActions } from "./plane-actions";

vi.mock("@/lib/k8s/hooks", () => ({
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
}));

afterEach(cleanup);

const now = "2026-11-15T12:00:00Z";

function claimOf(
  spec: Record<string, unknown> = {},
  resourceVersion = "7",
): Resource<"CamundaManagementClusterClaim"> {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: { namespace: "platform", name: "plane-a", resourceVersion },
    spec: { environments: ["platform"], ...spec },
  } as unknown as Resource<"CamundaManagementClusterClaim">;
}

// Pusher records one Git push, as the save of another flow of the tab does.
function Pusher({ result }: { result: WriteResult }) {
  const { record } = useGitPushes();
  useEffect(() => record(result, now), [record, result]);
  return null;
}

function actions(
  props: Partial<Parameters<typeof PlaneActions>[0]> = {},
  pushed?: WriteResult,
) {
  const apply = vi.fn(async (_change: Change): Promise<WriteResult> => ({
    ok: true,
    mode: "direct",
  }));
  const tree = (over: Partial<Parameters<typeof PlaneActions>[0]>) => (
    <QueryClientProvider client={new QueryClient()}>
      <GitPushesProvider>
        {pushed && <Pusher result={pushed} />}
        <PlaneActions
          claim={claimOf()}
          served={2}
          apply={apply}
          deleteHref="/fleet/planes/platform/plane-a/delete"
          now={now}
          {...props}
          {...over}
        />
      </GitPushesProvider>
    </QueryClientProvider>
  );
  const view = render(tree({}));
  return {
    apply,
    rerender: (over: Partial<Parameters<typeof PlaneActions>[0]>) =>
      view.rerender(tree(over)),
  };
}

const button = (name: string) => screen.getByRole("button", { name });

describe("PlaneActions", () => {
  it("Suspend asks first and the confirm says that sign-in stops for every served cluster", async () => {
    const { apply } = actions();
    await userEvent.click(button("Suspend"));
    const confirm = screen.getByRole("group", { name: "Suspend" });
    expect(confirm.textContent).toContain(
      "Sign-in to Console, Web Modeler, and Optimize stops for every cluster that this plane serves (2).",
    );
    expect(confirm.textContent).toContain("The served clusters keep running");
    expect(apply).not.toHaveBeenCalled();
  });

  it("the confirm writes spec.suspend true through apply, with the resourceVersion that the page read", async () => {
    const { apply } = actions();
    await userEvent.click(button("Suspend"));
    await userEvent.click(button("Suspend the management plane"));
    expect(apply).toHaveBeenCalledTimes(1);
    const change = apply.mock.calls[0][0];
    expect(change.apply[0]).toMatchObject({ spec: { suspend: true } });
    expect(change.preconditions).toEqual({
      "CamundaManagementClusterClaim/platform/plane-a": "7",
    });
    expect(
      await screen.findByText("Suspended the management plane plane-a."),
    ).toBeTruthy();
  });

  it("Cancel writes nothing", async () => {
    const { apply } = actions();
    await userEvent.click(button("Suspend"));
    await userEvent.click(button("Cancel"));
    expect(apply).not.toHaveBeenCalled();
    expect(button("Suspend")).toBeTruthy();
  });

  it("a suspended claim offers Resume, and Resume writes spec.suspend false", async () => {
    const { apply } = actions({ claim: claimOf({ suspend: true }) });
    expect(screen.queryByRole("button", { name: "Suspend" })).toBeNull();
    await userEvent.click(button("Resume"));
    expect(apply.mock.calls[0][0].apply[0]).toMatchObject({
      spec: { suspend: false },
    });
  });

  it("sends no second write until the claim shows a newer version", async () => {
    const { apply, rerender } = actions({ claim: claimOf({ suspend: true }) });
    await userEvent.click(button("Resume"));
    expect(button("Resume")).toHaveProperty("disabled", true);
    await userEvent.click(button("Resume"));
    expect(apply).toHaveBeenCalledTimes(1);
    rerender({ claim: claimOf({ suspend: false }, "8") });
    expect(button("Suspend")).toHaveProperty("disabled", false);
  });

  it("shows the refusal of a stale write", async () => {
    const { apply } = actions();
    apply.mockResolvedValueOnce({
      ok: false,
      errors: [{ code: "Stale", message: STALE_CLAIM_MESSAGE }],
    });
    await userEvent.click(button("Suspend"));
    await userEvent.click(button("Suspend the management plane"));
    expect((await screen.findByRole("alert")).textContent).toContain(
      STALE_CLAIM_MESSAGE,
    );
  });

  it("Delete links to the delete page", () => {
    actions();
    expect(
      screen.getByRole("link", { name: "Delete" }).getAttribute("href"),
    ).toBe("/fleet/planes/platform/plane-a/delete");
  });

  it("turns Suspend and Delete off, names the kinds, and closes an open confirm while a kind is unread", async () => {
    const { apply, rerender } = actions();
    await userEvent.click(button("Suspend"));
    rerender({ unread: ["CamundaManagementCluster"] });
    expect(screen.queryByRole("group", { name: "Suspend" })).toBeNull();
    expect(button("Suspend")).toHaveProperty("disabled", true);
    expect(button("Delete")).toHaveProperty("disabled", true);
    expect(screen.queryByRole("link", { name: "Delete" })).toBeNull();
    expect(
      screen.getByText(
        "Suspend and Delete are possible when the fleet manager can read CamundaManagementCluster.",
      ),
    ).toBeTruthy();
    expect(apply).not.toHaveBeenCalled();
  });

  it("in Git mode shows the waiting line of a pushed claim and keeps the actions off", async () => {
    const key = "CamundaManagementClusterClaim/platform/plane-a";
    await act(async () => {
      actions(
        {},
        {
          ok: true,
          mode: "git",
          commit: "a".repeat(40),
          revisions: { [key]: "r1" },
          paths: { [key]: "planes/plane-a.yaml" },
          deleted: {},
        },
      );
    });
    expect(screen.getByRole("status").textContent).toContain(
      "1 change waiting for GitOps",
    );
    expect(button("Suspend")).toHaveProperty("disabled", true);
  });
});
