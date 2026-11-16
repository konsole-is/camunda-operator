// @vitest-environment jsdom
import { createHash } from "node:crypto";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  applyClaimChange,
  applyClaimDelete,
  previewClaimDelete,
} from "@/app/fleet/clusters/[namespace]/[name]/actions";
import { watchId } from "@/lib/k8s/shared-watch";
import { REVISION_ANNOTATION } from "@/lib/labels";
import type { DeletionPolicy } from "@/lib/policy";
import type { WriteMode, WriteResult } from "@/lib/write/types";
import { FakeEventSource } from "../../../test/fakes/event-source";
import { fakeWriter } from "../../../test/fakes/writer";
import { GitPushesProvider } from "./git-pushes";
import { type ClaimDeleteActions, useClaimDelete } from "./use-claim-delete";

vi.mock("@/lib/write/writer", () => import("../../../test/fakes/writer"));

const KIND = "CamundaManagementClusterClaim";
const key = `${KIND}/platform/plane-a` as const;
const ref = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: KIND,
  namespace: "platform",
  name: "plane-a",
};
type Claim = {
  apiVersion: string;
  kind: string;
  metadata: {
    namespace: string;
    name: string;
    resourceVersion: string;
    uid: string;
    [key: string]: unknown;
  };
  spec: { releaseRef: string; deletionPolicy?: DeletionPolicy };
  status: { resolvedPolicy: { deletionPolicy: DeletionPolicy } };
};
const claim = (
  over: { metadata?: object; spec?: { deletionPolicy?: DeletionPolicy } } = {},
): Claim => ({
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: KIND,
  metadata: {
    namespace: "platform",
    name: "plane-a",
    resourceVersion: "41",
    uid: "uid-1",
    ...over.metadata,
  },
  spec: { releaseRef: "camunda-8-9-1", ...over.spec },
  status: { resolvedPolicy: { deletionPolicy: "Delete" } },
});

const actions: ClaimDeleteActions = {
  preview: previewClaimDelete,
  remove: applyClaimDelete,
  applyPolicy: applyClaimChange,
};
const removeOf = (resourceVersion = "41") => ({
  intent: "Delete the plane plane-a",
  apply: [],
  delete: [ref],
  uids: { [key]: "uid-1" },
  preconditions: { [key]: resourceVersion },
});

// read is the claim as the claim controller leaves it after it read the
// deletion policy: generation 2 is observed.
const read = (resourceVersion: string, policy: DeletionPolicy = "Orphan") => {
  const c = claim({
    metadata: { resourceVersion, generation: 2 },
    spec: { deletionPolicy: policy },
  });
  return {
    ...c,
    status: { ...c.status, observedGeneration: 2 },
  } as unknown as Claim;
};

function mount(
  initial: Claim,
  props: {
    policy?: DeletionPolicy;
    mode?: WriteMode;
    actions?: Partial<ClaimDeleteActions>;
  } = {},
) {
  const shown: [WriteResult, string][] = [];
  const client = new QueryClient();
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>
      <GitPushesProvider>{children}</GitPushesProvider>
    </QueryClientProvider>
  );
  const hook = renderHook(
    (p: { policy: DeletionPolicy }) =>
      useClaimDelete({
        initial,
        policy: p.policy,
        mode: props.mode ?? "direct",
        intent: "Delete the plane plane-a",
        pushedMessage: "Pushed the delete of plane-a.",
        actions: { ...actions, ...props.actions },
        show: (result, message) => shown.push([result, message]),
      }),
    { wrapper, initialProps: { policy: props.policy ?? "Delete" } },
  );
  return { hook, shown };
}

const watch = watchId({ kind: KIND, namespace: "platform", name: "plane-a" });

// live sends the claim on the watch of the claim as a snapshot, an empty
// one when it is undefined.
async function live(c: Claim | undefined) {
  await waitFor(() =>
    expect(FakeEventSource.instances.at(-1)?.ids()).toContain(watch),
  );
  FakeEventSource.instances
    .at(-1)!
    .emit("snapshot", { watch, kind: KIND, objects: c ? [c] : [] });
  await act(async () => {});
}

