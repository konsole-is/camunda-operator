import { beforeEach, describe, expect, it, vi } from "vitest";
import { fakeWriter } from "../../../../test/fakes/writer";
import {
  applyReleaseChange,
  applyReleaseMove,
  previewReleaseChange,
  previewReleaseMove,
} from "./actions";

vi.mock("@/lib/write/writer", () => import("../../../../test/fakes/writer"));

const releaseChange = (kind = "CamundaRelease") => ({
  intent: "Change the release camunda-8-9-0",
  apply: [
    {
      apiVersion: "core.camunda.io/v1",
      kind,
      metadata: { name: "camunda-8-9-0" },
      spec: { version: "8.9.1" },
    },
  ],
  preconditions: { [`${kind}//camunda-8-9-0` as const]: "3" },
  force: true,
});

const deleteWithout = {
  intent: "Delete the release camunda-8-9-1",
  apply: [],
  delete: [
    {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaRelease",
      name: "camunda-8-9-1",
    },
  ],
};

beforeEach(() => fakeWriter.reset());

describe("applyReleaseChange", () => {
  it("writes a CamundaRelease update", async () => {
    expect(await applyReleaseChange(releaseChange())).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([releaseChange()]);
  });

  it("refuses a change that writes another kind", async () => {
    const result = await applyReleaseChange(releaseChange("Environment"));
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });

  it("refuses a delete without a precondition", async () => {
    const result = await applyReleaseChange(deleteWithout);
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });
});

describe("previewReleaseChange", () => {
  it("previews a CamundaRelease update", async () => {
    await previewReleaseChange(releaseChange());
    expect(fakeWriter.previewed).toEqual([releaseChange()]);
  });

  it("refuses a change that writes another kind", async () => {
    const result = await previewReleaseChange(releaseChange("Environment"));
    expect(result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.previewed).toEqual([]);
  });
});

const clusterClaim = (namespace: string, name: string) => ({
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaClusterClaim",
  metadata: { namespace, name },
  spec: { releaseRef: "camunda-8-9-1" },
});

const clusterMove = {
  intent: "Move the cluster team-a/orders to the release camunda-8-9-1",
  apply: [
    {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaCluster",
      metadata: { namespace: "team-a", name: "orders" },
      spec: { releaseRef: "camunda-8-9-1" },
    },
  ],
  preconditions: { "CamundaCluster/team-a/orders": "4" } as const,
};

// liveClaim is the claim as the API server holds it before the move.
const liveClaim = (
  namespace: string,
  name: string,
  resourceVersion: string,
) => ({
  ...clusterClaim(namespace, name),
  metadata: { namespace, name, resourceVersion, uid: `${name}-uid` },
  spec: { releaseRef: "camunda-8-9-0", presetRef: "medium" },
  status: { phase: "Ready" },
});

const movedClaim = (namespace: string, name: string, spec = {}) => ({
  ...clusterClaim(namespace, name),
  spec: { releaseRef: "camunda-8-9-1", presetRef: "medium", ...spec },
});

const twoLiveClaimMove = {
  intent: "Move 2 clusters to the release camunda-8-9-1",
  apply: [movedClaim("team-a", "orders"), movedClaim("team-b", "payments")],
  preconditions: {
    "CamundaClusterClaim/team-a/orders": "4",
    "CamundaClusterClaim/team-b/payments": "9",
  } as const,
};

const backup = (kind: string, phase?: string) => ({
  apiVersion: "core.camunda.io/v1",
  kind,
  metadata: { namespace: "team-b", name: "nightly" },
  spec: { clusterRef: { name: "payments" } },
  ...(phase && { status: { phase } }),
});

function seedLiveClaims(...extra: object[]) {
  fakeWriter.objects = [
    liveClaim("team-a", "orders", "4"),
    liveClaim("team-b", "payments", "9"),
    ...extra,
  ] as typeof fakeWriter.objects;
}

