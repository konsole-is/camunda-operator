// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { GitSyncRef } from "@/lib/k8s/hooks";
import { watchId } from "@/lib/k8s/shared-watch";
import type { WriteResult } from "@/lib/write/types";
import { FakeEventSource } from "../../../test/fakes/event-source";
import { GitPushesProvider } from "./git-pushes";
import { Toast, toastResultOf, useWriteToast } from "./toast";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("Toast", () => {
  it("shows the Git waiting state with the short commit and no tool name", () => {
    render(
      <Toast result={{ kind: "waiting", commit: "4f1c3a7e9b2d8c6f0a5e" }} />,
    );
    const status = screen.getByRole("status");
    expect(status.textContent).toContain("Waiting for GitOps to apply");
    expect(status.textContent).toContain("4f1c3a7");
    expect(status.textContent).not.toMatch(/argo|flux/i);
  });

  it("announces an error as an alert", () => {
    render(
      <Toast
        result={{
          kind: "error",
          message: "The claim changed since you opened it.",
        }}
      />,
    );
    expect(screen.getByRole("alert").textContent).toContain(
      "The claim changed since you opened it.",
    );
  });

  it("shows a saved result as a status", () => {
    render(
      <Toast result={{ kind: "saved", message: "Saved the claim orders" }} />,
    );
    expect(screen.getByRole("status").textContent).toContain(
      "Saved the claim orders",
    );
  });
});

describe("toastResultOf", () => {
  it("maps a direct save to saved with the message of the page", () => {
    expect(
      toastResultOf({ ok: true, mode: "direct" }, "Saved the claim orders"),
    ).toEqual({
      kind: "saved",
      message: "Saved the claim orders",
    });
  });

  it("maps a Git push to waiting with the commit", () => {
    expect(
      toastResultOf(
        {
          ok: true,
          mode: "git",
          commit: "4f1c3a7e9b",
          revisions: {},
          paths: {},
          deleted: {},
        },
        "x",
      ),
    ).toEqual({ kind: "waiting", commit: "4f1c3a7e9b" });
  });

  it("maps a refusal to an error with the API messages", () => {
    expect(
      toastResultOf(
        {
          ok: false,
          errors: [
            {
              code: "Stale",
              message:
                "The CamundaClusterClaim orders changed since you opened it.",
            },
            { code: "Invalid", message: "spec.presetRef: Required value" },
          ],
        },
        "Saved",
      ),
    ).toEqual({
      kind: "error",
      message:
        "The CamundaClusterClaim orders changed since you opened it. spec.presetRef: Required value",
    });
  });
});