// confirm calls the apply of the hook as the review does, then onWritten
// on an ok result.
async function confirm(hook: ReturnType<typeof mount>["hook"]) {
  const { apply, remove, onWritten } = hook.result.current;
  const result = await act(() => apply(remove));
  if (result.ok) act(() => onWritten());
  return result;
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

describe("useClaimDelete", () => {
  it("gives the remove step of the live claim, a new one for each claim version and each pick", async () => {
    const { hook } = mount(claim());
    const first = hook.result.current.remove;
    expect(first).toEqual(removeOf());
    hook.rerender({ policy: "Orphan" });
    const picked = hook.result.current.remove;
    expect(picked).not.toBe(first);
    expect(picked).toEqual(removeOf());
    expect(hook.result.current.policyStep).toBe(true);
    await live(claim({ metadata: { resourceVersion: "42" } }));
    await waitFor(() => expect(hook.result.current.remove).not.toBe(picked));
    expect(hook.result.current.remove).toEqual(removeOf("42"));
  });

  it("reads holds the claim kind while its watch has no snapshot, and the error of a failed watch", async () => {
    const { hook } = mount(claim());
    expect(hook.result.current.reads).toEqual({ errors: [], stale: [KIND] });
    await live(claim());
    await waitFor(() =>
      expect(hook.result.current.reads).toEqual({ errors: [], stale: [] }),
    );
    act(() =>
      FakeEventSource.instances
        .at(-1)!
        .emit("failure", { watch, kind: KIND, message: "lost" }),
    );
    await waitFor(() =>
      expect(hook.result.current.reads.errors).toEqual([
        { kind: KIND, message: "lost" },
      ]),
    );
  });

  it("direct mode without a policy step sends the delete with the UID and the resourceVersion", async () => {
    const { hook } = mount(claim());
    expect(await confirm(hook)).toEqual({ ok: true, mode: "direct" });
    expect(fakeWriter.applied).toEqual([removeOf()]);
    expect(hook.result.current.stage).toEqual({
      kind: "deleting",
      gitWaiting: false,
    });
  });

  it("direct mode writes the policy, waits until the claim controller read it, then deletes with the new resourceVersion", async () => {
    const { hook, shown } = mount(claim(), { policy: "Orphan" });
    expect(hook.result.current.stage).toEqual({ kind: "choose" });
    expect(await confirm(hook)).toEqual({ ok: true, mode: "direct" });
    expect(fakeWriter.previewed).toEqual([removeOf()]);
    expect(fakeWriter.applied).toHaveLength(1);
    expect(fakeWriter.applied[0]).toMatchObject({
      apply: [{ kind: KIND, spec: { deletionPolicy: "Orphan" } }],
      preconditions: { [key]: "41" },
    });
    expect(hook.result.current.stage).toEqual({ kind: "waiting" });

    const unread = read("42");
    await live({
      ...unread,
      status: { ...unread.status, observedGeneration: 1 },
    } as unknown as Claim);
    await waitFor(() =>
      expect(hook.result.current.claim?.metadata.resourceVersion).toBe("42"),
    );
    expect(fakeWriter.applied).toHaveLength(1);

    await live(read("43"));
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(2));
    expect(fakeWriter.applied[1]).toEqual(removeOf("43"));
    await waitFor(() =>
      expect(hook.result.current.stage).toEqual({
        kind: "deleting",
        gitWaiting: false,
      }),
    );
    expect(shown).toEqual([]);
  });

  it("a delete refused after the policy write keeps its errors and the written policy, also when the claim changed first", async () => {
    const { hook } = mount(claim(), { policy: "Orphan" });
    await confirm(hook);
    fakeWriter.results = [
      { ok: false, errors: [{ code: "Forbidden", message: "No delete." }] },
    ];
    await live(read("42"));
    await waitFor(() =>
      expect(hook.result.current.stage).toEqual({
        kind: "choose",
        errors: [{ code: "Forbidden", message: "No delete." }],
        policyWritten: "Orphan",
      }),
    );
    await live(read("44"));
    await waitFor(() =>
      expect(hook.result.current.claim?.metadata.resourceVersion).toBe("44"),
    );
    expect(hook.result.current.stage).toEqual({
      kind: "choose",
      errors: [{ code: "Forbidden", message: "No delete." }],
      policyWritten: "Orphan",
    });
    expect(fakeWriter.applied).toHaveLength(2);
  });

  it("refuses with Stale and sends no delete when the claim changed beyond the policy during the wait", async () => {
    const { hook } = mount(claim(), { policy: "Orphan" });
    await confirm(hook);
    const edited = read("42");
    edited.spec.releaseRef = "camunda-8-9-2";
    await live(edited);
    await waitFor(() =>
      expect(hook.result.current.stage).toEqual({
        kind: "choose",
        errors: [
          {
            code: "Stale",
            message:
              "The claim plane-a changed after the review. Review the delete again.",
          },
        ],
        policyWritten: "Orphan",
      }),
    );
    expect(fakeWriter.applied).toHaveLength(1);
  });

  it("refuses with Stale when another change takes the policy back during the wait", async () => {
    const { hook } = mount(claim(), { policy: "Orphan" });
    await confirm(hook);
    await live(read("42", "Delete"));
    await waitFor(() =>
      expect(hook.result.current.stage).toMatchObject({
        kind: "choose",
        errors: [{ code: "Stale" }],
        policyWritten: "Orphan",
      }),
    );
    expect(fakeWriter.applied).toHaveLength(1);
  });

  it("keeps waiting while the claim still shows the spec from before the write", async () => {
    fakeWriter.reset("git");
    const { hook } = mount(claim(), { policy: "Orphan", mode: "git" });
    await confirm(hook);
    await live(claim({ metadata: { resourceVersion: "42" } }));
    await waitFor(() =>
      expect(hook.result.current.claim?.metadata.resourceVersion).toBe("42"),
    );
    expect(hook.result.current.stage).toEqual({ kind: "waiting" });
  });

  it("leaves the wait before the delete returns, so that the wait cannot be stopped during the write", async () => {
    let finish: (result: WriteResult) => void = () => {};
    const remove = vi.fn(
      () => new Promise<WriteResult>((resolve) => (finish = resolve)),
    );
    const { hook } = mount(claim(), { policy: "Orphan", actions: { remove } });
    await confirm(hook);
    await live(read("42"));
    await waitFor(() => expect(remove).toHaveBeenCalledTimes(1));
    expect(hook.result.current.stage).toEqual({
      kind: "deleting",
      gitWaiting: false,
    });
    await act(async () => finish({ ok: true, mode: "direct" }));
    expect(hook.result.current.stage).toEqual({
      kind: "deleting",
      gitWaiting: false,
    });
  });

  it("returns a refused policy write and sends no delete", async () => {
    fakeWriter.results = [
      { ok: false, errors: [{ code: "Stale", message: "Changed." }] },
    ];
    const { hook } = mount(claim(), { policy: "Orphan" });
    expect(await confirm(hook)).toEqual({
      ok: false,
      errors: [{ code: "Stale", message: "Changed." }],
    });
    expect(fakeWriter.applied).toHaveLength(1);
    expect(hook.result.current.stage).toEqual({ kind: "choose" });
  });

  it("refuses with the errors of the preview of the delete and writes no policy", async () => {
    fakeWriter.reset("git");
    fakeWriter.previews = [
      {
        objects: [],
        errors: [{ code: "Invalid", message: "No file in the repository." }],
      },
    ];
    const { hook } = mount(claim(), { policy: "Orphan", mode: "git" });
    expect(await confirm(hook)).toEqual({
      ok: false,
      errors: [{ code: "Invalid", message: "No file in the repository." }],
    });
    expect(fakeWriter.applied).toEqual([]);
    expect(hook.result.current.stage).toEqual({ kind: "choose" });
  });

  it("Git mode pushes the policy, waits until the live claim carries it, then pushes the delete once", async () => {
    fakeWriter.reset("git");
    const { hook, shown } = mount(claim(), { policy: "Orphan", mode: "git" });
    await confirm(hook);
    expect(fakeWriter.applied).toHaveLength(1);
    expect(hook.result.current.stage).toEqual({ kind: "waiting" });
    const revision = createHash("sha1")
      .update(JSON.stringify(fakeWriter.applied[0].apply[0]))
      .digest("hex");
    const applied = read("42");
    applied.metadata.annotations = { [REVISION_ANNOTATION]: revision };
    await live(applied);
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(2));
    expect(fakeWriter.applied[1]).toEqual(removeOf("42"));
    expect(shown.at(-1)?.[1]).toBe("Pushed the delete of plane-a.");
    await waitFor(() =>
      expect(hook.result.current.stage).toEqual({
        kind: "deleting",
        gitWaiting: true,
      }),
    );
    await live({
      ...applied,
      metadata: { ...applied.metadata, resourceVersion: "43" },
    });
    await waitFor(() =>
      expect(hook.result.current.claim?.metadata.resourceVersion).toBe("43"),
    );
    expect(fakeWriter.applied).toHaveLength(2);
  });

  it("Git mode goes back to the choice with the refusal of the delete after the wait", async () => {
    fakeWriter.reset("git");
    const { hook } = mount(claim(), { policy: "Orphan", mode: "git" });
    await confirm(hook);
    const revision = createHash("sha1")
      .update(JSON.stringify(fakeWriter.applied[0].apply[0]))
      .digest("hex");
    fakeWriter.results = [
      { ok: false, errors: [{ code: "Invalid", message: "No file." }] },
    ];
    const applied = read("42");
    applied.metadata.annotations = { [REVISION_ANNOTATION]: revision };
    await live(applied);
    await waitFor(() =>
      expect(hook.result.current.stage).toEqual({
        kind: "choose",
        errors: [{ code: "Invalid", message: "No file." }],
        policyWritten: "Orphan",
      }),
    );
  });

  it("stop waiting goes back to the choice with the written policy and sends no delete", async () => {
    fakeWriter.reset("git");
    const { hook } = mount(claim(), { policy: "Orphan", mode: "git" });
    await confirm(hook);
    act(() => hook.result.current.stopWaiting());
    expect(hook.result.current.stage).toEqual({
      kind: "choose",
      policyWritten: "Orphan",
    });
    const revision = createHash("sha1")
      .update(JSON.stringify(fakeWriter.applied[0].apply[0]))
      .digest("hex");
    const applied = read("42");
    applied.metadata.annotations = { [REVISION_ANNOTATION]: revision };
    await live(applied);
    await waitFor(() =>
      expect(hook.result.current.claim?.metadata.resourceVersion).toBe("42"),
    );
    expect(fakeWriter.applied).toHaveLength(1);
  });

  it("sends no delete when a claim made again carries the policy", async () => {
    const { hook } = mount(claim(), { policy: "Orphan" });
    await confirm(hook);
    const again = read("42");
    again.metadata.uid = "uid-2";
    await live(again);
    await waitFor(() =>
      expect(hook.result.current.stage).toEqual({ kind: "replaced" }),
    );
    expect(fakeWriter.applied).toHaveLength(1);
  });

  it("a claim that is being deleted shows the delete, also when it starts while the page is open", async () => {
    const opened = mount(
      claim({ metadata: { deletionTimestamp: "2026-11-15T10:00:00Z" } }),
      { mode: "git" },
    );
    expect(opened.hook.result.current.stage).toEqual({
      kind: "deleting",
      gitWaiting: false,
    });
    opened.hook.unmount();

    FakeEventSource.instances = [];
    const open = mount(claim());
    expect(open.hook.result.current.stage).toEqual({ kind: "choose" });
    await live(
      claim({ metadata: { deletionTimestamp: "2026-11-15T10:00:00Z" } }),
    );
    await waitFor(() =>
      expect(open.hook.result.current.stage).toEqual({
        kind: "deleting",
        gitWaiting: false,
      }),
    );
  });

  it("a claim that is gone is deleted, and a claim made again under its name is replaced", async () => {
    const gone = mount(claim());
    await live(undefined);
    await waitFor(() =>
      expect(gone.hook.result.current.stage).toEqual({ kind: "deleted" }),
    );
    gone.hook.unmount();

    FakeEventSource.instances = [];
    const again = mount(claim());
    await live(claim({ metadata: { uid: "uid-2" } }));
    await waitFor(() =>
      expect(again.hook.result.current.stage).toEqual({ kind: "replaced" }),
    );
    expect(fakeWriter.applied).toEqual([]);
  });
});
