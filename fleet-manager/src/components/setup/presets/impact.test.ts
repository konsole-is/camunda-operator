import { describe, expect, it } from "vitest";
import { RESTART_ALL } from "@/components/cluster/impacts";
import type { CamundaClusterPresetSpec } from "@/generated/crds/camundaclusterpreset";
import type { Resource } from "@/lib/k8s/kinds";
import { presetFixtures } from "./fixtures";
import {
  CONNECTORS_RESTART,
  impactUnread,
  presetImpact,
  STORAGE_NOTES,
} from "./impact";
import type { PresetSources } from "./sources";

const fixtures = presetFixtures();
const medium = fixtures.clusterPresets[0];
const before = medium.spec;

function cluster(
  name: string,
  spec: Partial<Resource<"CamundaCluster">["spec"]>,
): Resource<"CamundaCluster"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaCluster",
    metadata: { namespace: "team-billing", name },
    spec: { presetRef: "medium", releaseRef: "camunda-8-9-0", ...spec },
  } as Resource<"CamundaCluster">;
}

// orders sets zeebe.replicas 1 itself and runs no Connectors. billing
// runs Connectors with basic auth.
const sources: PresetSources = {
  ...fixtures,
  clusters: [
    ...fixtures.clusters,
    cluster("billing", {
      platformConfigRef: "shared",
      connectors: { enabled: true },
    }),
  ],
};

const marks = (impact: { rows: { name: string; mark: string }[] }) =>
  impact.rows.map((r) => [r.name, r.mark]);

const input = {
  kind: "CamundaClusterPreset" as const,
  name: "medium",
  before,
  sources,
};

function withCluster(
  change: (c: CamundaClusterPresetSpec["cluster"]) => void,
): CamundaClusterPresetSpec {
  const spec = structuredClone(before);
  change(spec.cluster);
  return spec;
}

const replicas = (n: number) =>
  withCluster((c) => {
    c.zeebe = { ...c.zeebe, replicas: n };
  });
const storage = (size: string) =>
  withCluster((c) => {
    c.zeebe = { ...c.zeebe, storageSize: size };
  });
const rotated = withCluster((c) => {
  c.auth = {
    basic: { ...c.auth?.basic, passwordRotation: "2026-11-15T10:42Z" },
  };
});

