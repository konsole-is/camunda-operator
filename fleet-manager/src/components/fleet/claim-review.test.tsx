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
import { useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  applyClaimChange,
  previewClaimChange,
} from "@/app/fleet/clusters/[namespace]/[name]/actions";
import type { Step } from "@/lib/fields";
import { CLAIM_LABEL } from "@/lib/labels";
import {
  claimAfter,
  openSession,
  resetValue,
  setValue,
  STALE_CLAIM_MESSAGE,
  type ClaimObject,
  type Session,
} from "@/lib/session";
import type { KubeObject, WriteMode } from "@/lib/write/types";
import { FakeEventSource } from "../../../test/fakes/event-source";
import { fakeWriter } from "../../../test/fakes/writer";
import { ClaimReview } from "./claim-review";
import { GitPushesProvider } from "./git-pushes";
import { Toast, useWriteToast } from "./toast";

vi.mock("@/lib/write/writer", () => import("../../../test/fakes/writer"));

const INTENT = "Scale the brokers of orders";
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

const claim: ClaimObject = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaClusterClaim",
  metadata: { name: "orders", namespace: "team-a", resourceVersion: "41" },
  spec: { cluster: { zeebe: { replicas: 3, partitions: 3 } } },
};

function session(change: (s: Session) => Session = scaled): Session {
  const s = openSession({
    claim,
    resource: { metadata: { labels: { [CLAIM_LABEL]: "orders" } } },
    locks: () => undefined,
  })!;
  return change(s);
}

function scaled(s: Session): Session {
  const result = setValue(s, replicas, 5);
  if (!("session" in result)) throw new Error("refused");
  return result.session;
}

function resetPartitions(s: Session): Session {
  const result = resetValue(s, partitions);
  if (!("session" in result)) throw new Error("refused");
  return result.session;
}

type Props = Partial<Parameters<typeof ClaimReview>[0]> & { mode?: WriteMode };

// Page owns the toast, as a page does, and closes the review after a write.
function Page(props: Props) {
  const { toast, show, dismiss } = useWriteToast();
  const [open, setOpen] = useState(true);
  return (
    <>
      {open && (
        <ClaimReview
          session={session()}
          mode={fakeWriter.mode}
          intent={INTENT}
          impacts={[]}
          preview={previewClaimChange}
          apply={applyClaimChange}
          show={show}
          {...props}
          onWritten={() => {
            props.onWritten?.();
            setOpen(false);
          }}
        />
      )}
      {toast && <Toast result={toast} onDismiss={dismiss} />}
    </>
  );
}

function review(props: Props = {}) {
  FakeEventSource.instances = [];
  vi.stubGlobal("EventSource", FakeEventSource);
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <GitPushesProvider>
        <Page {...props} />
      </GitPushesProvider>
    </QueryClientProvider>,
  );
}

// confirm waits until the preview has run and the confirm can be clicked.
async function confirm(label: string): Promise<HTMLButtonElement> {
  const button = screen.getByRole<HTMLButtonElement>("button", { name: label });
  await waitFor(() => expect(button.disabled).toBe(false));
  return button;
}