describe("useWriteToast", () => {
  const claimKey = "CamundaClusterClaim/team-a/orders";
  const claimRef = {
    kind: "CamundaClusterClaim",
    namespace: "team-a",
    name: "orders",
  } as const;
  const envRef = { kind: "Environment", name: "prod" } as const;
  const claim = {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { namespace: "team-a", name: "orders" },
  };
  const env = (revision?: string) => ({
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: {
      name: "prod",
      ...(revision
        ? { annotations: { "fleet.camunda.io/revision": revision } }
        : {}),
    },
  });
  const deleteResult: WriteResult = {
    ok: true,
    mode: "git",
    commit: "d00d",
    revisions: {},
    paths: {},
    deleted: { [claimKey]: "fleet/team-a/camundaclusterclaims/orders.yaml" },
  };
  const envResult = (revision: string): WriteResult => ({
    ok: true,
    mode: "git",
    commit: `c-${revision}`,
    revisions: { "Environment//prod": revision },
    paths: { "Environment//prod": "fleet/_cluster/environments/prod.yaml" },
    deleted: {},
  });

  function Page({
    name,
    result,
    message,
  }: {
    name: string;
    result: WriteResult;
    message: string;
  }) {
    const { toast, show, dismiss } = useWriteToast();
    return (
      <section aria-label={name}>
        <button onClick={() => show(result, message)}>Save {name}</button>
        {toast && <Toast result={toast} onDismiss={dismiss} />}
      </section>
    );
  }

  const page = (children: ReactNode) => {
    FakeEventSource.instances = [];
    vi.stubGlobal("EventSource", FakeEventSource);
    return render(
      <QueryClientProvider client={new QueryClient()}>
        <GitPushesProvider>{children}</GitPushesProvider>
      </QueryClientProvider>,
    );
  };

  // connection waits for the last connection that watches each ref.
  async function connection(...refs: GitSyncRef[]): Promise<FakeEventSource> {
    const ids = refs.map((ref) => watchId(ref));
    await waitFor(() =>
      expect(FakeEventSource.instances.at(-1)?.ids()).toEqual(
        expect.arrayContaining(ids),
      ),
    );
    return FakeEventSource.instances.at(-1)!;
  }

  const statusOf = (name: string) =>
    screen.getByRole("region", { name }).querySelector("[role=status]")
      ?.textContent;

  it("a direct save shows saved at once", () => {
    page(
      <Page
        name="a"
        result={{ ok: true, mode: "direct" }}
        message="Saved the claim orders"
      />,
    );

    fireEvent.click(screen.getByText("Save a"));

    expect(statusOf("a")).toContain("Saved the claim orders");
  });

  it("the toast of a delete-only change in Git mode turns from waiting to applied", async () => {
    page(
      <Page
        name="a"
        result={deleteResult}
        message="Deleted the claim orders"
      />,
    );

    fireEvent.click(screen.getByText("Save a"));
    const s = await connection(claimRef);
    const watch = watchId(claimRef);
    s.emit("snapshot", {
      watch,
      kind: "CamundaClusterClaim",
      objects: [claim],
    });
    await act(async () => {});
    expect(statusOf("a")).toContain("Waiting for GitOps to apply");

    s.emit("delta", {
      watch,
      type: "DELETED",
      kind: "CamundaClusterClaim",
      object: claim,
    });

    await waitFor(() =>
      expect(statusOf("a")).toContain("Deleted the claim orders"),
    );
    expect(statusOf("a")).not.toContain("Waiting");
  });

  it("a change that writes one object and deletes another settles only when both are done", async () => {
    const mixed: WriteResult = {
      ok: true,
      mode: "git",
      commit: "c-mixed",
      revisions: { "Environment//prod": "def" },
      paths: { "Environment//prod": "fleet/_cluster/environments/prod.yaml" },
      deleted: { [claimKey]: "fleet/team-a/camundaclusterclaims/orders.yaml" },
    };
    page(<Page name="a" result={mixed} message="Replaced orders with prod" />);

    fireEvent.click(screen.getByText("Save a"));
    const s = await connection(envRef, claimRef);
    s.emit("snapshot", {
      watch: watchId(envRef),
      kind: "Environment",
      objects: [env()],
    });
    s.emit("snapshot", {
      watch: watchId(claimRef),
      kind: "CamundaClusterClaim",
      objects: [claim],
    });
    s.emit("delta", {
      watch: watchId(claimRef),
      type: "DELETED",
      kind: "CamundaClusterClaim",
      object: claim,
    });
    await act(async () => {});
    expect(statusOf("a")).toContain("Waiting for GitOps to apply");

    s.emit("delta", {
      watch: watchId(envRef),
      type: "MODIFIED",
      kind: "Environment",
      object: env("def"),
    });

    await waitFor(() =>
      expect(statusOf("a")).toContain("Replaced orders with prod"),
    );
  });

  it("a later save of the same object keeps the toast waiting", async () => {
    page(
      <>
        <Page name="a" result={envResult("def")} message="Saved def" />
        <Page name="b" result={envResult("ghi")} message="Saved ghi" />
      </>,
    );

    fireEvent.click(screen.getByText("Save a"));
    fireEvent.click(screen.getByText("Save b"));
    const s = await connection(envRef);
    const watch = watchId(envRef);
    s.emit("snapshot", { watch, kind: "Environment", objects: [env("def")] });
    await act(async () => {});
    expect(statusOf("a")).toContain("Waiting for GitOps to apply");
    expect(statusOf("b")).toContain("Waiting for GitOps to apply");

    s.emit("delta", {
      watch,
      type: "MODIFIED",
      kind: "Environment",
      object: env("ghi"),
    });

    await waitFor(() => expect(statusOf("a")).toContain("Saved def"));
    expect(statusOf("b")).toContain("Saved ghi");
  });

  it("dismiss clears the toast", () => {
    page(
      <Page
        name="a"
        result={{ ok: true, mode: "direct" }}
        message="Saved the claim orders"
      />,
    );
    fireEvent.click(screen.getByText("Save a"));

    fireEvent.click(screen.getByLabelText("Dismiss"));

    expect(statusOf("a")).toBeUndefined();
  });
});
