import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import {
  namedErrors,
  planMove,
  rolloutRows,
  unreadMoveText,
  unreadRolloutKinds,
} from "./rollout";
import {
  cluster,
  clusterClaim,
  rolloutData,
  targetRelease,
  VERSION_SKIP_MESSAGE,
} from "./rollout-fixtures";

const rows = rolloutRows(targetRelease, rolloutData());
const row = (name: string) => rows.find((r) => r.name === name)!;

const release = (name: string, version: string) =>
  ({
    ...targetRelease,
    metadata: { name },
    spec: { version },
  }) as Resource<"CamundaRelease">;

const withClaim = (claim: Resource<"CamundaClusterClaim">) => {
  const data = rolloutData();
  return {
    ...data,
    claims: data.claims.map((c) =>
      c.metadata.namespace === claim.metadata.namespace &&
      c.metadata.name === claim.metadata.name
        ? claim
        : c,
    ),
  };
};

// refusedOnTarget gives rolloutData with team-pay/payments moved to
// targetRelease and refused by the operator for reason.
const refusedOnTarget = (reason: string, claimSpec: object = {}) => {
  const data = rolloutData();
  return {
    ...data,
    clusters: data.clusters.map((c) =>
      c.metadata?.name === "payments"
        ? cluster("team-pay", "payments", {
            version: "8.9.4",
            releaseRef: "camunda-8-9-1",
            conditions: [
              {
                type: "Ready",
                status: "False",
                reason,
                message: "the effective version is refused",
              },
            ],
          })
        : c,
    ),
    claims: data.claims.map((c) =>
      c.metadata.name === "payments"
        ? clusterClaim("team-pay", "payments", {
            releaseRef: "camunda-8-9-1",
            ...claimSpec,
          })
        : c,
    ),
  };
};
const refusedRow = (reason: string, claimSpec: object = {}) =>
  rolloutRows(targetRelease, refusedOnTarget(reason, claimSpec)).find(
    (r) => r.name === "payments",
  )!;

