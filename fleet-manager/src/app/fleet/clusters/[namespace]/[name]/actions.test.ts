import { beforeEach, describe, expect, it, vi } from "vitest";
import { fakeWriter } from "../../../../../../test/fakes/writer";
import {
  applyBackupChange,
  applyClaimChange,
  previewClaimChange,
} from "./actions";

vi.mock(
  "@/lib/write/writer",
  () => import("../../../../../../test/fakes/writer"),
);

const change = (kind = "CamundaClusterClaim") => ({
  intent: "Scale the brokers of orders",
  apply: [
    {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind,
      metadata: { name: "orders", namespace: "team-a" },
      spec: { cluster: { zeebe: { replicas: 5 } } },
    },
  ],
  preconditions: { [`${kind}/team-a/orders` as const]: "41" },
});

beforeEach(() => fakeWriter.reset());

describe("applyClaimChange", () => {
  it("applyClaimChange writes the change through the writer", async () => {
    expect(await applyClaimChange(change())).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([change()]);
  });

  it("applyClaimChange refuses a change of another kind and writes nothing", async () => {
    const result = await applyClaimChange(change("Environment"));
    expect(result.ok).toBe(false);
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });
});

describe("previewClaimChange", () => {
  it("previewClaimChange previews through the writer", async () => {
    const refused = {
      objects: [],
      errors: [{ code: "Invalid" as const, message: "CEL", path: "spec" }],
    };
    fakeWriter.previews = [refused];
    expect(await previewClaimChange(change())).toEqual(refused);
    expect(fakeWriter.previewed).toEqual([change()]);
  });

  it("previewClaimChange refuses a change of another kind and previews nothing", async () => {
    const result = await previewClaimChange(change("Environment"));
    expect(result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.previewed).toEqual([]);
  });
});

describe("applyBackupChange", () => {
  const orders = { namespace: "team-a", name: "orders" };
  const backup = (cluster = "orders") => ({
    intent: "Back up orders",
    apply: [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "LogicalBackupElasticsearch",
        metadata: { name: "orders-manual-1", namespace: "team-a" },
        spec: { clusterRef: { name: cluster } },
      },
    ],
    create: ["LogicalBackupElasticsearch/team-a/orders-manual-1" as const],
    directOnly: true,
  });

  it("applyBackupChange writes a backup of the cluster straight to the cluster, also in Git mode", async () => {
    fakeWriter.reset("git");
    expect(await applyBackupChange(orders, backup())).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([backup()]);
  });

  it.each(["LogicalBackupElasticsearch", "LogicalBackupRDBMS"])(
    "applyBackupChange refuses a name that a %s of the namespace has, and writes nothing",
    async (kind) => {
      fakeWriter.existing = [`${kind}/team-a/orders-manual-1`];
      expect(await applyBackupChange(orders, backup())).toEqual({
        ok: false,
        errors: [
          {
            code: "Stale",
            message: `The ${kind} orders-manual-1 exists already. Pick another name.`,
            ref: {
              apiVersion: "core.camunda.io/v1",
              kind,
              namespace: "team-a",
              name: "orders-manual-1",
            },
          },
        ],
      });
      expect(fakeWriter.applied).toEqual([]);
    },
  );

  it("applyBackupChange deletes a backup whatever exists", async () => {
    fakeWriter.existing = ["LogicalBackupRDBMS/team-a/orders-manual-1"];
    const remove = {
      intent: "Delete backup orders-manual-1",
      apply: [],
      delete: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "LogicalBackupRDBMS",
          namespace: "team-a",
          name: "orders-manual-1",
        },
      ],
      directOnly: true,
    };
    expect(await applyBackupChange(orders, remove)).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([remove]);
  });

  it("applyBackupChange refuses a backup of another cluster and writes nothing", async () => {
    const result = await applyBackupChange(orders, backup("billing"));
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });

  it("applyBackupChange refuses a claim change and writes nothing", async () => {
    const result = await applyBackupChange(orders, change());
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });
});
