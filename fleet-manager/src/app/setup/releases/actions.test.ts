import { beforeEach, describe, expect, it, vi } from "vitest";
import { releaseFixtures } from "@/components/setup/releases/fixtures";
import { RELEASE_SOURCE_KINDS } from "@/components/setup/releases/source-kinds";
import type { ReleasesData } from "@/components/setup/releases/sources";
import type { Change } from "@/lib/write/types";
import { fakeWriter } from "../../../../test/fakes/writer";
import {
  applyReleaseChange,
  applyReleaseMove,
  previewReleaseChange,
  previewReleaseMove,
} from "./actions";

vi.mock("@/lib/write/writer", () => import("../../../../test/fakes/writer"));

const cache = vi.hoisted(() => ({
  lists: {} as Record<string, unknown[]>,
  failing: [] as string[],
}));
vi.mock("@/lib/k8s/read", () => ({
  list: async (kind: string) => {
    if (cache.failing.includes(kind)) throw new Error(`${kind} is forbidden`);
    return cache.lists[kind] ?? [];
  },
}));

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

beforeEach(() => {
  fakeWriter.reset();
  cache.failing = [];
  const onServer = releaseFixtures();
  cache.lists = Object.fromEntries(
    Object.entries(RELEASE_SOURCE_KINDS).map(([key, kind]) => [
      kind,
      onServer[key as keyof ReleasesData] as unknown[],
    ]),
  );
});

// remove deletes the releases names, with a precondition and a UID for each.
function remove(...names: string[]): Change {
  return {
    intent: "Delete releases",
    apply: [],
    delete: names.map((name) => ({
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaRelease",
      name,
    })),
    preconditions: Object.fromEntries(
      names.map((name) => [`CamundaRelease//${name}`, "41"]),
    ),
    uids: Object.fromEntries(
      names.map((name) => [`CamundaRelease//${name}`, `uid-${name}`]),
    ),
  };
}

const IN_USE =
  "Delete is possible when no cluster, Elasticsearch cluster, database server, or claim names this release and no environment allows it.";

describe("the delete guard of applyReleaseChange", () => {
  it("deletes an unused release that no environment allows, with the precondition and the UID of the page", async () => {
    const change = remove("camunda-8-9-1");
    expect(await applyReleaseChange(change)).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([change]);
  });

  it("refuses a delete of a release in use, names its users, and writes nothing", async () => {
    const result = await applyReleaseChange(remove("camunda-8-9-0"));
    expect(!result.ok && result.errors).toEqual([
      {
        code: "Forbidden",
        message: `${IN_USE} Used by CamundaCluster team-orders/orders, ElasticsearchCluster team-orders/orders-search, CamundaClusterClaim team-orders/orders, Environment production, Environment staging.`,
        ref: {
          apiVersion: "core.camunda.io/v1",
          kind: "CamundaRelease",
          name: "camunda-8-9-0",
        },
      },
    ]);
    expect(fakeWriter.applied).toEqual([]);
  });

  it("refuses a delete of a release that only a claim names", async () => {
    const result = await applyReleaseChange(remove("camunda-8-10-0"));
    expect(!result.ok && result.errors.map((e) => e.message)).toEqual([
      `${IN_USE} Used by CamundaClusterClaim team-new/billing.`,
    ]);
    expect(fakeWriter.applied).toEqual([]);
  });

  it("refuses a delete of a release that only an environment allows", async () => {
    const result = await applyReleaseChange(remove("camunda-8-8-5"));
    expect(!result.ok && result.errors.map((e) => e.message)).toEqual([
      `${IN_USE} Used by Environment staging.`,
    ]);
  });

  it("refuses a bulk delete when one release is in use, and deletes none of them", async () => {
    const result = await applyReleaseChange(
      remove("camunda-8-9-1", "camunda-8-10-0"),
    );
    expect(!result.ok && result.errors.map((e) => e.ref?.name)).toEqual([
      "camunda-8-10-0",
    ]);
    expect(fakeWriter.applied).toEqual([]);
  });

  it("refuses a delete of an unused release while a list of the page cannot be read, and names the list", async () => {
    cache.failing = ["CamundaClusterClaim"];
    const result = await applyReleaseChange(remove("camunda-8-9-1"));
    expect(!result.ok && result.errors).toMatchObject([
      {
        code: "Forbidden",
        message:
          "Delete is possible when the fleet manager can read CamundaClusterClaim.",
      },
    ]);
    expect(fakeWriter.applied).toEqual([]);
  });

  it("writes an update of a release in use while a list cannot be read", async () => {
    cache.failing = ["CamundaClusterClaim"];
    expect((await applyReleaseChange(releaseChange())).ok).toBe(true);
  });
});

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

  it("refuses a delete of a release in use as the write does, and previews nothing", async () => {
    const preview = await previewReleaseChange(
      remove("camunda-8-9-1", "camunda-8-8-5"),
    );
    expect(preview.objects).toEqual([]);
    expect(preview.errors.map((e) => [e.code, e.message])).toEqual([
      ["Forbidden", `${IN_USE} Used by Environment staging.`],
    ]);
    expect(fakeWriter.previewed).toEqual([]);
  });

  it("refuses a delete while a list of the page cannot be read", async () => {
    cache.failing = ["Environment"];
    const preview = await previewReleaseChange(remove("camunda-8-9-1"));
    expect(preview.errors.map((e) => e.message)).toEqual([
      "Delete is possible when the fleet manager can read Environment.",
    ]);
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