describe("rolloutRows", () => {
  it("shows name, environment, namespace, current release, and running version", () => {
    expect(row("orders")).toMatchObject({
      key: "team-orders/orders",
      name: "orders",
      namespace: "team-orders",
      environment: "production",
      currentRelease: "camunda-8-9-0",
      runningVersion: "8.9.0",
    });
  });

  it.each([
    ["orders", "canMove", "move"],
    ["stage", "notAllowed", "openEnvironments"],
    ["payments", "downgrade", "confirmDowngrade"],
    ["ledger", "skipsMinor", undefined],
    ["billing", "backupRunning", undefined],
    ["legacy", "notManaged", undefined],
    ["orphan", "noClaim", undefined],
    ["current", "onRelease", undefined],
    ["paused", "waitsForResume", "move"],
    ["fresh", "waitsForVersion", "move"],
  ])(
    "maps the cluster %s to the status %s and the action %s",
    (name, kind, action) => {
      expect(row(name).status.kind).toBe(kind);
      expect(row(name).action).toBe(action);
    },
  );

  it("names the backup that blocks the move", () => {
    expect(row("billing").status.text).toBe(
      "Backup billing-nightly is pending. The move waits until it ends.",
    );
  });

  it("shows the message of a VersionSkipRefused refusal", () => {
    expect(row("ledger").refusal).toBe(VERSION_SKIP_MESSAGE);
    expect(row("orders").refusal).toBeUndefined();
  });

  it("shows a refused downgrade to the release with its message and offers Confirm downgrade", () => {
    const payments = refusedRow("VersionDowngradeRefused");
    expect(payments.status).toEqual({
      kind: "downgradeRefused",
      to: "8.9.1",
      text: "The operator refused the move: the effective version is refused",
    });
    expect(payments.action).toBe("confirmDowngrade");
    expect(payments.refusal).toBeUndefined();
  });

  it("offers no confirm for a refused downgrade that the claim already allows", () => {
    const payments = refusedRow("VersionDowngradeRefused", {
      allowVersionDowngrade: "8.9.1",
    });
    expect(payments.status.kind).toBe("refused");
    expect(payments.action).toBeUndefined();
  });

  it("shows a refused skip to the release with its message and no action", () => {
    const payments = refusedRow("VersionSkipRefused");
    expect(payments.status).toEqual({
      kind: "refused",
      text: "The operator refused the move: the effective version is refused",
    });
    expect(payments.action).toBeUndefined();
    expect(payments.refusal).toBeUndefined();
  });

  it("checks against status.management.version when spec.version differs from the release", () => {
    const data = rolloutData();
    const clusters = data.clusters.map((c) =>
      c.metadata?.name === "orders"
        ? ({
            ...c,
            spec: { ...c.spec, version: "8.9.0" },
            status: { management: { version: "8.8.3" } },
          } as typeof c)
        : c,
    );
    const environments = data.environments.map((e) => ({
      ...e,
      spec: { ...e.spec, releases: { allowed: ["camunda-8-10-0"] } },
    }));
    expect(
      rolloutRows(release("camunda-8-10-0", "8.10.0"), {
        ...data,
        clusters,
        environments,
      }).find((r) => r.name === "orders")?.status.kind,
    ).toBe("skipsMinor");
  });

  it("counts the release of the cluster when the claim names none", () => {
    const data = rolloutData();
    const claims = data.claims.map((c) =>
      c.metadata.name === "orders"
        ? { ...c, spec: { ...c.spec, releaseRef: undefined } }
        : c,
    );
    const clusters = data.clusters.map((c) =>
      c.metadata?.name === "orders"
        ? { ...c, spec: { ...c.spec, releaseRef: "camunda-8-9-1" } }
        : c,
    );
    const orders = rolloutRows(targetRelease, {
      ...data,
      claims,
      clusters,
    }).find((r) => r.name === "orders");
    expect(orders).toMatchObject({
      currentRelease: "camunda-8-9-1",
      status: { kind: "onRelease", text: "Runs this release." },
    });
    expect(orders?.action).toBeUndefined();
  });

  it("gives a cluster whose claim is gone no action", () => {
    expect(row("orphan")).toMatchObject({
      status: { kind: "noClaim", text: "The claim orphan is missing." },
    });
    expect(row("orphan").action).toBeUndefined();
  });

  it("shows a cluster of a namespace without an environment with none", () => {
    const data = rolloutData({
      namespaces: rolloutData().namespaces.filter(
        (n) => n.metadata?.name !== "team-orders",
      ),
    });
    expect(
      rolloutRows(targetRelease, data).find((r) => r.name === "orders"),
    ).not.toHaveProperty("environment");
  });

  it("orders the rows by namespace, then name", () => {
    expect(rows.map((r) => r.key)).toEqual([...rows.map((r) => r.key)].sort());
  });

  it("offers no move while a list that the move checks is not read", () => {
    const unread = rolloutRows(targetRelease, {
      ...rolloutData(),
      errors: [{ kind: "LogicalBackupRDBMS", message: "forbidden" }],
      stale: ["PluginRequest"],
    });
    for (const r of unread) {
      expect(r.status).toEqual({
        kind: "unread",
        text: "The checks wait until the fleet manager has read LogicalBackupRDBMS, PluginRequest.",
      });
      expect(r.action).toBeUndefined();
    }
  });

  it("keeps the checks for a list that the move does not read", () => {
    const read = rolloutRows(targetRelease, {
      ...rolloutData(),
      stale: ["ElasticsearchCluster"],
    });
    expect(read.find((r) => r.name === "orders")?.status.kind).toBe("canMove");
  });

  it("reads a live object with values of the wrong type without a throw", () => {
    const data = withClaim({
      ...clusterClaim("team-pay", "fresh"),
      spec: { releaseRef: 7, presetRef: null },
    } as unknown as Resource<"CamundaClusterClaim">);
    const clusters = data.clusters.map((c) =>
      c.metadata?.name === "fresh"
        ? ({
            ...c,
            spec: { ...c.spec, releaseRef: null },
            status: { management: { version: 8 }, conditions: "Ready" },
          } as unknown as Resource<"CamundaCluster">)
        : c,
    );
    const fresh = rolloutRows(targetRelease, { ...data, clusters }).find(
      (r) => r.name === "fresh",
    );
    expect(fresh).not.toHaveProperty("currentRelease");
    expect(fresh).not.toHaveProperty("runningVersion");
    expect(fresh?.status.kind).toBe("waitsForVersion");
  });
});

