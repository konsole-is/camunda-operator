// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { pathFrom, type Step } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL, REVISION_ANNOTATION } from "@/lib/labels";
import { claimAfter, setValue, valueAt } from "@/lib/session";
import type { Change, KubeObject, WriteResult } from "@/lib/write/types";
import { FakeEventSource } from "../../../test/fakes/event-source";
import {
  CHANGES_OFF,
  ClaimEditProvider,
  useClaimEdit,
  type ClaimActions,
  type ClaimEditHost,
} from "./claim-edit";
import { GitPushesProvider } from "./git-pushes";

type PlaneClaim = Resource<"CamundaManagementClusterClaim">;

const suspend: Step[] = [{ field: "spec" }, { field: "suspend" }];
const identity: Step[] = [
  { field: "spec" },
  { field: "versions" },
  { field: "identity" },
];
const KEY = "CamundaManagementClusterClaim/platform/hub";

// planeClaim is the plane claim hub in platform at resourceVersion.
function planeClaim(
  resourceVersion = "7",
  over: { spec?: object; revision?: string } = {},
): PlaneClaim {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: {
      namespace: "platform",
      name: "hub",
      resourceVersion,
      ...(over.revision && {
        annotations: { [REVISION_ANNOTATION]: over.revision },
      }),
    },
    spec: {
      environments: ["prod"],
      management: {},
      versions: { identity: "8.9.10" },
      ...over.spec,
    },
  } as unknown as PlaneClaim;
}

const plane = { metadata: { labels: { [CLAIM_LABEL]: "hub" } } };

function host(
  over: Partial<ClaimEditHost<PlaneClaim>> = {},
): ClaimEditHost<PlaneClaim> {
  return {
    claim: planeClaim(),
    resource: plane,
    writeMode: "direct",
    locks: () => undefined,
    validate: () => [],
    review: () => ({ impacts: ["Sign-in stops while Identity restarts."] }),
    ...over,
  };
}

// fakeActions previews clean and answers each write with result.
function fakeActions(result: WriteResult = { ok: true, mode: "direct" }) {
  const previewed: Change[] = [];
  const applied: Change[] = [];
  const actions: ClaimActions = {
    preview: async (change) => {
      previewed.push(change);
      return { objects: [], errors: [] };
    },
    apply: async (change) => {
      applied.push(change);
      return result;
    },
  };
  return { actions, previewed, applied };
}

const pushed: WriteResult = {
  ok: true,
  mode: "git",
  commit: "abc1234def",
  revisions: { [KEY]: "rev-1" },
  paths: { [KEY]: "planes/hub.yaml" },
  deleted: {},
};

// Probe is an editor: it changes the claim only through useClaimEdit.
function Probe() {
  const edit = useClaimEdit<PlaneClaim>();
  const [refusal, setRefusal] = useState<string>();
  const [opened, setOpened] = useState<boolean>();
  const after = edit.session ? claimAfter(edit.session) : undefined;
  return (
    <div>
      <p>Opened {opened === undefined ? "unknown" : opened ? "yes" : "no"}</p>
      <p>Session {edit.session ? "open" : "none"}</p>
      <p>Pending {edit.pending}</p>
      <p>Refusal {refusal ?? "none"}</p>
      <p>Suspended {String(valueAt(after, suspend) ?? false)}</p>
      <p>Identity {String(valueAt(after, identity))}</p>
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
      <button onClick={() => setRefusal(edit.set(identity, "8.9.11")?.reason)}>
        Identity 8.9.11
      </button>
      <button onClick={() => setOpened(edit.review())}>Try review</button>
    </div>
  );
}

function page(
  h: ClaimEditHost<PlaneClaim>,
  actions: ClaimActions = fakeActions().actions,
) {
  const client = new QueryClient();
  let mount = 0;
  let last = h;
  const tree = (next: ClaimEditHost<PlaneClaim>) => (
    <QueryClientProvider client={client}>
      <GitPushesProvider>
        <ClaimEditProvider key={mount} host={next} actions={actions}>
          <Probe />
        </ClaimEditProvider>
      </GitPushesProvider>
    </QueryClientProvider>
  );
  const result = render(tree(h));
  return {
    ...result,
    update: (next: ClaimEditHost<PlaneClaim>) => {
      last = next;
      result.rerender(tree(next));
    },
    // remount mounts a new host in the same tab, as when the user leaves
    // the plane page and comes back.
    remount: () => {
      mount++;
      result.rerender(tree(last));
    },
  };
}

