// @vitest-environment jsdom
import { createHash } from "node:crypto";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  applyPlaneClaimChange,
  applyPlaneClaimDelete,
  previewPlaneClaimDelete,
} from "@/app/fleet/planes/[namespace]/[name]/actions";
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
const NOT_ALLOWED =
  'the deletion policy is Orphan, and these registrations do not support it: PluginRegistration "ebs" of section "management-database"';
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
  spec: {
    releaseRef: string;
    deletionPolicy?: DeletionPolicy;
    plugins?: { section: string; registration: string }[];
  };
  status: { resolvedPolicy: { deletionPolicy: DeletionPolicy } };
};
const claim = (
  over: {
    metadata?: object;
    spec?: {
      deletionPolicy?: DeletionPolicy;
      plugins?: { section: string; registration: string }[];
    };
  } = {},
): Claim => ({
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: KIND,
  metadata: {
    namespace: "platform",
    name: "plane-a",
    resourceVersion: "41",
    uid: "uid-1",
    generation: 1,
    finalizers: ["fleet.camunda.io/cleanup"],
    ...over.metadata,
  },
  spec: { releaseRef: "camunda-8-9-1", ...over.spec },
  status: { resolvedPolicy: { deletionPolicy: "Delete" } },
});

const actions: ClaimDeleteActions = {
  preview: previewPlaneClaimDelete,
  remove: applyPlaneClaimDelete,
  applyPolicy: applyPlaneClaimChange,
};
// reviewOf is the remove that the hook gives to the review and sends.
const reviewOf = (generation = 1) => ({
  intent: "Delete the plane plane-a",
  apply: [],
  delete: [ref],
  uids: { [key]: "uid-1" },
  generations: { [key]: generation },
});
// removeOf is that remove as the direct writer gets it from
// runClaimDelete: at the resourceVersion of the claim on the server.
const removeOf = (generation = 1, resourceVersion = "41") => ({
  ...reviewOf(generation),
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

type Registration = Parameters<
  typeof useClaimDelete
>[0]["registrations"][number];

function mount(
  initial: Claim,
  props: {
    policy?: DeletionPolicy;
    mode?: WriteMode;
    actions?: Partial<ClaimDeleteActions>;
    registrations?: Registration[];
  } = {},
) {
  const shown: [WriteResult, string][] = [];
  fakeWriter.objects = [initial];
  const client = new QueryClient();
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>
      <GitPushesProvider>{children}</GitPushesProvider>
    </QueryClientProvider>
  );
  const hook = renderHook(
    (p: { policy: DeletionPolicy; registrations?: Registration[] }) =>
      useClaimDelete({
        initial,
        policy: p.policy,
        registrations: p.registrations ?? props.registrations ?? [],
        mode: props.mode ?? "direct",
        intent: "Delete the plane plane-a",
        pushedMessage: "Pushed the delete of plane-a.",
        actions: { ...actions, ...props.actions },
        show: (result, message) => shown.push([result, message]),
      }),
    {
      wrapper,
      initialProps: {
        policy: props.policy ?? "Delete",
        registrations: props.registrations,
      } as { policy: DeletionPolicy; registrations?: Registration[] },
    },
  );
  return { hook, shown };
}

// selecting gives c the plain EBS option, whose registration plainEbs
// gives the deletion policies.
const selecting = (c: Claim = claim()): Claim => ({
  ...c,
  spec: {
    ...c.spec,
    plugins: [{ section: "volume-encryption", registration: "ebs-plain" }],
  },
});
const plainEbs = (deletionPolicies: DeletionPolicy[]): Registration => ({
  metadata: { name: "ebs-plain" },
  spec: { deletionPolicies },
});
const KEEP_REFUSED_MESSAGE =
  'PluginRegistration "ebs-plain" of section "volume-encryption" cannot keep the resources. Choose Delete the resources.';
const KEEP_REFUSED_DELETING =
  'The delete waits, because PluginRegistration "ebs-plain" of section "volume-encryption" cannot keep the resources. Set the deletion policy of the claim to Delete, or add Orphan back to the deletion policies of the registration.';

const watch = watchId({ kind: KIND, namespace: "platform", name: "plane-a" });

const requestsWatch = watchId({
  kind: "PluginRequest",
  namespace: "platform",
});

// request is a PluginRequest of the claim with uid-1 that has policy.
const request = (name: string, policy: DeletionPolicy) => ({
  apiVersion: "plugins.camunda.io/v1alpha1",
  kind: "PluginRequest",
  metadata: {
    name,
    namespace: "platform",
    ownerReferences: [{ controller: true, uid: "uid-1" }],
  },
  spec: { deletionPolicy: policy },
});

// live shows c on the watch of the claim, and c undefined shows the claim
// as gone. requests "unread" leaves the list of the requests unread, and
// the default [] is a read list with no requests. The server then has
// server, c by default, and the requests.
async function live(
  c: Claim | undefined,
  server = c,
  requests: ReturnType<typeof request>[] | "unread" = [],
) {
  const listed = requests === "unread" ? [] : requests;
  fakeWriter.objects = [...(server ? [server] : []), ...listed];
  await waitFor(() =>
    expect(FakeEventSource.instances.at(-1)?.ids()).toContain(watch),
  );
  FakeEventSource.instances
    .at(-1)!
    .emit("snapshot", { watch, kind: KIND, objects: c ? [c] : [] });
  if (requests !== "unread") {
    await waitFor(() =>
      expect(FakeEventSource.instances.at(-1)?.ids()).toContain(requestsWatch),
    );
    FakeEventSource.instances.at(-1)!.emit("snapshot", {
      watch: requestsWatch,
      kind: "PluginRequest",
      objects: requests,
    });
  }
  await act(async () => {});
}

// revisionOfFirstPush is the revision that GitOps writes for the first
// push of fakeWriter.
function revisionOfFirstPush() {
  return createHash("sha1")
    .update(JSON.stringify(fakeWriter.applied[0].apply[0]))
    .digest("hex");
}

// pushed is read(resourceVersion) once GitOps applied the first push.
function pushed(resourceVersion: string): Claim {
  const c = read(resourceVersion);
  c.metadata.annotations = { [REVISION_ANNOTATION]: revisionOfFirstPush() };
  return c;
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
  it("gives the remove step of the live claim with its UID and generation, a new one for each pick and spec change but not for a status write", async () => {
    const { hook } = mount(claim());
    await live(claim());
    await waitFor(() =>
      expect(hook.result.current.reads).toEqual({ errors: [], stale: [] }),
    );
    const first = hook.result.current.remove;
    expect(first).toEqual(reviewOf());
    hook.rerender({ policy: "Orphan" });
    const picked = hook.result.current.remove;
    expect(picked).not.toBe(first);
    expect(picked).toEqual(reviewOf());
    expect(hook.result.current.policyStep).toBe(true);
    await live(claim({ metadata: { resourceVersion: "42" } }));
    await waitFor(() =>
      expect(hook.result.current.claim?.metadata.resourceVersion).toBe("42"),
    );
    expect(hook.result.current.remove).toBe(picked);
    const edited = claim({
      metadata: { resourceVersion: "44", generation: 2 },
    });
    edited.spec.releaseRef = "camunda-8-9-2";
    await live(edited);
    await waitFor(() => expect(hook.result.current.remove).not.toBe(picked));
    expect(hook.result.current.remove).toEqual(reviewOf(2));
  });

  it("gives a new remove step when the generation moved and the spec is back to the one before", async () => {
    const { hook } = mount(claim());
    const first = hook.result.current.remove;
    await live(claim({ metadata: { resourceVersion: "44", generation: 3 } }));
    await waitFor(() => expect(hook.result.current.remove).not.toBe(first));
    expect(hook.result.current.remove).toEqual(reviewOf(3));
    expect(await confirm(hook)).toEqual({ ok: true, mode: "direct" });
  });

  it("Git mode gives a new remove step when the requests of the claim come to carry its policy", async () => {
    fakeWriter.reset("git");
    const { hook } = mount(claim(), { mode: "git" });
    await live(claim(), claim(), [request("plane-a-db", "Orphan")]);
    await waitFor(() =>
      expect(hook.result.current.reads).toEqual({ errors: [], stale: [] }),
    );
    const behind = hook.result.current.remove;
    await live(claim(), claim(), [request("plane-a-db", "Delete")]);
    await waitFor(() => expect(hook.result.current.remove).not.toBe(behind));
  });

  it("gives a new remove step when the cleanup finalizer of the claim comes back", async () => {
    const lost = claim({ metadata: { finalizers: [] } });
    const { hook } = mount(lost);
    await live(lost);
    await waitFor(() =>
      expect(hook.result.current.reads).toEqual({ errors: [], stale: [] }),
    );
    const before = hook.result.current.remove;
    await live(claim({ metadata: { resourceVersion: "42" } }));
    await waitFor(() => expect(hook.result.current.remove).not.toBe(before));
  });

  it("a status write that the watch has not shown yet does not refuse the delete", async () => {
    const { hook } = mount(claim());
    fakeWriter.objects = [claim({ metadata: { resourceVersion: "43" } })];
    expect(await confirm(hook)).toEqual({ ok: true, mode: "direct" });
    expect(fakeWriter.applied).toEqual([removeOf(1, "43")]);
  });

  it("a refused delete gives one new remove once the watch shows a newer version, and none for the versions after it", async () => {
    const { hook } = mount(claim());
    // The spec on the server changed since the review.
    fakeWriter.objects = [
      claim({ metadata: { resourceVersion: "42", generation: 2 } }),
    ];
    const first = hook.result.current.remove;
    expect(await confirm(hook)).toMatchObject({
      ok: false,
      errors: [{ code: "Stale" }],
    });
    expect(fakeWriter.applied).toEqual([]);
    expect(hook.result.current.remove).toBe(first);
    await live(claim({ metadata: { resourceVersion: "42" } }));
    await waitFor(() => expect(hook.result.current.remove).not.toBe(first));
    const retried = hook.result.current.remove;
    for (const resourceVersion of ["43", "44"]) {
      await live(claim({ metadata: { resourceVersion } }));
      await waitFor(() =>
        expect(hook.result.current.claim?.metadata.resourceVersion).toBe(
          resourceVersion,
        ),
      );
      expect(hook.result.current.remove).toBe(retried);
    }
  });

  it("a delete refused after the watch showed a newer version during the write gives a new remove at once", async () => {
    let settle: (result: WriteResult) => void = () => {};
    const { hook } = mount(claim(), {
      actions: {
        remove: () =>
          new Promise<WriteResult>((resolve) => {
            settle = resolve;
          }),
      },
    });
    const first = hook.result.current.remove;
    const { apply } = hook.result.current;
    let pending: Promise<WriteResult> = Promise.resolve({
      ok: true,
      mode: "direct",
    });
    act(() => {
      pending = apply(first);
    });
    await live(claim({ metadata: { resourceVersion: "42" } }));
    await waitFor(() =>
      expect(hook.result.current.claim?.metadata.resourceVersion).toBe("42"),
    );
    await act(async () => {
      settle({ ok: false, errors: [{ code: "Stale", message: "Changed." }] });
      await pending;
    });
    await waitFor(() => expect(hook.result.current.remove).not.toBe(first));
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

  it("direct mode without a policy step sends the delete with the UID and the generation of the review", async () => {
    const { hook } = mount(claim());
    expect(await confirm(hook)).toEqual({ ok: true, mode: "direct" });
    expect(fakeWriter.applied).toEqual([removeOf()]);
    expect(hook.result.current.stage).toEqual({
      kind: "deleting",
      gitWaiting: false,
    });
  });

  it("direct mode writes the policy and deletes a claim with the cleanup finalizer once the claim shows it, without a wait for the claim controller or the requests", async () => {
    const { hook, shown } = mount(claim(), { policy: "Orphan" });
    expect(hook.result.current.stage).toEqual({ kind: "choose" });
    expect(await confirm(hook)).toEqual({ ok: true, mode: "direct" });
    expect(fakeWriter.previewed).toEqual([reviewOf()]);
    expect(fakeWriter.applied).toHaveLength(1);
    expect(fakeWriter.applied[0]).toMatchObject({
      apply: [{ kind: KIND, spec: { deletionPolicy: "Orphan" } }],
      preconditions: { [key]: "41" },
    });
    expect(hook.result.current.stage).toEqual({ kind: "waiting" });

    // The precheck of the claim fails, so the claim controller read the
    // write but does not give the policy to the request.
    const shownClaim = read("42");
    await live(
      {
        ...shownClaim,
        status: {
          ...shownClaim.status,
          conditions: [
            { type: "Ready", status: "False", reason: "InvalidReference" },
          ],
        },
      } as unknown as Claim,
      undefined,
      [request("plane-a-db", "Delete")],
    );
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(2));
    expect(fakeWriter.applied[1]).toEqual(removeOf(2, "42"));
    await waitFor(() =>
      expect(hook.result.current.stage).toEqual({
        kind: "deleting",
        gitWaiting: false,
      }),
    );
    expect(shown).toEqual([]);
  });

  it("a status write of the claim controller between the policy write and the delete does not refuse the delete", async () => {
    const { hook } = mount(claim(), { policy: "Orphan" });
    await confirm(hook);
    // The server has a newer status than the watch showed.
    await live(read("42"), read("43"));
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(2));
    expect(fakeWriter.applied[1]).toEqual(removeOf(2, "43"));
  });

  it("Git mode: after the policy write, the push waits while a request of the claim has the old policy, says why, and goes once each has the new one", async () => {
    fakeWriter.reset("git");
    const { hook } = mount(claim(), { policy: "Orphan", mode: "git" });
    await confirm(hook);
    const stuck = pushed("42");
    stuck.status = {
      ...stuck.status,
      conditions: [
        {
          type: "Ready",
          status: "False",
          reason: "NotAllowed",
          message: NOT_ALLOWED,
        },
      ],
    } as Claim["status"];
    await live(stuck, stuck, [
      request("plane-a-db", "Delete"),
      request("plane-a-access", "Orphan"),
    ]);
    await waitFor(() =>
      expect(hook.result.current.stage).toEqual({
        kind: "waiting",
        blocked: `The request plane-a-db does not have the deletion policy of the claim. The operator does not update it while the claim reports NotAllowed: ${NOT_ALLOWED}`,
      }),
    );
    expect(fakeWriter.applied).toHaveLength(1);

    const next = pushed("43");
    await live(next, next, [
      request("plane-a-db", "Orphan"),
      request("plane-a-access", "Orphan"),
    ]);
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(2));
    expect(fakeWriter.applied[1]).toEqual(reviewOf(2));
  });

  it("Git mode: while a request has the old policy and the claim reports another failed check, the wait does not blame the check", async () => {
    fakeWriter.reset("git");
    const { hook } = mount(claim(), { policy: "Orphan", mode: "git" });
    await confirm(hook);
    const stuck = pushed("42");
    stuck.status = {
      ...stuck.status,
      conditions: [
        {
          type: "Ready",
          status: "False",
          reason: "InvalidReference",
          message: "CamundaRelease r1 does not exist",
        },
      ],
    } as Claim["status"];
    await live(stuck, stuck, [request("plane-a-db", "Delete")]);
    await waitFor(() =>
      expect(hook.result.current.stage).toEqual({
        kind: "waiting",
        blocked:
          "The request plane-a-db does not have the deletion policy of the claim. The operator has not updated it yet.",
      }),
    );
    expect(fakeWriter.applied).toHaveLength(1);
  });

  it.each([
    ["the list of the requests is unread", read("42"), "unread" as const],
    [
      "the claim has a recorded policy and no observedGeneration",
      {
        ...read("42"),
        status: { resolvedPolicy: { deletionPolicy: "Delete" } },
      } as unknown as Claim,
      [],
    ],
  ])(
    "Git mode: after the policy write, the push waits while %s",
    async (_, c, requests) => {
      fakeWriter.reset("git");
      const remove = vi.fn(applyPlaneClaimDelete);
      const { hook } = mount(claim(), {
        policy: "Orphan",
        mode: "git",
        actions: { remove },
      });
      await confirm(hook);
      const applied = {
        ...c,
        metadata: {
          ...c.metadata,
          annotations: { [REVISION_ANNOTATION]: revisionOfFirstPush() },
        },
      };
      await live(applied, applied, requests);
      await waitFor(() =>
        expect(hook.result.current.claim?.metadata.resourceVersion).toBe("42"),
      );
      expect(hook.result.current.stage).toEqual({ kind: "waiting" });
      expect(remove).not.toHaveBeenCalled();
    },
  );

  it.each([
    [
      "a request that is being deleted",
      {
        ...request("plane-a-db", "Delete"),
        metadata: {
          ...request("plane-a-db", "Delete").metadata,
          deletionTimestamp: "2026-11-16T10:00:00Z",
        },
      },
    ],
    [
      "a request that the claim owns without the controller flag",
      {
        ...request("plane-a-db", "Delete"),
        metadata: {
          ...request("plane-a-db", "Delete").metadata,
          ownerReferences: [{ controller: false, uid: "uid-1" }],
        },
      },
    ],
  ])("Git mode: the push does not wait for %s", async (_, other) => {
    fakeWriter.reset("git");
    const { hook } = mount(claim(), { policy: "Orphan", mode: "git" });
    await confirm(hook);
    const applied = pushed("42");
    await live(applied, applied, [other as ReturnType<typeof request>]);
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(2));
    expect(hook.result.current.stage).toMatchObject({ kind: "deleting" });
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
        afterPolicyWrite: { held: "Orphan" },
      }),
    );
    await live(read("44"));
    await waitFor(() =>
      expect(hook.result.current.claim?.metadata.resourceVersion).toBe("44"),
    );
    expect(hook.result.current.stage).toEqual({
      kind: "choose",
      errors: [{ code: "Forbidden", message: "No delete." }],
      afterPolicyWrite: { held: "Orphan" },
    });
    expect(fakeWriter.applied).toHaveLength(2);
  });

  it("a refused retry after a refused delete keeps the written policy", async () => {
    const { hook } = mount(claim(), { policy: "Orphan" });
    await confirm(hook);
    fakeWriter.results = [
      { ok: false, errors: [{ code: "Forbidden", message: "No delete." }] },
    ];
    await live(read("42"));
    await waitFor(() =>
      expect(hook.result.current.stage).toMatchObject({
        kind: "choose",
        afterPolicyWrite: { held: "Orphan" },
      }),
    );
    fakeWriter.results = [
      { ok: false, errors: [{ code: "Forbidden", message: "Still no." }] },
    ];
    const retried = await confirm(hook);
    expect(retried.ok).toBe(false);
    expect(hook.result.current.stage).toMatchObject({
      kind: "choose",
      afterPolicyWrite: { held: "Orphan" },
    });
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
        afterPolicyWrite: { held: "Orphan" },
      }),
    );
    expect(fakeWriter.applied).toHaveLength(1);
  });

  it("refuses with Stale when another change takes the policy back during the wait, and gives the policy of the live claim", async () => {
    const { hook } = mount(claim(), { policy: "Orphan" });
    await confirm(hook);
    await live(read("42", "Delete"));
    await waitFor(() =>
      expect(hook.result.current.stage).toMatchObject({
        kind: "choose",
        errors: [{ code: "Stale" }],
        afterPolicyWrite: { held: "Delete" },
      }),
    );
    expect(fakeWriter.applied).toHaveLength(1);
  });

  it("Git mode refuses with Stale when the spec from before the write comes back after the claim showed the write", async () => {
    fakeWriter.reset("git");
    const { hook } = mount(claim(), { policy: "Orphan", mode: "git" });
    await confirm(hook);
    await live(read("42"));
    await waitFor(() =>
      expect(hook.result.current.claim?.metadata.resourceVersion).toBe("42"),
    );
    expect(hook.result.current.stage).toEqual({ kind: "waiting" });
    await live(claim({ metadata: { resourceVersion: "43" } }));
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
        afterPolicyWrite: { held: "Delete", unapplied: "Orphan" },
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
    expect(fakeWriter.applied[1]).toEqual(reviewOf(2));
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

  it("Git mode waits for GitOps and then for the requests before it pushes the delete", async () => {
    fakeWriter.reset("git");
    const { hook } = mount(claim(), { policy: "Orphan", mode: "git" });
    await confirm(hook);
    const revision = createHash("sha1")
      .update(JSON.stringify(fakeWriter.applied[0].apply[0]))
      .digest("hex");
    const applied = read("42");
    applied.metadata.annotations = { [REVISION_ANNOTATION]: revision };
    await live(applied, applied, [request("plane-a-db", "Delete")]);
    await waitFor(() =>
      expect(hook.result.current.claim?.metadata.resourceVersion).toBe("42"),
    );
    expect(hook.result.current.stage).toMatchObject({ kind: "waiting" });
    expect(fakeWriter.applied).toHaveLength(1);

    const next = {
      ...applied,
      metadata: { ...applied.metadata, resourceVersion: "43" },
    };
    await live(next, next, [request("plane-a-db", "Orphan")]);
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(2));
    expect(fakeWriter.applied[1]).toEqual(reviewOf(2));
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
        afterPolicyWrite: { held: "Orphan" },
      }),
    );
  });

  it("Git mode: stop waiting goes back to the choice with the commit that the claim does not carry, and sends no delete", async () => {
    fakeWriter.reset("git");
    const { hook } = mount(claim(), { policy: "Orphan", mode: "git" });
    await confirm(hook);
    act(() => hook.result.current.stopWaiting());
    expect(hook.result.current.stage).toEqual({
      kind: "choose",
      afterPolicyWrite: { held: "Delete", unapplied: "Orphan" },
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
    expect(hook.result.current.stage).toEqual({
      kind: "choose",
      afterPolicyWrite: { held: "Orphan" },
    });
    expect(fakeWriter.applied).toHaveLength(1);
  });

  it("direct mode: stop waiting gives the policy of the live claim", async () => {
    const { hook } = mount(claim(), { policy: "Orphan" });
    await confirm(hook);
    act(() => hook.result.current.stopWaiting());
    expect(hook.result.current.stage).toEqual({
      kind: "choose",
      afterPolicyWrite: { held: "Delete" },
    });
    const unread = read("42");
    await live({
      ...unread,
      status: { ...unread.status, observedGeneration: 1 },
    } as unknown as Claim);
    await waitFor(() =>
      expect(hook.result.current.stage).toEqual({
        kind: "choose",
        afterPolicyWrite: { held: "Orphan" },
      }),
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

  it("refuses Keep without a write while a registration of the claim cannot keep, and names it", async () => {
    const { hook } = mount(selecting(), {
      policy: "Orphan",
      registrations: [plainEbs(["Delete"])],
    });
    expect(hook.result.current.keepRefused).toBe(KEEP_REFUSED_MESSAGE);
    expect(await confirm(hook)).toEqual({
      ok: false,
      errors: [{ code: "Invalid", message: KEEP_REFUSED_MESSAGE }],
    });
    expect(fakeWriter.previewed).toEqual([]);
    expect(fakeWriter.applied).toEqual([]);
    hook.rerender({ policy: "Delete" });
    expect(hook.result.current.keepRefused).toBeUndefined();
  });

  describe("a selected registration that does not exist", () => {
    const MISSING =
      'PluginRegistration "ebs-plain" of section "volume-encryption" does not exist, so the operator cannot give Keep to the options. Bring the registration back, or choose Delete the resources.';
    // gone is a claim that holds Delete and selects the plain EBS option,
    // whose registration does not exist.
    const gone = () => selecting(read("41", "Delete"));

    it("offers Keep in direct mode with the cleanup finalizer, which gives Orphan to the requests on delete, and deletes the claim", async () => {
      const { hook } = mount(gone(), { policy: "Orphan", registrations: [] });
      await live(gone(), gone(), [request("plane-a-db", "Delete")]);
      expect(hook.result.current.keepRefused).toBeUndefined();
      expect((await confirm(hook)).ok).toBe(true);
      const kept = selecting(read("42"));
      await live(kept, kept, [request("plane-a-db", "Delete")]);
      await waitFor(() => expect(fakeWriter.applied).toHaveLength(2));
      expect(fakeWriter.applied[1].delete).toHaveLength(1);
    });

    it("refuses Keep in direct mode when the claim has no cleanup finalizer and a request does not hold Orphan", async () => {
      const bare = {
        ...gone(),
        metadata: { ...gone().metadata, finalizers: [] },
      } as Claim;
      const { hook } = mount(bare, { policy: "Orphan", registrations: [] });
      await live(bare, bare, [request("plane-a-db", "Delete")]);
      await waitFor(() =>
        expect(hook.result.current.keepRefused).toBe(MISSING),
      );
    });

    it("offers Keep in Git mode when the claim controls no request", async () => {
      fakeWriter.reset("git");
      const { hook } = mount(gone(), {
        policy: "Orphan",
        mode: "git",
        registrations: [],
      });
      await live(gone(), gone(), []);
      expect(hook.result.current.keepRefused).toBeUndefined();
      // A request that does not hold Orphan is refused, so the list of the
      // requests was read.
      await live(gone(), gone(), [request("plane-a-db", "Delete")]);
      await waitFor(() =>
        expect(hook.result.current.keepRefused).toBe(MISSING),
      );
    });

    it("offers Keep in Git mode when each request holds Orphan already", async () => {
      fakeWriter.reset("git");
      const kept = selecting(read("41"));
      const { hook } = mount(kept, {
        policy: "Orphan",
        mode: "git",
        registrations: [],
      });
      await live(kept, kept, [request("plane-a-db", "Orphan")]);
      expect(hook.result.current.keepRefused).toBeUndefined();
      // The same claim with a request that does not hold Orphan yet is
      // refused, so the list of the requests was read.
      await live(kept, kept, [request("plane-a-db", "Delete")]);
      await waitFor(() =>
        expect(hook.result.current.keepRefused).toBe(MISSING),
      );
    });

    it("refuses Keep without a write in Git mode while a request does not hold Orphan, and says why", async () => {
      fakeWriter.reset("git");
      const { hook } = mount(gone(), {
        policy: "Orphan",
        mode: "git",
        registrations: [],
      });
      await live(gone(), gone(), [request("plane-a-db", "Delete")]);
      await waitFor(() =>
        expect(hook.result.current.keepRefused).toBe(MISSING),
      );
      expect(await confirm(hook)).toEqual({
        ok: false,
        errors: [{ code: "Invalid", message: MISSING }],
      });
      expect(fakeWriter.applied).toEqual([]);
      hook.rerender({ policy: "Delete" });
      expect(hook.result.current.keepRefused).toBeUndefined();
    });

    it("does not refuse Keep while the list of the requests is not read", async () => {
      fakeWriter.reset("git");
      const { hook } = mount(gone(), {
        policy: "Orphan",
        mode: "git",
        registrations: [],
      });
      await live(gone(), gone(), "unread");
      expect(hook.result.current.keepRefused).toBeUndefined();
    });
  });

  it("sends no delete when a registration drops Orphan during the wait for the policy", async () => {
    const { hook } = mount(selecting(), {
      policy: "Orphan",
      registrations: [plainEbs(["Delete", "Orphan"])],
    });
    expect(hook.result.current.keepRefused).toBeUndefined();
    await confirm(hook);
    expect(hook.result.current.stage).toEqual({ kind: "waiting" });
    hook.rerender({ policy: "Orphan", registrations: [plainEbs(["Delete"])] });
    await live(selecting(read("42")));
    await waitFor(() =>
      expect(hook.result.current.stage).toEqual({
        kind: "choose",
        errors: [{ code: "Invalid", message: KEEP_REFUSED_MESSAGE }],
        afterPolicyWrite: { held: "Orphan" },
      }),
    );
    expect(fakeWriter.applied).toHaveLength(1);
  });

  it("a claim whose delete the claim controller refuses for a registration that cannot keep says what to do", async () => {
    const refusedClaim = (reason: string) => {
      const c = selecting(
        claim({
          metadata: {
            deletionTimestamp: "2026-11-15T10:00:00Z",
            generation: 2,
          },
          spec: { deletionPolicy: "Orphan" },
        }),
      );
      return {
        ...c,
        status: {
          ...c.status,
          observedGeneration: 2,
          conditions: [
            {
              type: "Ready",
              status: "False",
              reason,
              message: "the deletion policy is Orphan",
              observedGeneration: 2,
            },
          ],
        },
      } as unknown as Claim;
    };
    const { hook } = mount(refusedClaim("NotAllowed"), {
      registrations: [plainEbs(["Delete"])],
    });
    expect(hook.result.current.stage).toEqual({
      kind: "deleting",
      gitWaiting: false,
      keepRefused: KEEP_REFUSED_DELETING,
    });
    hook.unmount();
    FakeEventSource.instances = [];
    const other = mount(refusedClaim("SlotConflict"), {
      registrations: [plainEbs(["Delete"])],
    });
    expect(other.hook.result.current.stage).toEqual({
      kind: "deleting",
      gitWaiting: false,
    });
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