describe("unreadRolloutKinds", () => {
  it("names each unread kind that the move reads, once", () => {
    expect(
      unreadRolloutKinds({
        errors: [
          { kind: "CamundaClusterClaim", message: "forbidden" },
          { kind: "DatabaseServer", message: "forbidden" },
        ],
        stale: ["CamundaClusterClaim", "LogicalBackupElasticsearch"],
      }),
    ).toEqual([
      "CamundaClusterClaim",
      "DatabaseServer",
      "LogicalBackupElasticsearch",
    ]);
    expect(unreadRolloutKinds({})).toEqual([]);
  });

  it("says which lists a move waits for", () => {
    expect(unreadMoveText(["PluginRequest", "LogicalBackupRDBMS"])).toBe(
      "A move is possible when the fleet manager has read PluginRequest, LogicalBackupRDBMS.",
    );
  });
});

describe("planMove", () => {
  it("sets spec.releaseRef on each claim, in one change with a precondition each", () => {
    const plan = planMove(
      targetRelease,
      [
        { row: row("orders"), downgrade: false },
        { row: row("fresh"), downgrade: false },
      ],
      rolloutData(),
    );
    expect(plan.errors).toEqual([]);
    expect(
      plan.change?.apply.map((o) => [
        o.kind,
        o.metadata.namespace,
        o.metadata.name,
        (o.spec as { releaseRef?: string }).releaseRef,
      ]),
    ).toEqual([
      ["CamundaClusterClaim", "team-orders", "orders", "camunda-8-9-1"],
      ["CamundaClusterClaim", "team-pay", "fresh", "camunda-8-9-1"],
    ]);
    expect(plan.change?.preconditions).toEqual({
      "CamundaClusterClaim/team-orders/orders": "4",
      "CamundaClusterClaim/team-pay/fresh": "4",
    });
    expect(plan.change?.intent).toBe(
      "Move 2 clusters to the release camunda-8-9-1",
    );
  });

  it("names the one cluster in the intent of a move of one", () => {
    const plan = planMove(
      targetRelease,
      [{ row: row("orders"), downgrade: false }],
      rolloutData(),
    );
    expect(plan.change?.intent).toBe(
      "Move the cluster team-orders/orders to the release camunda-8-9-1",
    );
  });

  it("keeps every other field of the claim", () => {
    const plan = planMove(
      targetRelease,
      [{ row: row("orders"), downgrade: false }],
      rolloutData(),
    );
    expect(plan.change?.apply[0].spec).toEqual({
      ...row("orders").claim?.spec,
      releaseRef: "camunda-8-9-1",
    });
  });

  it("sets allowVersionDowngrade to the target version on a confirmed downgrade, and writes no CamundaCluster", () => {
    const plan = planMove(
      targetRelease,
      [{ row: row("payments"), downgrade: true }],
      rolloutData(),
    );
    expect(plan.errors).toEqual([]);
    expect(plan.change?.apply).toHaveLength(1);
    expect(plan.change?.apply[0]).toMatchObject({
      kind: "CamundaClusterClaim",
      spec: { releaseRef: "camunda-8-9-1", allowVersionDowngrade: "8.9.1" },
    });
  });

  it("sets only allowVersionDowngrade on a confirmed refused downgrade", () => {
    const data = refusedOnTarget("VersionDowngradeRefused");
    const payments = rolloutRows(targetRelease, data).find(
      (r) => r.name === "payments",
    )!;
    const plan = planMove(
      targetRelease,
      [{ row: payments, downgrade: true }],
      data,
    );
    expect(plan.errors).toEqual([]);
    expect(plan.change?.apply[0]).toMatchObject({
      spec: { releaseRef: "camunda-8-9-1", allowVersionDowngrade: "8.9.1" },
    });
  });

  it("refuses a downgrade that the user did not confirm", () => {
    const plan = planMove(
      targetRelease,
      [{ row: row("payments"), downgrade: false }],
      rolloutData(),
    );
    expect(plan.change).toBeUndefined();
    expect(plan.errors).toEqual([
      expect.stringMatching(
        /^team-pay\/payments: 8\.9\.4 to 8\.9\.1 is a downgrade/,
      ),
    ]);
  });

  it("keeps the allowVersionDowngrade of a claim that moves up", () => {
    const data = withClaim(
      clusterClaim("team-orders", "orders", { allowVersionDowngrade: "8.8.0" }),
    );
    const orders = rolloutRows(targetRelease, data).find(
      (r) => r.name === "orders",
    )!;
    const plan = planMove(
      targetRelease,
      [{ row: orders, downgrade: false }],
      data,
    );
    expect(plan.change?.apply[0].spec).toMatchObject({
      releaseRef: "camunda-8-9-1",
      allowVersionDowngrade: "8.8.0",
    });
  });

  it("lists each cluster with the broker restart and what moves with its claim", () => {
    const plan = planMove(
      targetRelease,
      [
        { row: row("orders"), downgrade: false },
        { row: row("fresh"), downgrade: false },
      ],
      rolloutData(),
    );
    expect(plan.impacts).toEqual([
      {
        cluster: "team-orders/orders",
        lines: expect.arrayContaining([
          "The brokers restart one at a time. The other processes restart.",
          "Elasticsearch moves to the release too.",
          "Optimize moves to the version of the release.",
        ]),
      },
      {
        cluster: "team-pay/fresh",
        lines: [
          "The brokers restart one at a time. The other processes restart.",
        ],
      },
    ]);
  });

  it("says that a database server of the claim on another PostgreSQL major refuses the release", () => {
    const pg = (version: string) =>
      ({
        ...targetRelease,
        spec: { version: "8.9.1", databaseServer: { version } },
      }) as Resource<"CamundaRelease">;
    const data = rolloutData({
      requests: [
        {
          apiVersion: "plugins.camunda.io/v1alpha1",
          kind: "PluginRequest",
          metadata: {
            namespace: "team-orders",
            name: "orders-rdbms",
            labels: { "fleet.camunda.io/claim": "orders" },
          },
          spec: {},
          status: {
            resources: [{ kind: "DatabaseServer", name: "orders-pg" }],
          },
        } as unknown as Resource<"PluginRequest">,
      ],
      databaseServers: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "DatabaseServer",
          metadata: { namespace: "team-orders", name: "orders-pg" },
          spec: { releaseRef: "camunda-8-9-0" },
          status: { version: "16" },
        } as unknown as Resource<"DatabaseServer">,
      ],
    });
    const lines = (version: string) =>
      planMove(pg(version), [{ row: row("orders"), downgrade: false }], data)
        .impacts[0].lines;
    expect(lines("17")).toContain(
      "The database server orders-pg runs PostgreSQL 16 and refuses PostgreSQL 17 of the release. It keeps PostgreSQL 16.",
    );
    expect(lines("16").join(" ")).not.toContain("PostgreSQL");
  });

  it("gives no change and names the claim when validateClusterClaim refuses it", () => {
    const data = withClaim(
      clusterClaim("team-orders", "orders", { cluster: { version: "8.9.1" } }),
    );
    const orders = rolloutRows(targetRelease, data).find(
      (r) => r.name === "orders",
    )!;
    const plan = planMove(
      targetRelease,
      [
        { row: orders, downgrade: false },
        { row: row("fresh"), downgrade: false },
      ],
      data,
    );
    expect(plan.change).toBeUndefined();
    expect(plan.errors).toEqual([
      "team-orders/orders: The release sets this field. Use allowVersionDowngrade for a downgrade.",
    ]);
  });

  it("names a claim with a value of the wrong type in the batch, and plans without a throw", () => {
    const broken = {
      ...clusterClaim("team-pay", "fresh"),
      spec: {
        presetRef: 7,
        releaseRef: null,
        attachments: null,
        plugins: null,
        deletionPolicy: 3,
        cluster: { version: 42 },
      },
    } as unknown as Resource<"CamundaClusterClaim">;
    const data = withClaim(broken);
    const fresh = rolloutRows(targetRelease, data).find(
      (r) => r.name === "fresh",
    )!;
    const plan = planMove(
      targetRelease,
      [
        { row: row("orders"), downgrade: false },
        { row: fresh, downgrade: false },
      ],
      data,
    );
    expect(plan.change).toBeUndefined();
    expect(plan.errors.length).toBeGreaterThan(0);
    for (const error of plan.errors)
      expect(error).toMatch(/^team-pay\/fresh: /);
  });

  it("refuses a row that offers no move", () => {
    const plan = planMove(
      targetRelease,
      [{ row: row("billing"), downgrade: false }],
      rolloutData(),
    );
    expect(plan.change).toBeUndefined();
    expect(plan.errors).toEqual([
      "team-pay/billing: Backup billing-nightly is pending. The move waits until it ends.",
    ]);
  });

  it("refuses a managed cluster whose claim cannot be opened", () => {
    const orphan = {
      ...row("orphan"),
      status: { kind: "canMove", text: "Can move." } as const,
      action: "move" as const,
      claim: clusterClaim("team-pay", "orphan", {}),
      cluster: cluster("team-pay", "orphan", { managed: false }),
    };
    const plan = planMove(
      targetRelease,
      [{ row: orphan, downgrade: false }],
      rolloutData(),
    );
    expect(plan.errors).toEqual([
      "team-pay/orphan: The claim cannot be changed.",
    ]);
  });
});