const click = (name: string) =>
  fireEvent.click(screen.getByRole("button", { name }));
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
  FakeEventSource.instances = [];
  vi.stubGlobal("EventSource", FakeEventSource);
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("ClaimEditProvider", () => {
  it("opens no session without a write mode or without the managed resource", () => {
    for (const h of [
      host({ writeMode: undefined }),
      host({ resource: undefined }),
    ]) {
      const { unmount } = page(h);
      expect(screen.queryByText("Session none")).toBeTruthy();
      click("Suspend");
      expect(screen.queryByText("Pending 0")).toBeTruthy();
      expect(bar()).toBeNull();
      unmount();
    }
    page(host({ writeMode: undefined }));
    expect(screen.getByRole("alert").textContent).toBe(CHANGES_OFF);
  });

  it("set records a pending change of the plane claim and the change bar counts it", () => {
    page(host());
    expect(screen.queryByText("Session open")).toBeTruthy();
    click("Suspend");
    expect(screen.queryByText("Pending 1")).toBeTruthy();
    expect(screen.queryByText("Suspended true")).toBeTruthy();
    expect(bar()).toBeTruthy();
  });

  it("set returns the lock of the host and changes nothing", () => {
    const locked = pathFrom("", identity);
    page(
      host({
        locks: (p) => (p === locked ? { reason: "atCreation" } : undefined),
      }),
    );
    click("Identity 8.9.11");
    expect(screen.queryByText("Refusal atCreation")).toBeTruthy();
    expect(screen.queryByText("Pending 0")).toBeTruthy();
  });

  it("a lock provider of a later render applies to the next change", () => {
    const { update } = page(host());
    const locked = pathFrom("", suspend);
    update(
      host({
        locks: (p) => (p === locked ? { reason: "atCreation" } : undefined),
      }),
    );
    click("Suspend");
    expect(screen.queryByText("Refusal atCreation")).toBeTruthy();
    expect(screen.queryByText("Pending 0")).toBeTruthy();
  });

  it("an error of validate blocks the review and shows on the change bar", () => {
    page(
      host({
        validate: (s) =>
          s.pending.size > 0
            ? [{ path: "spec.suspend", message: "Suspend is off here." }]
            : [],
      }),
    );
    click("Suspend");
    click("Try review");
    expect(screen.queryByText("Opened no")).toBeTruthy();
    expect(dialog()).toBeNull();
    expect(
      screen.getByRole("list", { name: "Field errors" }).textContent,
    ).toContain("spec.suspend: Suspend is off here.");
    expect(within(bar()!).getByText("Fix 1 field before review.")).toBeTruthy();
  });

  it("never previews or writes before the review opens", async () => {
    const { actions, previewed, applied } = fakeActions();
    page(host(), actions);
    click("Suspend");
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(previewed).toEqual([]);
    expect(applied).toEqual([]);
  });

  it("the review shows the impacts and the caution of the host, and the confirm writes the change with the precondition", async () => {
    const { actions, applied } = fakeActions();
    page(
      host({
        review: () => ({
          impacts: ["Sign-in stops while Identity restarts."],
          caution: <p>Console stops pinging the clusters.</p>,
          confirm: { label: "Change plane" },
        }),
      }),
      actions,
    );
    click("Suspend");
    openReview();
    const impacts = within(dialog()!).getByRole("list", {
      name: "What this change does",
    });
    expect(impacts.textContent).toContain(
      "Sign-in stops while Identity restarts.",
    );
    expect(
      within(dialog()!).getByText("Console stops pinging the clusters."),
    ).toBeTruthy();
    await confirm("Change plane");

    await waitFor(() => expect(dialog()).toBeNull());
    expect(applied).toHaveLength(1);
    const written = applied[0].apply[0] as KubeObject & {
      spec: { suspend?: boolean };
    };
    expect(written.kind).toBe("CamundaManagementClusterClaim");
    expect(written.spec.suspend).toBe(true);
    expect(applied[0].preconditions).toEqual({ [KEY]: "7" });
    expect(applied[0].intent).toBe("Change the claim hub");
    expect(screen.getByRole("status").textContent).toContain(
      "The claim hub is saved.",
    );
    expect(screen.queryByText("Pending 0")).toBeTruthy();
  });

  it("prepare changes the session when Review is clicked, and the write carries its change", async () => {
    const { actions, applied } = fakeActions();
    page(
      host({
        prepare: (s) => {
          const r = setValue(s, identity, "8.9.11");
          return "session" in r ? r.session : s;
        },
      }),
      actions,
    );
    click("Suspend");
    expect(screen.queryByText("Identity 8.9.10")).toBeTruthy();
    openReview();
    await confirm("Save");

    await waitFor(() => expect(applied).toHaveLength(1));
    expect(valueAt(applied[0].apply[0], identity)).toBe("8.9.11");
    expect(valueAt(applied[0].apply[0], suspend)).toBe(true);
  });

  it("the close function that review gets closes the review", () => {
    page(
      host({
        review: (_s, { close }) => ({
          impacts: [],
          caution: <button onClick={close}>Open backups</button>,
        }),
      }),
    );
    click("Suspend");
    openReview();
    fireEvent.click(
      within(dialog()!).getByRole("button", { name: "Open backups" }),
    );
    expect(dialog()).toBeNull();
    expect(screen.queryByText("Pending 1")).toBeTruthy();
  });

  it("in Git mode, Review waits until the plane claim shows the revision of the push", async () => {
    const { actions } = fakeActions(pushed);
    const { update } = page(host({ writeMode: "git" }), actions);
    click("Suspend");
    openReview();
    await confirm("Push to Git");
    await waitFor(() => expect(dialog()).toBeNull());
    expect(screen.queryByText("Suspended true")).toBeTruthy();
    click("Identity 8.9.11");

    const review = () =>
      within(bar()!).getByRole<HTMLButtonElement>("button", { name: "Review" });
    expect(review().disabled).toBe(true);
    expect(
      within(bar()!).getByText("Waiting for GitOps to apply your last change."),
    ).toBeTruthy();

    update(
      host({
        writeMode: "git",
        claim: planeClaim("8", { spec: { suspend: true }, revision: "rev-1" }),
      }),
    );

    expect(review().disabled).toBe(false);
    expect(screen.queryByText("Pending 1")).toBeTruthy();
    expect(screen.queryByText("Suspended true")).toBeTruthy();
  });

  it("in Git mode, a new host of a plane claim with a waiting push blocks Review and shows the live claim", async () => {
    const { actions } = fakeActions(pushed);
    const { remount } = page(host({ writeMode: "git" }), actions);
    click("Suspend");
    openReview();
    await confirm("Push to Git");
    await waitFor(() => expect(dialog()).toBeNull());

    remount();
    click("Identity 8.9.11");

    expect(screen.queryByText("Suspended false")).toBeTruthy();
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

  it("useClaimEdit throws without a provider", () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => render(<Probe />)).toThrow();
  });
});
