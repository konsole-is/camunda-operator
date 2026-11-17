import { beforeEach, describe, expect, it, vi } from "vitest";
import { fakeWriter } from "../../../../../../test/fakes/writer";
import {
  applyPlaneClaimChange,
  applyPlaneClaimDelete,
  previewPlaneClaimChange,
  previewPlaneClaimDelete,
} from "./actions";

vi.mock(
  "@/lib/write/writer",
  () => import("../../../../../../test/fakes/writer"),
);

const change = (kind = "CamundaManagementClusterClaim") => ({
  intent: "Suspend the management plane plane-a",
  apply: [
    {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind,
      metadata: { name: "plane-a", namespace: "platform" },
      spec: { suspend: true },
    },
  ],
  preconditions: { [`${kind}/platform/plane-a` as const]: "7" },
});

const remove = (kind = "CamundaManagementClusterClaim") => ({
  intent: "Delete the management plane plane-a",
  apply: [],
  delete: [
    {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind,
      namespace: "platform",
      name: "plane-a",
    },
  ],
  uids: { [`${kind}/platform/plane-a` as const]: "uid-1" },
  generations: { [`${kind}/platform/plane-a` as const]: 3 },
});

const CLEANUP = "fleet.camunda.io/cleanup";
// liveClaim is the plane claim as the API server holds it: the operator
// recorded the deletion policy Orphan for generation 3.
const liveClaim = (finalizers: string[] = [CLEANUP]) => ({
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaManagementClusterClaim",
  metadata: {
    name: "plane-a",
    namespace: "platform",
    uid: "uid-1",
    generation: 3,
    resourceVersion: "45",
    finalizers,
  },
  spec: { deletionPolicy: "Orphan" },
  status: {
    observedGeneration: 3,
    resolvedPolicy: { deletionPolicy: "Orphan" },
  },
});
// lagging is a request of the plane claim that still has the policy Delete.
const lagging = {
  apiVersion: "plugins.camunda.io/v1alpha1",
  kind: "PluginRequest",
  metadata: {
    name: "plane-a-management-database",
    namespace: "platform",
    ownerReferences: [
      {
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "CamundaManagementClusterClaim",
        name: "plane-a",
        uid: "uid-1",
        controller: true,
      },
    ],
  },
  spec: { section: "management-database", deletionPolicy: "Delete" },
};

beforeEach(() => fakeWriter.reset());

describe("applyPlaneClaimChange", () => {
  it("applyPlaneClaimChange writes a change of a plane claim through the writer", async () => {
    expect(await applyPlaneClaimChange(change())).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([change()]);
  });

  it("applyPlaneClaimChange refuses a change of another kind and writes nothing", async () => {
    const result = await applyPlaneClaimChange(change("Environment"));
    expect(result.ok).toBe(false);
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });
});

describe("previewPlaneClaimChange", () => {
  it("previewPlaneClaimChange previews through the writer", async () => {
    const refused = {
      objects: [],
      errors: [{ code: "Invalid" as const, message: "CEL", path: "spec" }],
    };
    fakeWriter.previews = [refused];
    expect(await previewPlaneClaimChange(change())).toEqual(refused);
    expect(fakeWriter.previewed).toEqual([change()]);
  });

  it("previewPlaneClaimChange refuses a change of another kind and previews nothing", async () => {
    const result = await previewPlaneClaimChange(change("Environment"));
    expect(result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.previewed).toEqual([]);
  });
});

describe("applyPlaneClaimDelete", () => {
  it("applyPlaneClaimDelete deletes the plane claim through the writer at the resourceVersion that it reads", async () => {
    fakeWriter.objects = [liveClaim()];
    expect(await applyPlaneClaimDelete(remove())).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([
      {
        ...remove(),
        preconditions: {
          "CamundaManagementClusterClaim/platform/plane-a": "45",
        },
      },
    ]);
  });

  it("in direct mode deletes a plane claim with the cleanup finalizer while a request still has the old policy", async () => {
    fakeWriter.objects = [liveClaim(), lagging];
    expect(await applyPlaneClaimDelete(remove())).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toHaveLength(1);
  });

  it("in Git mode refuses with Stale, names the request, and writes nothing while a request lacks the policy", async () => {
    fakeWriter.reset("git");
    fakeWriter.objects = [liveClaim(), lagging];
    const result = await applyPlaneClaimDelete(remove());
    expect(result).toMatchObject({
      ok: false,
      errors: [
        {
          code: "Stale",
          message: expect.stringContaining("plane-a-management-database"),
        },
      ],
    });
    expect(fakeWriter.applied).toEqual([]);
  });

  it("in Git mode sends the delete once each request has the policy", async () => {
    fakeWriter.reset("git");
    fakeWriter.objects = [
      liveClaim(),
      { ...lagging, spec: { ...lagging.spec, deletionPolicy: "Orphan" } },
    ];
    expect(await applyPlaneClaimDelete(remove())).toMatchObject({
      ok: true,
      mode: "git",
    });
    expect(fakeWriter.applied).toEqual([remove()]);
  });

  it("refuses with Invalid a plane claim that lost its cleanup finalizer after the operator recorded its policy", async () => {
    fakeWriter.objects = [liveClaim([])];
    expect(await applyPlaneClaimDelete(remove())).toMatchObject({
      ok: false,
      errors: [{ code: "Invalid", message: expect.stringContaining(CLEANUP) }],
    });
    expect(fakeWriter.applied).toEqual([]);
  });

  it("refuses with Stale and writes nothing when the spec of the plane claim changed since the review", async () => {
    fakeWriter.objects = [
      { ...liveClaim(), metadata: { ...liveClaim().metadata, generation: 4 } },
    ];
    expect(await applyPlaneClaimDelete(remove())).toMatchObject({
      ok: false,
      errors: [{ code: "Stale" }],
    });
    expect(fakeWriter.applied).toEqual([]);
  });

  it("applyPlaneClaimDelete refuses a delete of another kind and writes nothing", async () => {
    const result = await applyPlaneClaimDelete(remove("Environment"));
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });

  it("applyPlaneClaimDelete refuses a change that applies an object and writes nothing", async () => {
    const result = await applyPlaneClaimDelete({
      ...remove(),
      apply: change().apply,
    });
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });
});

describe("previewPlaneClaimDelete", () => {
  it("previewPlaneClaimDelete previews the delete through the writer", async () => {
    fakeWriter.objects = [liveClaim()];
    expect(await previewPlaneClaimDelete(remove())).toEqual({
      objects: [],
      errors: [],
    });
    expect(fakeWriter.previewed).toEqual([remove()]);
  });

  it("previewPlaneClaimDelete refuses a delete of another kind and previews nothing", async () => {
    const result = await previewPlaneClaimDelete(remove("Environment"));
    expect(result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.previewed).toEqual([]);
  });
});

describe("the plane actions refuse a cluster claim", () => {
  it("each plane action refuses a cluster claim as Invalid and writes nothing", async () => {
    const cluster = "CamundaClusterClaim";
    const changed = await applyPlaneClaimChange(change(cluster));
    expect(!changed.ok && changed.errors[0].code).toBe("Invalid");
    expect(
      (await previewPlaneClaimChange(change(cluster))).errors[0].code,
    ).toBe("Invalid");
    const removed = await applyPlaneClaimDelete(remove(cluster));
    expect(!removed.ok && removed.errors[0].code).toBe("Invalid");
    expect(
      (await previewPlaneClaimDelete(remove(cluster))).errors[0].code,
    ).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
    expect(fakeWriter.previewed).toEqual([]);
  });
});