describe("namedErrors", () => {
  it("names the claim of each error", () => {
    expect(
      namedErrors({
        ok: false,
        errors: [
          {
            code: "Stale",
            message:
              "The CamundaClusterClaim payments changed since you opened it.",
            ref: {
              apiVersion: "fleet.camunda.io/v1alpha1",
              kind: "CamundaClusterClaim",
              namespace: "team-pay",
              name: "payments",
            },
          },
          { code: "Unavailable", message: "The API server did not answer." },
        ],
      }),
    ).toEqual({
      ok: false,
      errors: [
        expect.objectContaining({
          message:
            "team-pay/payments: The CamundaClusterClaim payments changed since you opened it.",
        }),
        { code: "Unavailable", message: "The API server did not answer." },
      ],
    });
  });

  it("puts the path of an error behind the claim name", () => {
    const result = namedErrors({
      ok: false,
      errors: [
        {
          code: "Invalid",
          message: "Unsupported value",
          path: "spec.releaseRef",
          ref: {
            apiVersion: "fleet.camunda.io/v1alpha1",
            kind: "CamundaClusterClaim",
            namespace: "team-pay",
            name: "fresh",
          },
        },
      ],
    });
    expect(!result.ok && result.errors[0]).toEqual({
      code: "Invalid",
      message: "team-pay/fresh: spec.releaseRef: Unsupported value",
      ref: expect.objectContaining({ name: "fresh" }),
    });
  });

  it("names the field manager that owns a field and the remedy for a Conflict", () => {
    const ref = {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "CamundaClusterClaim",
      namespace: "team-orders",
      name: "orders",
    };
    const result = namedErrors({
      ok: false,
      errors: [
        {
          code: "Conflict",
          message: 'conflict with "kubectl-client-side-apply"',
          path: "spec.releaseRef",
          manager: "kubectl-client-side-apply",
          ref,
        },
        {
          code: "Conflict",
          message: "conflict",
          path: "spec.releaseRef",
          ref,
        },
      ],
    });
    expect(!result.ok && result.errors.map((e) => e.message)).toEqual([
      'team-orders/orders: spec.releaseRef belongs to the field manager "kubectl-client-side-apply". Change it with that tool, or leave this cluster out of the move.',
      "team-orders/orders: spec.releaseRef belongs to another field manager. Change it with that tool, or leave this cluster out of the move.",
    ]);
  });

  it("returns an ok result as it is", () => {
    expect(namedErrors({ ok: true, mode: "direct" })).toEqual({
      ok: true,
      mode: "direct",
    });
  });
});