describe("presetImpact of a cluster preset", () => {
  it("marks every inheriting cluster Pods roll when the fingerprint moves, also one that sets the field itself", () => {
    const impact = presetImpact({ ...input, after: replicas(5) });
    expect(impact.rows.map((r) => [r.name, r.mark])).toEqual([
      ["billing", "Pods roll"],
      ["orders", "Pods roll"],
    ]);
  });

  it("starts the notes with the restart line of the brokers when a cluster rolls", () => {
    expect(presetImpact({ ...input, after: replicas(5) }).notes[0]).toBe(
      RESTART_ALL,
    );
  });

  it("marks Connectors restart for a rotation that reaches a cluster with Connectors and basic auth", () => {
    const impact = presetImpact({ ...input, after: rotated });
    expect(impact.rows[0]).toMatchObject({
      name: "billing",
      mark: "Connectors restart",
      connectorsRestart: true,
    });
    expect(impact.notes).toEqual([CONNECTORS_RESTART]);
  });

  it("marks No roll for a rotation of a cluster without Connectors", () => {
    expect(marks(presetImpact({ ...input, after: rotated }))[1]).toEqual([
      "orders",
      "No roll",
    ]);
  });

  it("marks No roll for a rotation of a cluster with OIDC", () => {
    const oidc = {
      ...sources,
      platformConfigs: [
        {
          ...sources.platformConfigs[0],
          spec: { auth: { method: "oidc" } },
        } as Resource<"CamundaPlatformConfig">,
      ],
    };
    expect(
      marks(presetImpact({ ...input, after: rotated, sources: oidc }))[0],
    ).toEqual(["billing", "No roll"]);
  });

  it("marks No roll when the rotation that a cluster applied is set again", () => {
    const applied = {
      ...sources.clusters.find((c) => c.metadata.name === "billing"),
      status: { adminPassword: { rotation: "2026-11-15T10:42Z" } },
    } as Resource<"CamundaCluster">;
    const impact = presetImpact({
      ...input,
      after: rotated,
      sources: { ...sources, clusters: [applied] },
    });
    expect(marks(impact)).toEqual([["billing", "No roll"]]);
    expect(impact.notes).toEqual([]);
  });

  it("marks No roll when the rotation is cleared, because nothing rotates", () => {
    const impact = presetImpact({ ...input, before: rotated, after: before });
    expect(marks(impact)[0]).toEqual(["billing", "No roll"]);
    expect(impact.notes).toEqual([]);
  });

  it("marks Connectors restart for new annotations of a Connectors account that the operator creates", () => {
    const impact = presetImpact({
      ...input,
      after: withCluster((c) => {
        c.connectors = {
          serviceAccount: {
            annotations: { "eks.amazonaws.com/role-arn": "x" },
          },
        };
      }),
    });
    expect(marks(impact)).toEqual([
      ["billing", "Connectors restart"],
      ["orders", "No roll"],
    ]);
  });

  it("marks No roll for new annotations of a Connectors account that the user owns", () => {
    const owned = (annotations: Record<string, string>) =>
      withCluster((c) => {
        c.connectors = { serviceAccount: { create: false, annotations } };
      });
    const impact = presetImpact({
      ...input,
      before: owned({}),
      after: owned({ "example.com/owner": "team" }),
    });
    expect(marks(impact)[0]).toEqual(["billing", "No roll"]);
  });

  it("marks No roll when an account without annotations stops being created", () => {
    const account = (create: boolean) =>
      withCluster((c) => {
        c.connectors = { serviceAccount: { create } };
      });
    const impact = presetImpact({
      ...input,
      before: account(true),
      after: account(false),
    });
    expect(marks(impact)[0]).toEqual(["billing", "No roll"]);
  });

  it("marks Connectors restart for an Azure client ID on a Connectors account that the user owns", () => {
    const owned = (annotations: Record<string, string>) =>
      withCluster((c) => {
        c.connectors = { serviceAccount: { create: false, annotations } };
      });
    const impact = presetImpact({
      ...input,
      before: owned({}),
      after: owned({ "azure.workload.identity/client-id": "id" }),
    });
    expect(marks(impact)[0]).toEqual(["billing", "Connectors restart"]);
  });

  it("marks Unchanged a cluster that sets its own auth.basic, so the rotation does not reach it", () => {
    const own = cluster("own-auth", {
      auth: { basic: { adminEmail: "ops@example.com" } },
    });
    const impact = presetImpact({
      ...input,
      after: rotated,
      sources: { ...sources, clusters: [own] },
    });
    expect(impact.rows).toEqual([
      {
        kind: "CamundaCluster",
        namespace: "team-billing",
        name: "own-auth",
        mark: "Unchanged",
        connectorsRestart: false,
      },
    ]);
    expect(impact.notes).toEqual([]);
  });

  it("marks No roll for an admin email, with no restart", () => {
    const impact = presetImpact({
      ...input,
      after: withCluster((c) => {
        c.auth = { basic: { adminEmail: "team@example.com" } };
      }),
    });
    expect(impact.rows.map((r) => [r.mark, r.connectorsRestart])).toEqual([
      ["No roll", false],
      ["No roll", false],
    ]);
    expect(impact.notes).toEqual([]);
  });

  it("marks Unchanged for a change of the description only", () => {
    const impact = presetImpact({ ...input, after: before });
    expect(impact.rows.map((r) => r.mark)).toEqual(["Unchanged", "Unchanged"]);
    expect(impact.notes).toEqual([]);
  });

  it("names the shrink rule for a smaller storage size and not for a larger one", () => {
    expect(presetImpact({ ...input, after: storage("16Gi") }).notes).toContain(
      STORAGE_NOTES.CamundaClusterPreset.shrink,
    );
    expect(
      presetImpact({ ...input, after: storage("64Gi") }).notes,
    ).not.toContain(STORAGE_NOTES.CamundaClusterPreset.shrink);
  });

  it("names the shrink rule for a storage size written as a number", () => {
    const num = withCluster((c) => {
      c.zeebe = { ...c.zeebe, storageSize: 17179869184 };
    });
    expect(presetImpact({ ...input, after: num }).notes).toContain(
      STORAGE_NOTES.CamundaClusterPreset.shrink,
    );
  });

  it("names the shrink rule when a cleared storage size falls back to a smaller default", () => {
    const after = withCluster((c) => {
      c.zeebe = { ...c.zeebe };
      delete c.zeebe.storageSize;
    });
    expect(presetImpact({ ...input, after }).notes).toContain(
      STORAGE_NOTES.CamundaClusterPreset.shrink,
    );
  });

  it("names the storage class rule for a new class", () => {
    const after = withCluster((c) => {
      c.zeebe = { ...c.zeebe, storageClassName: "gp3" };
    });
    expect(presetImpact({ ...input, after }).notes).toContain(
      STORAGE_NOTES.CamundaClusterPreset.storageClass,
    );
  });
});

