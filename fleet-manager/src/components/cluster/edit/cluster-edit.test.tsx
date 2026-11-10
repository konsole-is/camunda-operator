// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  applyClaimChange,
  previewClaimChange,
} from "@/app/fleet/clusters/[namespace]/[name]/actions";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import { pathFrom, type ClusterField, type Step } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import type { KubeObject, WriteResult } from "@/lib/write/types";
import { FakeEventSource } from "../../../../test/fakes/event-source";
import {
  claim,
  cluster,
  slotContext,
} from "../../../../test/fakes/slot-context";
import { fakeWriter } from "../../../../test/fakes/writer";
import type { ClusterSlotContext } from "../slots";
import {
  type ClaimActions,
  ClusterEditProvider,
  useClusterEdit,
} from "./cluster-edit";

vi.mock("@/lib/write/writer", () => import("../../../../test/fakes/writer"));

const suspend: Step[] = [{ field: "spec" }, { field: "suspend" }];
const replicas: Step[] = [
  { field: "spec" },
  { field: "cluster" },
  { field: "zeebe" },
  { field: "replicas" },
];
const releaseRef: Step[] = [{ field: "spec" }, { field: "releaseRef" }];
const presetRef: Step[] = [{ field: "spec" }, { field: "presetRef" }];
const PRECONDITION = "CamundaClusterClaim/team-a/orders";

// Probe is an editor: it changes the claim only through useClusterEdit.
function Probe() {
  const edit = useClusterEdit();
  const [refusal, setRefusal] = useState<string>();
  return (
    <div>
      <p>Session {edit.session ? "open" : "none"}</p>
      <p>Pending {edit.pending}</p>
      <p>Refusal {refusal ?? "none"}</p>
      <ul aria-label="Field errors">
        {edit.errors.map((e, i) => (
          <li key={i}>
            {e.path}: {e.message}
          </li>
        ))}
      </ul>
      <button onClick={() => setRefusal(edit.set(suspend, true)?.reason)}>
        Suspend
      </button>
      <button onClick={() => setRefusal(edit.set(replicas, 5)?.reason)}>
        Scale
      </button>
      <button onClick={() => edit.set(releaseRef, "missing")}>
        Missing release
      </button>
      <button onClick={() => edit.set(presetRef, "large")}>Large preset</button>
      <button
        onClick={() => {
          edit.set(suspend, true);
          edit.set(replicas, 5);
          edit.review();
        }}
      >
        Suspend and review
      </button>
      <button
        onClick={() =>
          edit.show(
            { ok: true, mode: "direct" },
            "Backup orders-manual is created.",
          )
        }
      >
        Back up
      </button>
    </div>
  );
}

const serverActions: ClaimActions = {
  preview: previewClaimChange,
  apply: applyClaimChange,
};

function Page({
  context,
  actions,
}: {
  context: ClusterSlotContext;
  actions: ClaimActions;
}) {
  return (
    <ClusterEditProvider context={context} actions={actions}>
      <Probe />
    </ClusterEditProvider>
  );
}

function page(
  context: ClusterSlotContext = slotContext(),
  actions: ClaimActions = serverActions,
) {
  const client = new QueryClient();
  const tree = (c: ClusterSlotContext) => (
    <QueryClientProvider client={client}>
      <GitPushesProvider>
        <Page context={c} actions={actions} />
      </GitPushesProvider>
    </QueryClientProvider>
  );
  const result = render(tree(context));
  return {
    ...result,
    update: (c: ClusterSlotContext) => result.rerender(tree(c)),
  };
}

// delayedActions holds each write until resolve is called, so that a test
// can change the page while a save runs.
function delayedActions() {
  let release: () => void = () => {};
  const actions: ClaimActions = {
    preview: previewClaimChange,
    apply: (change) =>
      applyClaimChange(change).then(
        (result) =>
          new Promise<WriteResult>((r) => {
            release = () => r(result);
          }),
      ),
  };
  return { actions, resolve: () => release() };
}

const click = (name: string) =>
  fireEvent.click(screen.getByRole("button", { name }));
const text = (t: string) => screen.queryByText(t);
const bar = () => screen.queryByRole("region", { name: "Unsaved changes" });
const dialog = () =>
  screen.queryByRole("dialog", { name: "Review the change" });

// confirm waits until the preview has run and the confirm can be clicked.
async function confirm(label: string) {
  const button = within(dialog()!).getByRole<HTMLButtonElement>("button", {
    name: label,
  });
  await waitFor(() => expect(button.disabled).toBe(false));
  fireEvent.click(button);
}

function openReview() {
  fireEvent.click(within(bar()!).getByRole("button", { name: "Review" }));
}