describe("applyReleaseMove", () => {
  it("sends a move of two claims to the writer as one change", async () => {
    seedLiveClaims();
    expect(await applyReleaseMove(twoLiveClaimMove)).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([twoLiveClaimMove]);
  });

  it("accepts a move that also sets allowVersionDowngrade", async () => {
    seedLiveClaims();
    const change = {
      ...twoLiveClaimMove,
      apply: [
        movedClaim("team-a", "orders", { allowVersionDowngrade: "8.9.1" }),
        movedClaim("team-b", "payments"),
      ],
    };
    expect((await applyReleaseMove(change)).ok).toBe(true);
  });

  it("refuses a claim that changes a field other than the move fields and writes nothing", async () => {
    seedLiveClaims();
    const result = await applyReleaseMove({
      ...twoLiveClaimMove,
      apply: [
        movedClaim("team-a", "orders"),
        movedClaim("team-b", "payments", { presetRef: "large" }),
      ],
    });
    expect(!result.ok && result.errors).toEqual([
      expect.objectContaining({
        code: "Invalid",
        ref: expect.objectContaining({ namespace: "team-b", name: "payments" }),
      }),
    ]);
    expect(fakeWriter.applied).toEqual([]);
  });

  it("refuses a claim that does not exist and writes nothing", async () => {
    fakeWriter.objects = [
      liveClaim("team-a", "orders", "4"),
    ] as typeof fakeWriter.objects;
    const result = await applyReleaseMove(twoLiveClaimMove);
    expect(!result.ok && result.errors).toEqual([
      expect.objectContaining({
        code: "Stale",
        message: "The CamundaClusterClaim payments does not exist.",
      }),
    ]);
    expect(fakeWriter.applied).toEqual([]);
  });

  it("refuses a claim that changed since the page read it", async () => {
    fakeWriter.objects = [
      liveClaim("team-a", "orders", "4"),
      liveClaim("team-b", "payments", "10"),
    ] as typeof fakeWriter.objects;
    const result = await applyReleaseMove(twoLiveClaimMove);
    expect(!result.ok && result.errors[0].code).toBe("Stale");
    expect(fakeWriter.applied).toEqual([]);
  });

  it.each([
    ["LogicalBackupRDBMS", undefined, "pending"],
    ["LogicalBackupRDBMS", "Pending", "pending"],
    ["LogicalBackupElasticsearch", "Running", "running"],
  ])(
    "refuses a cluster with a %s backup in phase %s and names it",
    async (kind, phase, state) => {
      seedLiveClaims(backup(kind, phase));
      const result = await applyReleaseMove(twoLiveClaimMove);
      expect(!result.ok && result.errors).toEqual([
        {
          code: "Stale",
          message: `Backup nightly of the cluster is ${state}. Move the cluster after it ends.`,
          ref: expect.objectContaining({
            namespace: "team-b",
            name: "payments",
          }),
        },
      ]);
      expect(fakeWriter.applied).toEqual([]);
    },
  );

  it("moves a cluster whose backups ended", async () => {
    seedLiveClaims(backup("LogicalBackupRDBMS", "Completed"));
    expect((await applyReleaseMove(twoLiveClaimMove)).ok).toBe(true);
  });

  it("gives Unavailable and writes nothing when the claims cannot be read", async () => {
    fakeWriter.readError = new Error("connection refused");
    const result = await applyReleaseMove(twoLiveClaimMove);
    expect(!result.ok && result.errors).toEqual([
      { code: "Unavailable", message: "connection refused" },
    ]);
    expect(fakeWriter.applied).toEqual([]);
  });

  it("refuses a move that writes the CamundaCluster", async () => {
    const result = await applyReleaseMove(clusterMove);
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });
});

describe("previewReleaseMove", () => {
  it("previews a move of two claims as one change", async () => {
    seedLiveClaims();
    await previewReleaseMove(twoLiveClaimMove);
    expect(fakeWriter.previewed).toEqual([twoLiveClaimMove]);
  });

  it("refuses a cluster with a running backup and previews nothing", async () => {
    seedLiveClaims(backup("LogicalBackupRDBMS", "Running"));
    const result = await previewReleaseMove(twoLiveClaimMove);
    expect(result.errors).toEqual([
      expect.objectContaining({
        code: "Stale",
        message:
          "Backup nightly of the cluster is running. Move the cluster after it ends.",
      }),
    ]);
    expect(fakeWriter.previewed).toEqual([]);
  });

  it("refuses a move that writes the CamundaCluster", async () => {
    const result = await previewReleaseMove(clusterMove);
    expect(result.errors).toEqual([
      expect.objectContaining({ code: "Invalid" }),
    ]);
    expect(fakeWriter.previewed).toEqual([]);
  });
});