describe("presetImpact of an Elasticsearch or PostgreSQL preset", () => {
  const es = fixtures.elasticsearchPresets[0];
  const esUser = (
    name: string,
    spec: object,
  ): Resource<"ElasticsearchCluster"> =>
    ({
      apiVersion: "core.camunda.io/v1",
      kind: "ElasticsearchCluster",
      metadata: { namespace: "search", name },
      spec: { presetRef: "standard", ...spec },
    }) as Resource<"ElasticsearchCluster">;
  const esSources: PresetSources = {
    ...fixtures,
    elasticsearchClusters: [
      esUser("logs", {}),
      esUser("pinned", { storageSize: "64Gi" }),
    ],
  };

  it("marks Pods may roll for an Elasticsearch user whose merged spec changes, else Unchanged", () => {
    const impact = presetImpact({
      kind: "ElasticsearchClusterPreset",
      name: "standard",
      before: es.spec,
      after: { cluster: { ...es.spec.cluster, storageSize: "80Gi" } },
      sources: esSources,
    });
    expect(
      impact.rows.map((r) => [r.name, r.mark, r.connectorsRestart]),
    ).toEqual([
      ["logs", "Pods may roll", false],
      ["pinned", "Unchanged", false],
    ]);
    expect(impact.notes).toEqual([]);
  });

  it("names the Elasticsearch storage rules", () => {
    const impact = presetImpact({
      kind: "ElasticsearchClusterPreset",
      name: "standard",
      before: es.spec,
      after: {
        cluster: {
          ...es.spec.cluster,
          storageSize: "32Gi",
          storageClassName: "gp3",
        },
      },
      sources: esSources,
    });
    expect(impact.notes).toEqual([
      STORAGE_NOTES.ElasticsearchClusterPreset.shrink,
      STORAGE_NOTES.ElasticsearchClusterPreset.storageClass,
    ]);
  });

  it("names the PostgreSQL shrink rule for a smaller WAL size", () => {
    const pg = fixtures.databasePresets[1];
    const impact = presetImpact({
      kind: "DatabaseServerPreset",
      name: "standard",
      before: pg.spec,
      after: { server: { ...pg.spec.server, walStorageSize: "1Gi" } },
      sources: fixtures,
    });
    expect(impact.notes).toEqual([STORAGE_NOTES.DatabaseServerPreset.shrink]);
  });

  it("names the kept WAL volume for a cleared WAL size", () => {
    const pg = fixtures.databasePresets[1];
    const server = { ...pg.spec.server };
    delete server.walStorageSize;
    const impact = presetImpact({
      kind: "DatabaseServerPreset",
      name: "standard",
      before: pg.spec,
      after: { server },
      sources: fixtures,
    });
    expect(impact.notes).toEqual([STORAGE_NOTES.DatabaseServerPreset.cleared]);
  });
});

describe("presetImpact while a list cannot be read", () => {
  it("lists no resource and no restart while the clusters cannot be read", () => {
    const impact = presetImpact({
      ...input,
      after: replicas(5),
      unread: ["CamundaCluster"],
    });
    expect(impact.rows).toEqual([]);
    expect(impact.unread).toEqual(["CamundaCluster"]);
    expect(impact.notes).not.toContain(RESTART_ALL);
  });

  it("still names the storage rules that the change touches", () => {
    const impact = presetImpact({
      ...input,
      after: storage("16Gi"),
      unread: ["CamundaCluster"],
    });
    expect(impact.notes).toEqual([STORAGE_NOTES.CamundaClusterPreset.shrink]);
  });
});

describe("impactUnread", () => {
  it("names the users, the releases, and the platform configs of a cluster preset that are not read or not current", () => {
    expect(
      impactUnread(
        "CamundaClusterPreset",
        [
          { kind: "CamundaRelease", message: "forbidden" },
          { kind: "PluginRequest", message: "forbidden" },
        ],
        ["CamundaCluster", "DatabaseServer", "CamundaPlatformConfig"],
      ),
    ).toEqual(["CamundaCluster", "CamundaRelease", "CamundaPlatformConfig"]);
  });

  it("names only the users of an Elasticsearch or PostgreSQL preset", () => {
    expect(
      impactUnread(
        "ElasticsearchClusterPreset",
        [],
        ["CamundaRelease", "ElasticsearchCluster"],
      ),
    ).toEqual(["ElasticsearchCluster"]);
    expect(
      impactUnread(
        "DatabaseServerPreset",
        [{ kind: "CamundaCluster", message: "gone" }],
        [],
      ),
    ).toEqual([]);
  });
});
