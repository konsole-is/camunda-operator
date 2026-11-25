// @vitest-environment jsdom
import { createHash } from "node:crypto";
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
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  applyClaimChange,
  previewClaimChange,
} from "@/app/fleet/clusters/[namespace]/[name]/actions";
import { GitPushesProvider, useGitPushes } from "@/components/fleet/git-pushes";
import { REVISION_ANNOTATION } from "@/lib/labels";
import { pathFrom, type ClusterField, type Step } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { STALE_CLAIM_MESSAGE, valueAt } from "@/lib/session";
import type { KubeObject, WriteResult } from "@/lib/write/types";
import { FakeEventSource } from "../../../../test/fakes/event-source";
import {
  claim,
  cluster,
  slotContext,
} from "../../../../test/fakes/slot-context";
import { fakeWriter } from "../../../../test/fakes/writer";
import type { BackupObject } from "../backups";
import type { ClusterSlotContext } from "../slots";
import { ComponentSettingsEditor } from "./settings-slot";
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
const partitions: Step[] = [
  { field: "spec" },
  { field: "cluster" },
  { field: "zeebe" },
  { field: "partitions" },
];
const releaseRef: Step[] = [{ field: "spec" }, { field: "releaseRef" }];
const presetRef: Step[] = [{ field: "spec" }, { field: "presetRef" }];
const allowDowngrade: Step[] = [
  { field: "spec" },
  { field: "allowVersionDowngrade" },
];
const schedule: Step[] = [
  { field: "spec" },
  { field: "attachments" },
  { field: "backupSchedule" },
];
const zeebeCpu = (side: "requests" | "limits"): Step[] => [
  { field: "spec" },
  { field: "cluster" },
  { field: "zeebe" },
  { field: "resources" },
  { field: side },
  { field: "cpu" },
];
const zeebeResources: Step[] = [
  { field: "spec" },
  { field: "cluster" },
  { field: "zeebe" },
  { field: "resources" },
];
const PRECONDITION = "CamundaClusterClaim/team-a/orders";

// Probe is an editor: it changes the claim only through useClusterEdit.
function Probe() {
  const edit = useClusterEdit();
  const [refusal, setRefusal] = useState<string>();
  const [opened, setOpened] = useState<boolean>();
  return (
    <div>
      <p>Opened {opened === undefined ? "unknown" : opened ? "yes" : "no"}</p>
      <p>Session {edit.session ? "open" : "none"}</p>
      <p>Pending {edit.pending}</p>
      <p>Refusal {refusal ?? "none"}</p>
      <p>Replicas {String(valueAt(edit.session?.claim, replicas))}</p>
      <p>Suspended {String(valueAt(edit.session?.claim, suspend) ?? false)}</p>
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
      <button onClick={() => edit.set(replicas, 3)}>Scale back</button>
      <button onClick={() => edit.set(partitions, 3)}>Partition</button>
      <button onClick={() => edit.set(schedule, { cron: "0 1 * * *" })}>
        Attach schedule
      </button>
      <button onClick={() => edit.set(releaseRef, "missing")}>
        Missing release
      </button>
      <button onClick={() => edit.set(presetRef, "large")}>Large preset</button>
      <button onClick={() => edit.reset(zeebeCpu("requests"))}>
        Clear the CPU request
      </button>
      <button onClick={() => edit.set(zeebeCpu("limits"), "1")}>
        Limit the CPU to 1
      </button>
      <button
        onClick={() => edit.set(zeebeResources, { limits: { cpu: "1" } })}
      >
        Write the resources with only a CPU limit of 1
      </button>
      <button onClick={() => edit.reset(zeebeResources)}>
        Reset the resources
      </button>
      <button
        onClick={() =>
          edit.set(
            [
              { field: "spec" },
              { field: "cluster" },
              { field: "monitoring" },
              { field: "serviceMonitor" },
              { field: "enabled" },
            ],
            true,
          )
        }
      >
        Turn on the ServiceMonitor
      </button>
      <button
        onClick={() => edit.invalid(replicas, "Replicas: Enter 1 or more")}
      >
        Show replicas error
      </button>
      <button onClick={() => edit.invalid(replicas, undefined)}>
        Clear replicas error
      </button>
      <button onClick={() => setOpened(edit.review())}>Try review</button>
      <button
        onClick={() => {
          edit.set(releaseRef, "camunda-8-9-0");
          edit.set(allowDowngrade, "8.9.0");
        }}
      >
        Downgrade
      </button>
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
      <Dropper />
      <ComponentSettingsEditor ctx={context} />
    </ClusterEditProvider>
  );
}

// Dropper stops following every pending Git push of the tab, as when the
// push never reaches the cluster.
function Dropper() {
  const { pushes, settle } = useGitPushes();
  return (
    <>
      <p>Pushes {pushes.length}</p>
      <button onClick={() => pushes.forEach((p) => settle(p.key))}>
        Drop pushes
      </button>
    </>
  );
}