beforeEach(() => fakeWriter.reset());
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("ClaimReview", () => {
  it("shows the caution when given, and no caution otherwise", () => {
    const { unmount } = review();
    expect(screen.queryByRole("note", { name: "Caution" })).toBeNull();
    unmount();
    review({
      caution: "A backup of orders runs now. The rollout makes it fail.",
    });
    expect(screen.getByRole("note", { name: "Caution" }).textContent).toContain(
      "A backup of orders runs now.",
    );
  });

  it("direct mode applies exactly the changed claim with the precondition", async () => {
    const onWritten = vi.fn();
    review({ onWritten });

    fireEvent.click(await confirm("Save"));

    await waitFor(() => expect(onWritten).toHaveBeenCalledOnce());
    // The field manager is the default of the direct writer, which
    // test/k8s/writer.test.ts "an apply sets the field manager
    // fleet-manager on the object" proves.
    expect(fakeWriter.applied).toEqual([
      {
        intent: INTENT,
        apply: [claimAfter(session())],
        preconditions: { "CamundaClusterClaim/team-a/orders": "41" },
      },
    ]);
    expect(
      (fakeWriter.applied[0].apply[0] as KubeObject & typeof claim).spec,
    ).toEqual({ cluster: { zeebe: { replicas: 5, partitions: 3 } } });
    expect(screen.getByRole("status").textContent).toContain(
      "The claim orders is saved.",
    );
  });

  it("Git mode commits exactly the changed claim with the precondition", async () => {
    fakeWriter.reset("git");
    const onWritten = vi.fn();
    review({ onWritten });

    fireEvent.click(await confirm("Push to Git"));

    await waitFor(() => expect(onWritten).toHaveBeenCalledOnce());
    expect(fakeWriter.applied).toEqual([
      {
        intent: INTENT,
        apply: [claimAfter(session())],
        preconditions: { "CamundaClusterClaim/team-a/orders": "41" },
      },
    ]);
    expect(screen.getByRole("status").textContent).toContain(
      "Waiting for GitOps to apply",
    );
  });

  it.each(["direct", "git"] as const)(
    "Stale in %s mode shows the claim text, keeps the pending changes, and writes nothing",
    async (mode) => {
      fakeWriter.reset(mode);
      fakeWriter.results = [
        { ok: false, errors: [{ code: "Stale", message: "x" }] },
      ];
      const s = session();
      const onWritten = vi.fn();
      review({ session: s, onWritten });

      fireEvent.click(await confirm(mode === "git" ? "Push to Git" : "Save"));

      await waitFor(() =>
        expect(screen.getByRole("alert").textContent).toContain(
          STALE_CLAIM_MESSAGE,
        ),
      );
      expect(screen.getByRole("alert").textContent).not.toContain("x");
      expect(onWritten).not.toHaveBeenCalled();
      expect([...s.pending.keys()]).toEqual(["spec.cluster.zeebe.replicas"]);
      // The writer refused, so the one record is the refused attempt, and
      // no commit and no toast follow it.
      expect(fakeWriter.applied).toHaveLength(1);
      expect(screen.queryByRole("status")).toBeNull();
    },
  );

  it("shows a CEL refusal of the dry run and disables the confirm", async () => {
    fakeWriter.previews = [
      {
        objects: [],
        errors: [
          {
            code: "Invalid",
            message: "spec.cluster: presetRef cannot be set in spec.cluster",
            path: "spec.cluster",
          },
        ],
      },
    ];
    const onConfirm = vi.fn();
    review({ apply: onConfirm });

    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toContain(
        "presetRef cannot be set in spec.cluster",
      ),
    );
    const button = screen.getByRole<HTMLButtonElement>("button", {
      name: "Save",
    });
    expect(button.disabled).toBe(true);
    fireEvent.click(button);
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it("a dry-run refusal with a field path goes to onRefused", async () => {
    fakeWriter.previews = [
      {
        objects: [],
        errors: [
          {
            code: "Invalid",
            message: "Invalid value: 0: must be at least 1",
            path: "spec.cluster.zeebe.replicas",
          },
        ],
      },
    ];
    const onRefused = vi.fn();
    review({ onRefused });

    await waitFor(() =>
      expect(onRefused).toHaveBeenCalledWith([
        {
          path: "spec.cluster.zeebe.replicas",
          message: "Invalid value: 0: must be at least 1",
        },
      ]),
    );
    expect(onRefused).toHaveBeenCalledOnce();
  });

  it("an API refusal with a field path goes to onRefused", async () => {
    fakeWriter.results = [
      {
        ok: false,
        errors: [
          {
            code: "Invalid",
            message: "Invalid value: 0: must be at least 1",
            path: "spec.cluster.zeebe.replicas",
          },
        ],
      },
    ];
    const onRefused = vi.fn();
    review({ onRefused });

    fireEvent.click(await confirm("Save"));

    await waitFor(() =>
      expect(onRefused).toHaveBeenCalledWith([
        {
          path: "spec.cluster.zeebe.replicas",
          message: "Invalid value: 0: must be at least 1",
        },
      ]),
    );
    expect(screen.getByRole("alert").textContent).toContain(
      "spec.cluster.zeebe.replicas: Invalid value: 0: must be at least 1",
    );
  });

  it("sends one write for two clicks", async () => {
    const apply = vi.fn(applyClaimChange);
    review({ apply });

    const button = await confirm("Save");
    // One act holds back the render that disables the button, so the second
    // click reaches the handler.
    act(() => {
      button.click();
      button.click();
    });

    await waitFor(() =>
      expect(screen.getByRole("status").textContent).toContain("saved"),
    );
    expect(apply).toHaveBeenCalledOnce();
    expect(fakeWriter.applied).toHaveLength(1);
  });

  it("shows a note when the preview keeps a reset field", async () => {
    const s = session(resetPartitions);
    // Another field manager owns partitions, so the dry run keeps it.
    fakeWriter.previews = [
      {
        objects: [
          {
            ref: {
              apiVersion: claim.apiVersion,
              kind: claim.kind,
              namespace: "team-a",
              name: "orders",
            },
            after: claim as KubeObject,
            diff: [],
          },
        ],
        errors: [],
      },
    ];
    review({ session: s });

    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toContain(
        "Another field manager keeps spec.cluster.zeebe.partitions, so the reset does not remove it.",
      ),
    );
    expect((await confirm("Save")).disabled).toBe(false);
  });

  it("shows a note when the preview keeps a reset field of a list item named by index", async () => {
    const env = { name: "TZ", value: "UTC" };
    const withEnv: ClaimObject = {
      ...claim,
      spec: { cluster: { zeebe: { replicas: 3 }, extraEnv: [env] } },
    };
    const s = openSession({
      claim: withEnv,
      resource: { metadata: { labels: { [CLAIM_LABEL]: "orders" } } },
      locks: () => undefined,
    })!;
    const result = resetValue(s, [
      { field: "spec" },
      { field: "cluster" },
      { field: "extraEnv" },
      { index: 0 },
      { field: "value" },
    ]);
    if (!("session" in result)) throw new Error("refused");
    fakeWriter.previews = [
      {
        objects: [
          {
            ref: {
              apiVersion: claim.apiVersion,
              kind: claim.kind,
              namespace: "team-a",
              name: "orders",
            },
            after: withEnv as KubeObject,
            diff: [],
          },
        ],
        errors: [],
      },
    ];
    review({ session: result.session });

    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toContain(
        "Another field manager keeps spec.cluster.extraEnv[0].value, so the reset does not remove it.",
      ),
    );
  });

  it("shows a note when the preview keeps a reset list item without keys", async () => {
    const withEnv: ClaimObject = {
      ...claim,
      spec: {
        cluster: {
          zeebe: { replicas: 3 },
          extraEnvFrom: [{ configMapRef: { name: "a" } }],
        },
      },
    };
    const s = openSession({
      claim: withEnv,
      resource: { metadata: { labels: { [CLAIM_LABEL]: "orders" } } },
      locks: () => undefined,
    })!;
    const result = resetValue(s, [
      { field: "spec" },
      { field: "cluster" },
      { field: "extraEnvFrom" },
      { index: 0 },
    ]);
    if (!("session" in result)) throw new Error("refused");
    fakeWriter.previews = [
      {
        objects: [
          {
            ref: {
              apiVersion: claim.apiVersion,
              kind: claim.kind,
              namespace: "team-a",
              name: "orders",
            },
            after: withEnv as KubeObject,
            diff: [],
          },
        ],
        errors: [],
      },
    ];
    review({ session: result.session });

    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toContain(
        "Another field manager keeps spec.cluster.extraEnvFrom[0], so the reset does not remove it.",
      ),
    );
  });

  it("shows no note when the preview removes a reset field", async () => {
    review({ session: session(resetPartitions) });

    await confirm("Save");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("shows the impacts and the merged diff", async () => {
    review({
      impacts: ["Broker pods restart."],
      mergedLines: [
        { op: "removed", text: "replicas: 3" },
        { op: "added", text: "replicas: 5" },
      ],
    });

    expect(screen.getByText("Broker pods restart.")).toBeTruthy();
    expect(
      screen.getByRole("figure", { name: "Changes to the cluster" })
        .textContent,
    ).toContain("replicas: 5");
    expect(
      screen.getByRole("figure", { name: "Changes to the claim" }).textContent,
    ).toContain("replicas: 5");
    await confirm("Save");
  });

  it("previews the change of the session with its precondition", async () => {
    review();

    await confirm("Save");
    expect(fakeWriter.previewed).toEqual([
      {
        intent: INTENT,
        apply: [claimAfter(session())],
        preconditions: { "CamundaClusterClaim/team-a/orders": "41" },
      },
    ]);
  });
});
