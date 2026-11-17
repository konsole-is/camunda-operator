// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CHANGES_OFF } from "@/components/fleet/claim-edit";
import { GitPushesProvider, useGitPushes } from "@/components/fleet/git-pushes";
import type { Resource } from "@/lib/k8s/kinds";
import { STALE_CLAIM_MESSAGE } from "@/lib/session";
import type { Change, Preview, WriteResult } from "@/lib/write/types";
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
  metadata: Record<string, unknown> = {},
): Resource<"CamundaManagementClusterClaim"> {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: {
      namespace: "platform",
      name: "plane-a",
      resourceVersion,
      ...metadata,
    },
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
  const apply = vi.fn<(change: Change) => Promise<WriteResult>>(async () => ({
    ok: true,
    mode: "direct",
  }));
  const preview = vi.fn<(change: Change) => Promise<Preview>>(async () => ({
    objects: [],
    errors: [],
  }));
  const tree = (over: Partial<Parameters<typeof PlaneActions>[0]>) => (
    <QueryClientProvider client={new QueryClient()}>
      <GitPushesProvider>
        {pushed && <Pusher result={pushed} />}
        <PlaneActions
          claim={claimOf()}
          served={2}
          apply={apply}
          preview={preview}
          deleteHref="/fleet/planes/platform/plane-a/delete"
          writeMode="direct"
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
    preview,
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

  it("in the keycloak mode the confirm says that the served OIDC clusters cannot sign in or get tokens", async () => {
    actions({ ownKeycloak: true });
    await userEvent.click(button("Suspend"));
    expect(
      screen.getByRole("group", { name: "Suspend" }).textContent,
    ).toContain(
      "Each cluster that uses the Keycloak of this plane as its OIDC issuer loses sign-in: its users and API clients cannot sign in or get tokens until the plane resumes.",
    );
  });

  it("states nothing about a Keycloak of the plane without one", async () => {
    actions({ ownKeycloak: false });
    await userEvent.click(button("Suspend"));
    expect(
      screen.getByRole("group", { name: "Suspend" }).textContent,
    ).not.toContain("Keycloak");
  });

  it("in the keycloak mode warns also while no cluster is attached", async () => {
    actions({ ownKeycloak: true, served: 0 });
    await userEvent.click(button("Suspend"));
    expect(
      screen.getByRole("group", { name: "Suspend" }).textContent,
    ).toContain(
      "Each cluster that uses the Keycloak of this plane as its OIDC issuer loses sign-in: its users and API clients cannot sign in or get tokens until the plane resumes.",
    );
  });

  it("offers no Suspend and no Resume while the claim is being deleted, and says why", () => {
    const deleting = (spec: Record<string, unknown>) => {
      const claim = claimOf(spec);
      claim.metadata!.deletionTimestamp = now;
      return claim;
    };
    const { rerender } = actions({ claim: deleting({}) });
    expect(button("Suspend")).toHaveProperty("disabled", true);
    expect(
      screen.getByText(
        "Suspend and Resume are off while the claim is being deleted.",
      ),
    ).toBeTruthy();
    rerender({ claim: deleting({ suspend: true }) });
    expect(button("Resume")).toHaveProperty("disabled", true);
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

  it("offers no Suspend while the write mode is not known, and says why", async () => {
    const { apply, rerender } = actions();
    await userEvent.click(button("Suspend"));
    rerender({ writeMode: undefined });
    expect(screen.queryByRole("group", { name: "Suspend" })).toBeNull();
    expect(button("Suspend")).toHaveProperty("disabled", true);
    expect(screen.getByText(CHANGES_OFF)).toBeTruthy();
    rerender({ writeMode: undefined, claim: claimOf({ suspend: true }) });
    expect(button("Resume")).toHaveProperty("disabled", true);
    expect(apply).not.toHaveBeenCalled();
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

  it.each([
    ["Suspend", {}],
    ["Resume", { suspend: true }],
  ])(
    "in Git mode keeps %s and Delete off while a pushed delete of the claim waits for GitOps",
    async (label, spec) => {
      const key = "CamundaManagementClusterClaim/platform/plane-a";
      await act(async () => {
        actions(
          { claim: claimOf(spec) },
          {
            ok: true,
            mode: "git",
            commit: "a".repeat(40),
            revisions: {},
            paths: {},
            deleted: { [key]: "planes/plane-a.yaml" },
          },
        );
      });
      expect(screen.getByRole("status").textContent).toContain(
        "1 delete waiting for GitOps",
      );
      expect(button(label)).toHaveProperty("disabled", true);
      expect(button("Delete")).toHaveProperty("disabled", true);
    },
  );

  it("shows a refusal of the save after a reload as an alert", async () => {
    const { apply } = actions();
    const message =
      "The repository holds a delete of the CamundaManagementClusterClaim plane-a that GitOps has not applied yet.";
    apply.mockResolvedValueOnce({
      ok: false,
      errors: [{ code: "Conflict", message }],
    });
    await userEvent.click(button("Suspend"));
    await userEvent.click(button("Suspend the management plane"));
    expect((await screen.findByRole("alert")).textContent).toContain(message);
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

describe("PlaneActions with a field of another tool", () => {
  const ref = {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    namespace: "platform",
    name: "plane-a",
  };
  const key = "CamundaManagementClusterClaim/platform/plane-a";
  const conflict = {
    code: "Conflict" as const,
    message: 'conflict with "kubectl"',
    path: "spec.suspend",
    manager: "kubectl",
    ref,
  };
  const suspendOf = (kind: "other" | "operator", name = "kubectl") => ({
    objects: [],
    errors: [conflict],
    ownership: [
      {
        ref,
        changed: [
          {
            path: "spec.suspend",
            steps: [{ field: "spec" }, { field: "suspend" }],
            managers: [{ name, operation: "Apply", kind, known: true }],
          },
        ],
        cleared: [],
      },
    ],
  });
  const group = () =>
    screen.getByRole("group", { name: "Fields that another tool set" });

  // conflicted makes the first write of apply fail on spec.suspend, as the
  // direct writer does when kubectl set the field.
  function conflicted(props: Partial<Parameters<typeof PlaneActions>[0]> = {}) {
    const view = actions(props);
    view.apply.mockResolvedValueOnce({ ok: false, errors: [conflict] });
    view.preview.mockResolvedValue(suspendOf("other"));
    return view;
  }

  it("Suspend of a plane whose spec.suspend kubectl set asks to take it over, then suspends", async () => {
    const { apply, preview } = conflicted();
    await userEvent.click(button("Suspend"));
    await userEvent.click(button("Suspend the management plane"));
    expect(
      (
        await screen.findByRole("group", {
          name: "Fields that another tool set",
        })
      ).textContent,
    ).toContain("spec.suspend: kubectl set it. The save changes it.");
    expect(preview).toHaveBeenCalledWith(apply.mock.calls[0][0]);
    // The panel names the conflict, so no alert repeats it.
    expect(screen.queryByRole("alert")).toBeNull();
    const confirm = button("Suspend the management plane");
    expect(confirm).toHaveProperty("disabled", true);

    await userEvent.click(
      screen.getByRole("checkbox", { name: "Take over these fields" }),
    );
    expect(confirm).toHaveProperty("disabled", false);
    await userEvent.click(confirm);
    expect(apply).toHaveBeenCalledTimes(2);
    expect(apply.mock.calls[1][0]).toEqual({
      ...apply.mock.calls[0][0],
      takeOver: { [key]: ["spec.suspend"] },
    });
    expect(
      await screen.findByText("Suspended the management plane plane-a."),
    ).toBeTruthy();
    expect(
      screen.queryByRole("group", { name: "Fields that another tool set" }),
    ).toBeNull();
  });

  it("Resume does the same", async () => {
    const { apply } = conflicted({ claim: claimOf({ suspend: true }) });
    await userEvent.click(button("Resume"));
    expect(group().textContent).toContain("spec.suspend: kubectl set it.");
    const confirm = button("Resume the management plane");
    expect(confirm).toHaveProperty("disabled", true);
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Take over these fields" }),
    );
    await userEvent.click(confirm);
    expect(apply.mock.calls[1][0].takeOver).toEqual({
      [key]: ["spec.suspend"],
    });
    expect(apply.mock.calls[1][0].apply[0]).toMatchObject({
      spec: { suspend: false },
    });
  });

  it("the next Suspend asks again", async () => {
    const { apply, rerender } = conflicted({
      claim: claimOf({ suspend: true }),
    });
    await userEvent.click(button("Resume"));
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Take over these fields" }),
    );
    await userEvent.click(button("Resume the management plane"));
    expect(apply).toHaveBeenCalledTimes(2);
    rerender({ claim: claimOf({ suspend: false }, "8") });

    apply.mockResolvedValueOnce({ ok: false, errors: [conflict] });
    await userEvent.click(button("Suspend"));
    await userEvent.click(button("Suspend the management plane"));
    const checkbox = await screen.findByRole("checkbox", {
      name: "Take over these fields",
    });
    expect(checkbox.getAttribute("aria-checked")).toBe("false");
    expect(button("Suspend the management plane")).toHaveProperty(
      "disabled",
      true,
    );
  });

  // ownedBy gives the managedFields of a claim whose spec.suspend manager
  // set with apply, and a status write of the operator when status is set.
  const ownedBy = (manager: string, status = false) => ({
    generation: 3,
    managedFields: [
      {
        manager,
        operation: "Apply",
        fieldsType: "FieldsV1",
        fieldsV1: { "f:spec": { "f:suspend": {} } },
      },
      ...(status
        ? [
            {
              manager: "camunda-operator",
              operation: "Update",
              subresource: "status",
              fieldsType: "FieldsV1",
              fieldsV1: { "f:status": { "f:phase": {} } },
            },
          ]
        : []),
    ],
  });

  // asked opens the take-over of Suspend on a claim at generation 3 whose
  // spec.suspend kubectl set, and makes the choice.
  async function asked() {
    const view = conflicted({ claim: claimOf({}, "7", ownedBy("kubectl")) });
    await userEvent.click(button("Suspend"));
    await userEvent.click(button("Suspend the management plane"));
    await userEvent.click(
      await screen.findByRole("checkbox", { name: "Take over these fields" }),
    );
    return view;
  }

  it("a status write of the operator keeps the take-over open, and the save reads the newer version", async () => {
    const { apply, rerender } = await asked();
    rerender({ claim: claimOf({}, "8", ownedBy("kubectl", true)) });

    expect(group().textContent).toContain("spec.suspend: kubectl set it.");
    expect(screen.queryByText(/The plane changed/)).toBeNull();
    await userEvent.click(button("Suspend the management plane"));
    expect(apply).toHaveBeenCalledTimes(2);
    expect(apply.mock.calls[1][0].preconditions).toEqual({ [key]: "8" });
    expect(apply.mock.calls[1][0].takeOver).toEqual({
      [key]: ["spec.suspend"],
    });
    expect(apply.mock.calls[1][0].apply[0]).toMatchObject({
      metadata: { resourceVersion: "8" },
      spec: { suspend: true },
    });
  });

  it("a change of the spec drops the take-over and says to select Suspend again", async () => {
    const { apply, rerender } = await asked();
    rerender({
      claim: claimOf({}, "8", { ...ownedBy("kubectl"), generation: 4 }),
    });

    expect(
      screen.queryByRole("group", { name: "Fields that another tool set" }),
    ).toBeNull();
    expect(
      screen.getByText("The plane changed. Select Suspend again."),
    ).toBeTruthy();
    expect(apply).toHaveBeenCalledTimes(1);

    apply.mockResolvedValueOnce({ ok: false, errors: [conflict] });
    await userEvent.click(button("Suspend"));
    await userEvent.click(button("Suspend the management plane"));
    const checkbox = await screen.findByRole("checkbox", {
      name: "Take over these fields",
    });
    expect(checkbox.getAttribute("aria-checked")).toBe("false");
  });

  it("a new manager of spec.suspend drops the take-over and says to select Suspend again", async () => {
    const { apply, rerender } = await asked();
    rerender({ claim: claimOf({}, "8", ownedBy("helm")) });

    expect(
      screen.queryByRole("group", { name: "Fields that another tool set" }),
    ).toBeNull();
    expect(
      screen.getByText("The plane changed. Select Suspend again."),
    ).toBeTruthy();
    expect(apply).toHaveBeenCalledTimes(1);
  });

  it("a GitOps marker on the claim drops the take-over, as the review names kubectl another way", async () => {
    const { rerender } = await asked();
    rerender({
      claim: claimOf({}, "8", {
        ...ownedBy("kubectl"),
        labels: { "kustomize.toolkit.fluxcd.io/name": "planes" },
      }),
    });
    expect(
      screen.queryByRole("group", { name: "Fields that another tool set" }),
    ).toBeNull();
    expect(
      screen.getByText("The plane changed. Select Suspend again."),
    ).toBeTruthy();
  });

  it("the old take-over does not come back when the managers come back", async () => {
    const { rerender } = await asked();
    rerender({ claim: claimOf({}, "8", ownedBy("helm")) });
    rerender({ claim: claimOf({}, "9", ownedBy("kubectl")) });
    expect(
      screen.queryByRole("group", { name: "Fields that another tool set" }),
    ).toBeNull();
    expect(
      screen.getByText("The plane changed. Select Suspend again."),
    ).toBeTruthy();
  });

  it("a change of the plane to the asked state says only that the plane changed", async () => {
    const { rerender } = await asked();
    rerender({
      claim: claimOf({ suspend: true }, "8", {
        ...ownedBy("kubectl"),
        generation: 4,
      }),
    });
    expect(screen.getByText("The plane changed.")).toBeTruthy();
  });

  it("Cancel closes the take-over and writes nothing more", async () => {
    const { apply } = conflicted();
    await userEvent.click(button("Suspend"));
    await userEvent.click(button("Suspend the management plane"));
    await screen.findByRole("group", { name: "Fields that another tool set" });
    await userEvent.click(button("Cancel"));
    expect(
      screen.queryByRole("group", { name: "Fields that another tool set" }),
    ).toBeNull();
    expect(button("Suspend")).toBeTruthy();
    expect(apply).toHaveBeenCalledTimes(1);
  });

  it("Cancel of the take-over puts the focus on the Suspend or Resume that replaces it", async () => {
    conflicted();
    await userEvent.click(button("Suspend"));
    await userEvent.click(button("Suspend the management plane"));
    await screen.findByRole("group", { name: "Fields that another tool set" });
    await userEvent.click(button("Cancel"));
    expect(document.activeElement).toBe(button("Suspend"));
    cleanup();

    conflicted({ claim: claimOf({ suspend: true }) });
    await userEvent.click(button("Resume"));
    await screen.findByRole("group", { name: "Fields that another tool set" });
    await userEvent.click(button("Cancel"));
    expect(document.activeElement).toBe(button("Resume"));
  });

  it("names the controller of an operator field and offers no confirm", async () => {
    const { preview } = conflicted();
    preview.mockResolvedValue(
      suspendOf(
        "operator",
        "CamundaManagementClusterClaim/plane/0b5c7a2e-1f0e-4f43-9d0a-6a0c2f1e9b11",
      ),
    );
    await userEvent.click(button("Suspend"));
    await userEvent.click(button("Suspend the management plane"));
    expect(
      (
        await screen.findByRole("group", {
          name: "Fields that another tool set",
        })
      ).textContent,
    ).toContain("of the operator sets it.");
    expect(
      screen.queryByRole("checkbox", { name: "Take over these fields" }),
    ).toBeNull();
    expect(button("Suspend the management plane")).toHaveProperty(
      "disabled",
      true,
    );
  });

  it("shows the conflict as before when the preview names no field", async () => {
    const { preview } = conflicted();
    preview.mockResolvedValue({ objects: [], errors: [conflict] });
    await userEvent.click(button("Suspend"));
    await userEvent.click(button("Suspend the management plane"));
    expect((await screen.findByRole("alert")).textContent).toContain(
      'conflict with "kubectl"',
    );
    expect(
      screen.queryByRole("group", { name: "Fields that another tool set" }),
    ).toBeNull();
  });

  it("in Git mode shows no choice", async () => {
    const { preview } = conflicted({ writeMode: "git" });
    await userEvent.click(button("Suspend"));
    await userEvent.click(button("Suspend the management plane"));
    expect((await screen.findByRole("alert")).textContent).toContain(
      'conflict with "kubectl"',
    );
    expect(preview).not.toHaveBeenCalled();
  });
});