beforeEach(() => {
  fakeWriter.reset();
  FakeEventSource.instances = [];
  vi.stubGlobal("EventSource", FakeEventSource);
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("ClusterEditProvider", () => {
  it("opens no session for an unmanaged cluster, a cluster without its claim, or a page without a write mode", () => {
    for (const context of [
      slotContext({ cluster: { ...cluster(), metadata: { name: "orders" } } }),
      slotContext({ claim: undefined }),
      slotContext({ writeMode: undefined }),
    ]) {
      const { unmount } = page(context);
      expect(text("Session none")).toBeTruthy();
      click("Suspend");
      expect(text("Refusal none")).toBeTruthy();
      expect(text("Pending 0")).toBeTruthy();
      expect(bar()).toBeNull();
      unmount();
    }
    page(slotContext({ writeMode: undefined }));
    expect(screen.getByRole("alert").textContent).toBe(
      "Changes are off: the fleet manager could not read its settings.",
    );
  });

  it("shows no alert for a managed cluster with a claim and a write mode", () => {
    page();
    expect(text("Session open")).toBeTruthy();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("set records a pending change and the change bar counts it", () => {
    page();
    click("Suspend");
    expect(text("Pending 1")).toBeTruthy();
    expect(within(bar()!).getByText("1 unsaved change")).toBeTruthy();
  });

  it("set returns the lock of a locked field and changes nothing", () => {
    const locked: ClusterField = {
      key: "CamundaCluster:spec.zeebe.replicas",
      target: { kind: "CamundaCluster" },
      group: "zeebe",
      path: "spec.zeebe.replicas",
      label: "replicas",
      value: 3,
      state: { kind: "admin" },
      lock: { reason: "admin" },
      claimPath: pathFrom("", replicas),
    };
    page(slotContext({ fields: [locked] }));
    click("Scale");
    expect(text("Refusal admin")).toBeTruthy();
    expect(text("Pending 0")).toBeTruthy();
  });

  it("two calls in one handler build on each other", async () => {
    page();
    click("Suspend and review");
    const review = dialog();
    expect(review).toBeTruthy();
    const diff = within(review!).getByRole("figure", {
      name: "Changes to the claim",
    }).textContent;
    expect(diff).toContain("suspend");
    expect(diff).toContain("replicas");
    await waitFor(() => expect(fakeWriter.previewed).toHaveLength(1));
  });

  it("discard drops every pending change", () => {
    page();
    click("Suspend");
    click("Scale");
    fireEvent.click(within(bar()!).getByRole("button", { name: "Discard" }));
    expect(text("Pending 0")).toBeTruthy();
    expect(bar()).toBeNull();
  });

  it("a claim update keeps a pending change, and drops one that the update made equal", () => {
    const { update } = page();
    click("Suspend");
    click("Scale");
    expect(text("Pending 2")).toBeTruthy();
    update(slotContext({ claim: claim("42", { suspend: true }) }));
    expect(text("Pending 1")).toBeTruthy();
  });

  it("a validation error blocks the review and shows on the bar", () => {
    page();
    click("Missing release");
    expect(
      screen.getByRole("list", { name: "Field errors" }).textContent,
    ).toContain("spec.releaseRef: Release missing does not exist.");
    const review = within(bar()!).getByRole<HTMLButtonElement>("button", {
      name: "Review",
    });
    expect(review.disabled).toBe(true);
    expect(
      within(bar()!).getByText(/^Fix \d+ fields? before review\.$/),
    ).toBeTruthy();
  });

  it("the review shows the impacts and saves the claim with the precondition", async () => {
    page();
    click("Suspend");
    openReview();
    const impacts = within(dialog()!).getByRole("list", {
      name: "What this change does",
    });
    expect(impacts.textContent).toContain(
      "Every workload goes to zero. The volumes stay.",
    );
    await confirm("Save");

    await waitFor(() => expect(dialog()).toBeNull());
    const written = fakeWriter.applied[0];
    expect(
      (written.apply[0] as KubeObject & { spec: { suspend?: boolean } }).spec
        .suspend,
    ).toBe(true);
    expect(written.preconditions).toEqual({ [PRECONDITION]: "41" });
    expect(written.intent).toBe("Change the claim orders");
    expect(screen.getByRole("status").textContent).toContain(
      "The claim orders is saved.",
    );
    expect(text("Pending 0")).toBeTruthy();
  });

  it("in Git mode the review pushes to Git and the toast waits for GitOps", async () => {
    fakeWriter.reset("git");
    page(slotContext({ writeMode: "git" }));
    click("Suspend");
    openReview();
    await confirm("Push to Git");

    await waitFor(() =>
      expect(screen.getByRole("status").textContent).toContain(
        "Waiting for GitOps to apply",
      ),
    );
  });

  it("in Git mode, a second save before GitOps applies keeps the first change", async () => {
    fakeWriter.reset("git");
    page(slotContext({ writeMode: "git" }));
    click("Suspend");
    openReview();
    await confirm("Push to Git");
    await waitFor(() => expect(dialog()).toBeNull());

    click("Scale");
    openReview();
    await confirm("Push to Git");
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(2));

    const second = fakeWriter.applied[1];
    const spec = (
      second.apply[0] as KubeObject & {
        spec: { suspend?: boolean; cluster: { zeebe: { replicas: number } } };
      }
    ).spec;
    expect(spec.suspend).toBe(true);
    expect(spec.cluster.zeebe.replicas).toBe(5);
    expect(second.preconditions).toEqual({ [PRECONDITION]: "41" });
  });

  it("a claim update during the review previews again with the new resourceVersion", async () => {
    const { update } = page();
    click("Suspend");
    openReview();
    await waitFor(() => expect(fakeWriter.previewed).toHaveLength(1));

    update(slotContext({ claim: claim("42") }));

    await waitFor(() =>
      expect(fakeWriter.previewed.at(-1)?.preconditions).toEqual({
        [PRECONDITION]: "42",
      }),
    );
    expect(dialog()).toBeTruthy();
  });

  it("a claim update that makes the change invalid closes the review and blocks it", async () => {
    const environment = (presets: string[]) =>
      ({
        kind: "Environment",
        metadata: { name: "prod" },
        spec: { presets: { camundaCluster: presets } },
      }) as unknown as Resource<"Environment">;
    const presets = ["medium", "large"].map(
      (name) =>
        ({
          kind: "CamundaClusterPreset",
          metadata: { name },
          spec: { cluster: {} },
        }) as unknown as Resource<"CamundaClusterPreset">,
    );
    const { update } = page(
      slotContext({ presets, environment: environment(["medium", "large"]) }),
    );
    click("Large preset");
    openReview();
    expect(dialog()).toBeTruthy();

    update(slotContext({ presets, environment: environment(["medium"]) }));

    await waitFor(() => expect(dialog()).toBeNull());
    expect(
      within(bar()!).getByRole<HTMLButtonElement>("button", { name: "Review" })
        .disabled,
    ).toBe(true);
    expect(fakeWriter.applied).toEqual([]);
  });

  it("a claim update that arrives before the save returns is kept", async () => {
    const { actions, resolve } = delayedActions();
    const { update } = page(slotContext(), actions);
    click("Suspend");
    openReview();
    await confirm("Save");
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));

    update(slotContext({ claim: claim("42", { suspend: true }) }));
    await act(async () => resolve());

    await waitFor(() => expect(dialog()).toBeNull());
    expect(text("Pending 0")).toBeTruthy();
    click("Scale");
    openReview();
    await confirm("Save");
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(2));
    await act(async () => resolve());
    expect(fakeWriter.applied[1].preconditions).toEqual({
      [PRECONDITION]: "42",
    });
  });

  it("the review stays open while the save runs", async () => {
    const { actions, resolve } = delayedActions();
    page(slotContext(), actions);
    click("Scale");
    openReview();
    await confirm("Save");
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));

    expect(
      within(dialog()!).queryByRole("button", { name: "Back" }),
    ).toBeNull();
    fireEvent.keyDown(dialog()!, { key: "Escape" });
    fireEvent.click(within(dialog()!).getByRole("button", { name: "Close" }));
    expect(dialog()).toBeTruthy();

    await act(async () => resolve());
    await waitFor(() => expect(dialog()).toBeNull());
    expect(text("Pending 0")).toBeTruthy();
  });

  it("a review that closes on a claim update does not open again on the next change", async () => {
    const { update } = page();
    click("Suspend");
    openReview();
    expect(dialog()).toBeTruthy();

    update(slotContext({ claim: claim("42", { suspend: true }) }));
    await waitFor(() => expect(dialog()).toBeNull());
    click("Scale");

    expect(text("Pending 1")).toBeTruthy();
    expect(dialog()).toBeNull();
  });

  it("a refused write keeps the pending changes and gives the field error", async () => {
    fakeWriter.results = [
      {
        ok: false,
        errors: [
          {
            code: "Invalid",
            message: "must be 1 or more",
            path: "spec.cluster.zeebe.replicas",
          },
        ],
      },
    ];
    page();
    click("Scale");
    openReview();
    await confirm("Save");

    await waitFor(() =>
      expect(within(dialog()!).getByRole("alert").textContent).toContain(
        "must be 1 or more",
      ),
    );
    expect(text("Pending 1")).toBeTruthy();
    // The open dialog hides the page from the accessibility tree.
    expect(
      screen.getByRole("list", { name: "Field errors", hidden: true })
        .textContent,
    ).toContain("spec.cluster.zeebe.replicas: must be 1 or more");
  });

  it("rendering the page previews and writes nothing", async () => {
    page();
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(fakeWriter.previewed).toEqual([]);
    expect(fakeWriter.applied).toEqual([]);
  });

  it("useClusterEdit throws without a provider", () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => render(<Probe />)).toThrow();
  });

  it("show reports a write of another flow through the toast of the page", () => {
    page();
    click("Back up");
    const toasts = screen.getAllByRole("status");
    expect(toasts).toHaveLength(1);
    expect(toasts[0].textContent).toContain("Backup orders-manual is created.");
  });
});