// appliedClaim is the claim at resourceVersion with spec, as GitOps applies
// the first push of fakeWriter: it carries the revision of that push.
function appliedClaim(resourceVersion: string, spec: object) {
  const pushed = fakeWriter.applied[0].apply[0];
  const revision = createHash("sha1")
    .update(JSON.stringify(pushed))
    .digest("hex");
  const c = claim(resourceVersion, spec);
  return {
    ...c,
    metadata: {
      ...c.metadata,
      annotations: { [REVISION_ANNOTATION]: revision },
    },
  };
}

function page(
  context: ClusterSlotContext = slotContext(),
  actions: ClaimActions = serverActions,
) {
  const client = new QueryClient();
  let host = 0;
  let last = context;
  const tree = (c: ClusterSlotContext) => (
    <QueryClientProvider client={client}>
      <GitPushesProvider>
        <Page key={host} context={c} actions={actions} />
      </GitPushesProvider>
    </QueryClientProvider>
  );
  const result = render(tree(context));
  return {
    ...result,
    update: (c: ClusterSlotContext) => {
      last = c;
      result.rerender(tree(c));
    },
    // remount mounts a new host in the same tab, as when the user leaves
    // the cluster page and comes back.
    remount: () => {
      host++;
      result.rerender(tree(last));
    },
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

// removedLines returns the removed lines of the claim diff of the review.
function removedLines() {
  const figure = within(dialog()!).getByRole("figure", {
    name: "Changes to the claim",
  });
  return [...figure.querySelectorAll("del")].map((d) => d.textContent?.trim());
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

  it("an error that an editor shows blocks the review and names the field until the editor clears it", () => {
    page();
    click("Scale");
    click("Show replicas error");
    const review = within(bar()!).getByRole<HTMLButtonElement>("button", {
      name: "Review",
    });
    expect(review.disabled).toBe(true);
    expect(within(bar()!).getByText("Replicas: Enter 1 or more")).toBeTruthy();
    click("Try review");
    expect(text("Opened no")).toBeTruthy();
    expect(dialog()).toBeNull();
    click("Clear replicas error");
    expect(review.disabled).toBe(false);
    expect(within(bar()!).queryByText("Replicas: Enter 1 or more")).toBeNull();
    click("Try review");
    expect(text("Opened yes")).toBeTruthy();
    expect(dialog()).toBeTruthy();
  });

  it("review says that it did not open the review without a pending change or with a validation error", () => {
    page();
    click("Try review");
    expect(text("Opened no")).toBeTruthy();
    click("Missing release");
    click("Try review");
    expect(text("Opened no")).toBeTruthy();
    expect(dialog()).toBeNull();
  });

  it("closing the review gives the focus back to Review of the change bar", async () => {
    page();
    click("Suspend");
    openReview();
    expect(dialog()).toBeTruthy();
    fireEvent.click(within(dialog()!).getByRole("button", { name: "Back" }));
    await waitFor(() => expect(dialog()).toBeNull());
    expect(document.activeElement).toBe(
      within(bar()!).getByRole("button", { name: "Review" }),
    );
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

  it("checks a storage preset of the claim against the requests of the context", () => {
    const storageClaim = {
      ...claim("41", {
        plugins: [
          {
            section: "secondary-storage",
            registration: "elasticsearch",
            parameters: { preset: "small" },
          },
        ],
      }),
      status: {
        resolvedPolicy: {
          sections: { "secondary-storage": { allowed: ["elasticsearch"] } },
          parameters: { elasticsearch: { preset: { claimEditable: true } } },
        },
      },
    } as unknown as Resource<"CamundaClusterClaim">;
    const request = {
      metadata: { namespace: "team-a", name: "orders-secondary-storage" },
      spec: {
        claimRef: { kind: "CamundaClusterClaim", name: "orders" },
        section: "secondary-storage",
        registration: "elasticsearch",
        parameters: { preset: "small" },
      },
    } as unknown as Resource<"PluginRequest">;
    const sections = [
      {
        kind: "PluginSection",
        metadata: { name: "secondary-storage" },
        spec: {
          label: "Secondary storage",
          slot: "secondaryStorage",
          appliesTo: ["CamundaClusterClaim"],
          selection: "One",
        },
      } as unknown as Resource<"PluginSection">,
    ];
    const errors = () =>
      screen.getByRole("list", { name: "Field errors" }).textContent;
    const { update } = page(
      slotContext({ claim: storageClaim, sections, requests: [request] }),
    );
    click("Scale");
    expect(errors()).toBe("");
    update(slotContext({ claim: storageClaim, sections, requests: [] }));
    expect(errors()).toContain(
      'ElasticsearchClusterPreset "small" is not in spec.presets.elasticsearchCluster of Environment "prod"',
    );
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

  it("the review of a claim without its cluster names no impact of a running cluster and saves the claim", async () => {
    page(
      slotContext({
        drafted: true,
        cluster: { ...cluster(), status: undefined },
      }),
    );
    click("Suspend");
    openReview();
    expect(
      within(dialog()!).queryByRole("list", { name: "What this change does" }),
    ).toBeNull();
    expect(
      within(dialog()!).queryByText(
        "Every workload goes to zero. The volumes stay.",
      ),
    ).toBeNull();
    await confirm("Save");

    await waitFor(() => expect(dialog()).toBeNull());
    const written = fakeWriter.applied[0];
    expect(
      (written.apply[0] as KubeObject & { spec: { suspend?: boolean } }).spec
        .suspend,
    ).toBe(true);
    expect(written.preconditions).toEqual({ [PRECONDITION]: "41" });
  });

  it("the review of a claim without its cluster blocks the save while the cluster is not read", () => {
    page(
      slotContext({
        drafted: true,
        unread: ["CamundaCluster"],
        cluster: { ...cluster(), status: undefined },
      }),
    );
    click("Suspend");
    openReview();
    expect(
      within(dialog()!).getByText(
        "A change is possible when the fleet manager can read CamundaCluster.",
      ),
    ).toBeTruthy();
    expect(
      within(dialog()!).getByRole<HTMLButtonElement>("button", { name: "Save" })
        .disabled,
    ).toBe(true);
  });

  it("the review of a claim without its cluster offers the save again once the cluster is read", async () => {
    const unread = (kinds: string[]) =>
      slotContext({
        drafted: true,
        unread: kinds,
        cluster: { ...cluster(), status: undefined },
      });
    const { update } = page(unread(["CamundaCluster"]));
    click("Suspend");
    update(unread([]));
    openReview();
    expect(
      within(dialog()!).queryByText(
        "A change is possible when the fleet manager can read CamundaCluster.",
      ),
    ).toBeNull();
    await confirm("Save");
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
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

  it("in Git mode, the session goes on from the pushed claim", async () => {
    fakeWriter.reset("git");
    page(slotContext({ writeMode: "git" }));
    click("Suspend");
    openReview();
    await confirm("Push to Git");
    await waitFor(() => expect(dialog()).toBeNull());

    expect(text("Suspended true")).toBeTruthy();
    click("Scale");
    expect(text("Pending 1")).toBeTruthy();
  });

  it("in Git mode, a status update while the push waits keeps the pushed claim and a change back to the live value", async () => {
    fakeWriter.reset("git");
    const { update } = page(slotContext({ writeMode: "git" }));
    click("Scale");
    openReview();
    await confirm("Push to Git");
    await waitFor(() => expect(dialog()).toBeNull());
    click("Scale back");
    expect(text("Pending 1")).toBeTruthy();

    update(slotContext({ writeMode: "git", claim: claim("42") }));
    update(slotContext({ writeMode: "git", claim: claim("43") }));

    expect(text("Pending 1")).toBeTruthy();
    expect(text("Replicas 5")).toBeTruthy();
  });

  it("in Git mode, Review waits until GitOps applies the push", async () => {
    fakeWriter.reset("git");
    const { update } = page(slotContext({ writeMode: "git" }));
    click("Scale");
    openReview();
    await confirm("Push to Git");
    await waitFor(() => expect(dialog()).toBeNull());
    click("Suspend");

    const review = () =>
      within(bar()!).getByRole<HTMLButtonElement>("button", { name: "Review" });
    expect(review().disabled).toBe(true);
    expect(
      within(bar()!).getByText("Waiting for GitOps to apply your last change."),
    ).toBeTruthy();

    update(
      slotContext({
        writeMode: "git",
        claim: appliedClaim("43", { cluster: { zeebe: { replicas: 5 } } }),
      }),
    );

    expect(review().disabled).toBe(false);
    expect(text("Pending 1")).toBeTruthy();
    expect(text("Replicas 5")).toBeTruthy();
  });

  it("in Git mode, a claim that GitOps moved past the push ends the wait", async () => {
    fakeWriter.reset("git");
    const { update } = page(slotContext({ writeMode: "git" }));
    click("Scale");
    openReview();
    await confirm("Push to Git");
    await waitFor(() => expect(dialog()).toBeNull());
    click("Suspend");

    const later = claim("44", { cluster: { zeebe: { replicas: 7 } } });
    update(
      slotContext({
        writeMode: "git",
        claim: {
          ...later,
          metadata: {
            ...later.metadata,
            annotations: { [REVISION_ANNOTATION]: "a later revision" },
          },
        },
      }),
    );

    expect(text("Replicas 7")).toBeTruthy();
    expect(
      within(bar()!).getByRole<HTMLButtonElement>("button", { name: "Review" })
        .disabled,
    ).toBe(false);
    await waitFor(() => expect(text("Pushes 0")).toBeTruthy());
    expect(screen.getByRole("status").textContent).not.toContain(
      "Waiting for GitOps",
    );
  });

  it.each([
    ["moves to a new revision", "a later revision", false],
    ["changes without a new revision", undefined, true],
  ] as const)(
    "in Git mode, a claim that %s while the push runs decides the wait from the revision before the push",
    async (_, revision, waits) => {
      fakeWriter.reset("git");
      const { actions, resolve } = delayedActions();
      const { update } = page(slotContext({ writeMode: "git" }), actions);
      click("Scale");
      openReview();
      await confirm("Push to Git");
      await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));

      const moved = claim("42");
      update(
        slotContext({
          writeMode: "git",
          claim: {
            ...moved,
            metadata: {
              ...moved.metadata,
              annotations: revision ? { [REVISION_ANNOTATION]: revision } : {},
            },
          },
        }),
      );
      await act(async () => resolve());
      await waitFor(() => expect(dialog()).toBeNull());
      click("Suspend");

      expect(
        within(bar()!).getByRole<HTMLButtonElement>("button", {
          name: "Review",
        }).disabled,
      ).toBe(waits);
    },
  );

  it("in Git mode, a push that the page stops following returns to the live claim and says so", async () => {
    fakeWriter.reset("git");
    page(slotContext({ writeMode: "git" }));
    click("Scale");
    openReview();
    await confirm("Push to Git");
    await waitFor(() => expect(dialog()).toBeNull());
    expect(text("Replicas 5")).toBeTruthy();
    click("Partition");

    click("Drop pushes");

    expect(
      screen.getByText(
        "GitOps did not apply your last change. The page shows the live claim again.",
      ),
    ).toBeTruthy();
    expect(text("Replicas 3")).toBeTruthy();
    expect(text("Pending 1")).toBeTruthy();
  });

  it("in Git mode, a push that GitOps never applies, as after a revert of the file, ends on Stop waiting", async () => {
    fakeWriter.reset("git");
    const { update } = page(slotContext({ writeMode: "git" }));
    click("Scale");
    openReview();
    await confirm("Push to Git");
    await waitFor(() => expect(dialog()).toBeNull());
    click("Partition");
    // The claim keeps the revision that it had before the push.
    update(slotContext({ writeMode: "git", claim: claim("42") }));
    const review = () =>
      within(bar()!).getByRole<HTMLButtonElement>("button", { name: "Review" });
    expect(review().disabled).toBe(true);

    click("Stop waiting");

    expect(review().disabled).toBe(false);
    expect(
      screen.getByText(
        "GitOps did not apply your last change. The page shows the live claim again.",
      ),
    ).toBeTruthy();
    expect(text("Replicas 3")).toBeTruthy();
    expect(text("Pushes 0")).toBeTruthy();
  });

  it("in Git mode, Stop waiting removes the waiting toast and shows no saved toast", async () => {
    fakeWriter.reset("git");
    page(slotContext({ writeMode: "git" }));
    click("Scale");
    openReview();
    await confirm("Push to Git");
    await waitFor(() => expect(dialog()).toBeNull());
    expect(text("Waiting for GitOps to apply")).toBeTruthy();

    click("Stop waiting");

    expect(text("Waiting for GitOps to apply")).toBeNull();
    expect(text("The claim orders is saved.")).toBeNull();
  });

  it("a write whose request fails while the claim updates keeps the failure in the review", async () => {
    let fail: (error: Error) => void = () => {};
    const actions: ClaimActions = {
      preview: previewClaimChange,
      apply: () =>
        new Promise<WriteResult>((_, reject) => {
          fail = reject;
        }),
    };
    const { update } = page(slotContext(), actions);
    click("Scale");
    openReview();
    await confirm("Save");

    update(slotContext({ claim: claim("42") }));
    await act(async () =>
      fail(new Error("The fleet manager is not reachable.")),
    );

    expect(
      within(dialog()!).getByText(/The fleet manager is not reachable\./),
    ).toBeTruthy();
  });

  it("in Git mode, a new host of a claim with a waiting push blocks Review and shows the live claim", async () => {
    fakeWriter.reset("git");
    const { remount } = page(slotContext({ writeMode: "git" }));
    click("Scale");
    openReview();
    await confirm("Push to Git");
    await waitFor(() => expect(dialog()).toBeNull());

    remount();
    click("Partition");

    expect(text("Replicas 3")).toBeTruthy();
    expect(
      within(bar()!).getByRole<HTMLButtonElement>("button", { name: "Review" })
        .disabled,
    ).toBe(true);
    expect(
      within(bar()!).getByText(
        "Waiting for GitOps to apply your last change. The page shows the live claim until GitOps applies it.",
      ),
    ).toBeTruthy();
  });

  it("in Git mode, a push of a new host that the Git writer refuses shows the message of the writer", async () => {
    fakeWriter.reset("git");
    const message =
      "team-a/camundaclusterclaim/orders.yaml holds a save that GitOps has not applied yet.";
    const { remount } = page(slotContext({ writeMode: "git" }));
    click("Scale");
    openReview();
    await confirm("Push to Git");
    await waitFor(() => expect(dialog()).toBeNull());
    remount();
    click("Stop waiting");
    fakeWriter.results = [
      { ok: false, errors: [{ code: "Conflict", message }] },
    ];

    click("Partition");
    openReview();
    await confirm("Push to Git");

    await waitFor(() =>
      expect(within(dialog()!).getByRole("alert").textContent).toContain(
        message,
      ),
    );
  });

  it("in direct mode, a second save before the claim update sends both changes with the resourceVersion that the page read", async () => {
    page();
    click("Suspend");
    openReview();
    await confirm("Save");
    await waitFor(() => expect(dialog()).toBeNull());

    click("Scale");
    openReview();
    await confirm("Save");
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

  it("in direct mode, a second save that is Stale before the claim update saves again after it", async () => {
    const { update } = page();
    click("Suspend");
    openReview();
    await confirm("Save");
    await waitFor(() => expect(dialog()).toBeNull());
    fakeWriter.results = [
      { ok: false, errors: [{ code: "Stale", message: "x" }] },
    ];
    click("Scale");
    openReview();
    await confirm("Save");
    await waitFor(() =>
      expect(within(dialog()!).getByRole("alert").textContent).toContain(
        STALE_CLAIM_MESSAGE,
      ),
    );

    update(slotContext({ claim: claim("42", { suspend: true }) }));
    await confirm("Save");

    await waitFor(() => expect(fakeWriter.applied).toHaveLength(3));
    expect(fakeWriter.applied[2].preconditions).toEqual({
      [PRECONDITION]: "42",
    });
  });

  it("the note of a refused write ends with a review that a claim update closed", async () => {
    fakeWriter.results = [
      { ok: false, errors: [{ code: "Conflict", message: "Refused." }] },
    ];
    const { update } = page();
    click("Scale");
    openReview();
    await confirm("Save");
    await waitFor(() =>
      expect(within(dialog()!).getByRole("alert").textContent).toContain(
        "Refused.",
      ),
    );

    update(
      slotContext({
        claim: claim("42", { cluster: { zeebe: { replicas: 5 } } }),
      }),
    );
    await waitFor(() => expect(dialog()).toBeNull());
    click("Scale back");
    openReview();

    expect(within(dialog()!).queryByText("Refused.")).toBeNull();
  });

  it("a change that restarts the brokers during a running backup shows the caution and Change anyway", async () => {
    const backup = {
      kind: "LogicalBackupRDBMS",
      metadata: { namespace: "team-a", name: "nightly" },
      spec: { clusterRef: { name: "orders" } },
      status: { phase: "Running" },
    } as unknown as BackupObject;
    page(slotContext({ backups: [backup] }));
    click("Scale");
    openReview();

    expect(
      within(dialog()!).getByRole("note", { name: "Caution" }).textContent,
    ).toContain("Wait until backup nightly finishes.");
    await waitFor(() =>
      expect(
        within(dialog()!)
          .getByRole("button", { name: "Change anyway" })
          .getAttribute("data-variant"),
      ).toBe("destructive"),
    );
  });

  describe("a save that keeps a CPU request of another tool", () => {
    const spec = {
      zeebe: { replicas: 3, resources: { requests: { cpu: "2" } } },
    };
    const kubectl = {
      name: "kubectl",
      operation: "Apply",
      kind: "other" as const,
      known: true,
    };
    const ref = {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "CamundaClusterClaim",
      namespace: "team-a",
      name: "orders",
    };
    const ownership = [
      {
        ref,
        changed: [],
        cleared: [
          {
            path: "spec.cluster.zeebe.resources.requests.cpu",
            steps: zeebeCpu("requests") as { field: string }[],
            managers: [kubectl],
          },
        ],
      },
    ];
    // context holds a claim whose CPU request kubectl owns, under a preset
    // whose CPU request is 1.
    function context(): ClusterSlotContext {
      const owned = claim("41", { cluster: spec });
      owned.metadata.managedFields = [
        {
          manager: "kubectl",
          operation: "Apply",
          fieldsV1: {
            "f:spec": {
              "f:cluster": {
                "f:zeebe": {
                  "f:resources": { "f:requests": { "f:cpu": {} } },
                },
              },
            },
          },
        },
      ];
      return slotContext({
        cluster: cluster(spec),
        claim: owned,
        presets: [
          {
            kind: "CamundaClusterPreset",
            metadata: { name: "medium" },
            spec: {
              cluster: { zeebe: { resources: { requests: { cpu: "1" } } } },
            },
          } as unknown as Resource<"CamundaClusterPreset">,
        ],
      });
    }
    const message =
      "zeebe.resources: The CPU limit of 1 is below the CPU request of 2. A field that you cleared stays in the claim that the save leaves, or gets its default value again. That causes this error. Go back and change the edit, or take the fields over when the review offers that.";
    const choice = () =>
      within(dialog()!).getByRole("checkbox", {
        name: "Take over these fields",
      });
    const save = () =>
      within(dialog()!).getByRole<HTMLButtonElement>("button", {
        name: "Save",
      });

    it("refuses the save and names the kept value", async () => {
      fakeWriter.previews = [{ objects: [], errors: [], ownership }];
      page(context());
      click("Clear the CPU request");
      click("Limit the CPU to 1");
      openReview();
      await waitFor(() => expect(choice()).toBeTruthy());
      expect(within(dialog()!).getByRole("alert").textContent).toBe(message);
      expect(save().disabled).toBe(true);
    });

    it("in Git mode, refuses the push when the dry run keeps the request", async () => {
      fakeWriter.reset("git");
      const kept = { ...context(), writeMode: "git" as const };
      fakeWriter.previews = [
        {
          objects: [{ ref, after: kept.claim as KubeObject, diff: [] }],
          errors: [],
        },
      ];
      page(kept);
      click("Clear the CPU request");
      click("Limit the CPU to 1");
      openReview();
      await waitFor(() =>
        expect(within(dialog()!).getByRole("alert").textContent).toContain(
          message,
        ),
      );
      expect(
        within(dialog()!).getByRole<HTMLButtonElement>("button", {
          name: "Push to Git",
        }).disabled,
      ).toBe(true);
    });

    // clearRequestAndLimit clears the CPU request and sets the CPU limit
    // to 1 in the resources editor of the brokers.
    async function clearRequestAndLimit() {
      fireEvent.click(screen.getByRole("button", { name: "Edit settings" }));
      const amount = (label: string) =>
        within(
          within(
            within(
              screen.getByRole("region", { name: "Brokers settings" }),
            ).getAllByRole("group", { name: "resources" })[0],
          ).getByRole("group", { name: label }),
        ).getByRole("textbox", { name: "Amount" });
      await userEvent.clear(amount("CPU request"));
      await userEvent.type(amount("CPU limit"), "1");
    }

    it("refuses the save after the edit in the resources editor", async () => {
      fakeWriter.previews = [{ objects: [], errors: [], ownership }];
      page({ ...context(), openId: "zeebe" });
      await clearRequestAndLimit();
      openReview();
      await waitFor(() => expect(choice()).toBeTruthy());
      expect(within(dialog()!).getByRole("alert").textContent).toBe(message);
      expect(save().disabled).toBe(true);
    });

    it("in Git mode, refuses the push after the edit in the resources editor and names the kept request once", async () => {
      fakeWriter.reset("git");
      const kept = {
        ...context(),
        openId: "zeebe" as const,
        writeMode: "git" as const,
      };
      fakeWriter.previews = [
        {
          objects: [{ ref, after: kept.claim as KubeObject, diff: [] }],
          errors: [],
        },
      ];
      page(kept);
      await clearRequestAndLimit();
      openReview();
      await waitFor(() =>
        expect(
          within(dialog()!).getAllByRole("alert")[0].textContent,
        ).toContain(message),
      );
      const notes = within(dialog()!)
        .getAllByRole("alert")
        .map((a) => a.textContent ?? "")
        .join("\n");
      expect(
        notes.split("spec.cluster.zeebe.resources.requests.cpu").length - 1,
      ).toBe(1);
      expect(
        within(dialog()!).getByRole<HTMLButtonElement>("button", {
          name: "Push to Git",
        }).disabled,
      ).toBe(true);
    });

    it("saves once the user takes the request over, and refuses again when the choice goes off", async () => {
      fakeWriter.previews = [
        { objects: [], errors: [], ownership },
        { objects: [], errors: [] },
      ];
      page(context());
      click("Clear the CPU request");
      click("Limit the CPU to 1");
      openReview();
      await waitFor(() => expect(choice()).toBeTruthy());
      fireEvent.click(choice());
      await waitFor(() => expect(save().disabled).toBe(false));
      expect(within(dialog()!).queryByRole("alert")).toBeNull();

      fireEvent.click(choice());
      expect(within(dialog()!).getByRole("alert").textContent).toBe(message);
      expect(save().disabled).toBe(true);
    });
  });

  describe("a label key of the claim that is not valid, with no editor open", () => {
    const review = () =>
      within(bar()!).getByRole<HTMLButtonElement>("button", {
        name: "Review",
      });
    const withCluster = (spec: object) =>
      slotContext({
        cluster: cluster(spec),
        claim: claim("41", { cluster: { zeebe: { replicas: 3 }, ...spec } }),
      });

    it("lets a change through while the ServiceMonitor is off, and blocks Review once the change turns it on", () => {
      page(
        withCluster({
          monitoring: {
            serviceMonitor: { enabled: false, labels: { "bad key": "x" } },
          },
        }),
      );
      click("Scale");
      expect(review().disabled).toBe(false);
      click("Turn on the ServiceMonitor");
      expect(review().disabled).toBe(true);
      expect(bar()!.textContent).toContain(
        'monitoring.serviceMonitor.labels: The label key "bad key" is not valid. Correct it, or remove its row.',
      );
    });

    it("blocks Review for a label key of the pod labels of the brokers", () => {
      page(
        withCluster({ zeebe: { replicas: 3, podLabels: { "bad key": "x" } } }),
      );
      click("Scale");
      expect(review().disabled).toBe(true);
      expect(bar()!.textContent).toContain(
        'zeebe.podLabels: The label key "bad key" is not valid. Correct it, or remove its row.',
      );
    });
  });

  describe("in Git mode, a push that keeps fields of another tool", () => {
    const ref = {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "CamundaClusterClaim",
      namespace: "team-a",
      name: "orders",
    };
    // owns returns a managedFields entry of an apply by manager that owns
    // the fields of zeebe.resources in resources.
    const owns = (manager: string, resources: object, revision = false) => ({
      manager,
      operation: "Apply",
      fieldsV1: {
        ...(revision && {
          "f:metadata": {
            "f:annotations": { [`f:${REVISION_ANNOTATION}`]: {} },
          },
        }),
        "f:spec": { "f:cluster": { "f:zeebe": { "f:resources": resources } } },
      },
    });
    // gitPage renders the page in Git mode for a claim with the broker
    // resources of live, the managedFields of managers, and a preset with
    // the broker resources of preset. after is the claim that the dry run
    // of the push gives.
    function gitPage(
      live: object,
      managers: object[],
      preset: object,
      after: object,
    ) {
      fakeWriter.reset("git");
      const spec = { zeebe: { replicas: 3, resources: live } };
      const owned = claim("41", { cluster: spec });
      owned.metadata.managedFields =
        managers as typeof owned.metadata.managedFields;
      const dryRun = claim("41", {
        cluster: { zeebe: { replicas: 3, resources: after } },
      });
      fakeWriter.previews = [
        {
          objects: [{ ref, after: dryRun as KubeObject, diff: [] }],
          errors: [],
        },
      ];
      page(
        slotContext({
          writeMode: "git",
          cluster: cluster(spec),
          claim: owned,
          presets: [
            {
              kind: "CamundaClusterPreset",
              metadata: { name: "medium" },
              spec: { cluster: { zeebe: { resources: preset } } },
            } as unknown as Resource<"CamundaClusterPreset">,
          ],
        }),
      );
    }
    const push = () =>
      within(dialog()!).getByRole<HTMLButtonElement>("button", {
        name: "Push to Git",
      });
    const KEPT =
      "A field that you cleared stays in the claim that the save leaves, or gets its default value again. That causes this error. Go back and change the edit, or take the fields over when the review offers that.";

    it("refuses a write of the whole block that leaves a CPU request of another tool above the new limit", async () => {
      gitPage(
        { requests: { cpu: "2" } },
        [
          owns("kubectl", { "f:requests": { "f:cpu": {} } }),
          owns("argocd-controller", {}, true),
        ],
        { requests: { cpu: "1" } },
        { requests: { cpu: "2" }, limits: { cpu: "1" } },
      );
      click("Write the resources with only a CPU limit of 1");
      openReview();
      await waitFor(() => expect(fakeWriter.previewed).toHaveLength(1));
      expect(within(dialog()!).getByRole("alert").textContent).toContain(
        `zeebe.resources: The CPU limit of 1 is below the CPU request of 2. ${KEPT}`,
      );
      expect(push().disabled).toBe(true);
    });

    it("pushes a reset of the block when only a valid field of another tool stays", async () => {
      gitPage(
        { limits: { cpu: "1" }, requests: { memory: "1Gi" } },
        [
          owns("argocd-controller", { "f:limits": { "f:cpu": {} } }, true),
          owns("kubectl", { "f:requests": { "f:memory": {} } }),
        ],
        { requests: { cpu: "2" } },
        { requests: { memory: "1Gi" } },
      );
      click("Reset the resources");
      openReview();
      await waitFor(() => expect(push().disabled).toBe(false));
      expect(dialog()!.textContent).not.toContain("zeebe.resources: ");
    });

    it("refuses a reset of the block when a request of another tool stays above the limit of the preset", async () => {
      gitPage(
        { requests: { cpu: "2" }, limits: { cpu: "4" } },
        [
          owns("kubectl", { "f:requests": { "f:cpu": {} } }),
          owns("argocd-controller", { "f:limits": { "f:cpu": {} } }, true),
        ],
        { limits: { cpu: "1" } },
        { requests: { cpu: "2" } },
      );
      click("Reset the resources");
      openReview();
      await waitFor(() => expect(fakeWriter.previewed).toHaveLength(1));
      expect(within(dialog()!).getByRole("alert").textContent).toContain(
        `zeebe.resources: The CPU limit of 1 comes from the preset and is below the CPU request of 2. ${KEPT}`,
      );
      expect(push().disabled).toBe(true);
    });
  });

  it("a claim update that changes a field with a pending change keeps the pending value and shows the new live value before it", async () => {
    const { update } = page();
    click("Scale");
    update(
      slotContext({
        claim: claim("42", { cluster: { zeebe: { replicas: 4 } } }),
      }),
    );

    expect(text("Pending 1")).toBeTruthy();
    openReview();
    const diff = within(dialog()!).getByRole("figure", {
      name: "Changes to the claim",
    });
    expect(diff.querySelector("del")?.textContent).toContain("replicas: 4");
    expect(diff.querySelector("ins")?.textContent).toContain("replicas: 5");
  });

  it.each(["before", "after"] as const)(
    "an ok save leaves no pending change for a value that the server defaulted, with the claim update %s the save returns",
    async (order) => {
      const { actions, resolve } = delayedActions();
      const { update } = page(slotContext(), actions);
      click("Attach schedule");
      openReview();
      await confirm("Save");
      await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
      const delivered = slotContext({
        claim: claim("42", {
          attachments: {
            backupSchedule: { cron: "0 1 * * *", keepCompleted: 3 },
          },
        }),
      });

      if (order === "before") update(delivered);
      await act(async () => resolve());
      if (order === "after") update(delivered);

      await waitFor(() => expect(dialog()).toBeNull());
      expect(text("Pending 0")).toBeTruthy();
    },
  );

  it("a Stale write shows the claim text and blocks Save until the claim updates", async () => {
    fakeWriter.results = [
      { ok: false, errors: [{ code: "Stale", message: "x" }] },
    ];
    const { update } = page();
    click("Scale");
    openReview();
    await confirm("Save");

    await waitFor(() =>
      expect(within(dialog()!).getByRole("alert").textContent).toContain(
        STALE_CLAIM_MESSAGE,
      ),
    );
    const save = () =>
      within(dialog()!).getByRole<HTMLButtonElement>("button", {
        name: "Save",
      });
    expect(save().disabled).toBe(true);

    update(slotContext({ claim: claim("42") }));

    await waitFor(() => expect(save().disabled).toBe(false));
    expect(
      within(dialog()!).getByText(
        "The claim changed while the save ran. Review the change and save again.",
      ),
    ).toBeTruthy();
  });

  it("a Conflict write shows the message of the writer and keeps it after the claim updates", async () => {
    const message =
      "clusters/team-a/orders.yaml holds a save that GitOps has not applied yet.";
    fakeWriter.results = [
      { ok: false, errors: [{ code: "Conflict", message }] },
    ];
    const { update } = page();
    click("Scale");
    openReview();
    await confirm("Save");
    await waitFor(() =>
      expect(within(dialog()!).getByRole("alert").textContent).toContain(
        message,
      ),
    );
    const save = within(dialog()!).getByRole<HTMLButtonElement>("button", {
      name: "Save",
    });
    expect(save.disabled).toBe(false);

    update(slotContext({ claim: claim("42") }));

    await waitFor(() => expect(fakeWriter.previewed).toHaveLength(2));
    expect(within(dialog()!).getByText(message)).toBeTruthy();
  });

  it("a claim update during the write keeps the refusal of the write in the review", async () => {
    fakeWriter.results = [
      { ok: false, errors: [{ code: "Stale", message: "x" }] },
    ];
    const { actions, resolve } = delayedActions();
    const { update } = page(slotContext(), actions);
    click("Scale");
    openReview();
    await confirm("Save");
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));

    update(slotContext({ claim: claim("42") }));
    await act(async () => resolve());

    expect(
      within(dialog()!).getByText(
        "The claim changed while the save ran. Review the change and save again.",
      ),
    ).toBeTruthy();
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

  it("a downgrade asks for the downgrade confirm", async () => {
    page();
    click("Downgrade");
    openReview();
    expect(
      within(dialog()!).getByRole("note", { name: "Caution" }).textContent,
    ).toContain("This change moves the cluster from 8.9.1 to 8.9.0.");
    await waitFor(() =>
      expect(
        within(dialog()!)
          .getByRole("button", { name: "Downgrade" })
          .getAttribute("data-variant"),
      ).toBe("destructive"),
    );
    cleanup();
    fakeWriter.reset("git");
    page(slotContext({ writeMode: "git" }));
    click("Downgrade");
    openReview();
    expect(
      within(dialog()!).getByRole("button", { name: "Push downgrade" }),
    ).toBeTruthy();
  });

  it("the next save adds the reset of allowVersionDowngrade as a diff line and writes it", async () => {
    page(
      slotContext({ claim: claim("41", { allowVersionDowngrade: "8.9.1" }) }),
    );
    click("Suspend");
    openReview();
    expect(removedLines()).toEqual(["allowVersionDowngrade: 8.9.1"]);
    await confirm("Save");
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
    const spec = (
      fakeWriter.applied[0].apply[0] as Resource<"CamundaClusterClaim">
    ).spec;
    expect(spec.allowVersionDowngrade).toBeUndefined();
    expect(spec.suspend).toBe(true);
  });

  it("keeps allowVersionDowngrade while the cluster runs another version", () => {
    page(
      slotContext({ claim: claim("41", { allowVersionDowngrade: "8.9.0" }) }),
    );
    click("Suspend");
    openReview();
    expect(removedLines()).toEqual([]);
  });

  it("opening the page writes nothing when allowVersionDowngrade can go", async () => {
    page(
      slotContext({ claim: claim("41", { allowVersionDowngrade: "8.9.1" }) }),
    );
    await act(async () => {});
    expect(text("Pending 0")).toBeTruthy();
    expect(bar()).toBeNull();
    expect(fakeWriter.previewed).toEqual([]);
    expect(fakeWriter.applied).toEqual([]);
  });

  it("closing the review gives the focus to Discard while Review is disabled", async () => {
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
    fireEvent.click(within(dialog()!).getByRole("button", { name: "Back" }));
    await waitFor(() => expect(dialog()).toBeNull());
    expect(document.activeElement).toBe(
      within(bar()!).getByRole("button", { name: "Discard" }),
    );
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
