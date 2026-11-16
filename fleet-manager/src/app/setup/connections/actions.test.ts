import { beforeEach, describe, expect, it, vi } from "vitest";
import { CONNECTION_SOURCE_KINDS } from "@/components/setup/connections/source-kinds";
import { connectionFixtures } from "@/components/setup/connections/fixtures";
import type { ConnectionSources } from "@/components/setup/connections/sources";
import type { Change } from "@/lib/write/types";
import { fakeWriter } from "../../../../test/fakes/writer";
import { applyContractChange, previewContractChange } from "./actions";

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

const CORE = "core.camunda.io/v1";

// sourcesOnServer returns the fixtures with the namespace camunda-system
// that no Environment holds, an unused contract created by hand in it, and a
// Management Identity contract created by hand. Each contract has the
// resourceVersion of the changes below.
function sourcesOnServer(): ConnectionSources {
  const sources = connectionFixtures();
  sources.namespaces = [
    ...sources.namespaces,
    { metadata: { name: "camunda-system" } },
  ];
  sources.objectStorageConfigs.push({
    ...sources.objectStorageConfigs[0],
    metadata: { namespace: "camunda-system", name: "archive" },
  });
  sources.managementAuthConfigs.push({
    ...sources.managementAuthConfigs[0],
    metadata: { name: "keycloak-auth" },
  });
  for (const contract of [
    ...sources.objectStorageConfigs,
    ...sources.secondaryStorageConfigs,
    ...sources.databaseServerConfigs,
    ...sources.databaseConfigs,
    ...sources.managementAuthConfigs,
  ])
    contract.metadata = { ...contract.metadata, resourceVersion: "41" };
  return sources;
}

beforeEach(() => {
  fakeWriter.reset();
  cache.failing = [];
  const sources = sourcesOnServer();
  cache.lists = Object.fromEntries(
    Object.entries(CONNECTION_SOURCE_KINDS).map(([key, kind]) => [
      kind,
      sources[key as keyof ConnectionSources] as unknown[],
    ]),
  );
});

function update(
  kind: string,
  namespace: string | undefined,
  name: string,
): Change {
  const key = `${kind}/${namespace ?? ""}/${name}` as const;
  return {
    intent: `Change the contract ${name}`,
    apply: [
      {
        apiVersion: CORE,
        kind,
        metadata: { name, ...(namespace !== undefined && { namespace }) },
        spec: {},
      },
    ],
    preconditions: { [key]: "41" },
    force: true,
  };
}

function remove(
  kind: string,
  namespace: string | undefined,
  name: string,
): Change {
  const key = `${kind}/${namespace ?? ""}/${name}` as const;
  return {
    intent: `Delete the contract ${name}`,
    apply: [],
    delete: [
      {
        apiVersion: CORE,
        kind,
        name,
        ...(namespace !== undefined && { namespace }),
      },
    ],
    preconditions: { [key]: "41" },
    uids: { [key]: "u-1" },
  };
}

async function refusedWith(change: Change, code: string) {
  const result = await applyContractChange(change);
  expect(result.ok).toBe(false);
  expect(fakeWriter.applied).toEqual([]);
  return (result.ok ? [] : result.errors).map((e) => {
    expect(e.code).toBe(code);
    return e.message;
  });
}

describe("applyContractChange", () => {
  it("writes a contract in a namespace of the cluster that no Environment holds", async () => {
    const change = update("ObjectStorageConfig", "camunda-system", "archive");
    expect(await applyContractChange(change)).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([change]);
  });

  it("passes the precondition of the page to the writer", async () => {
    await applyContractChange(
      update("ObjectStorageConfig", "team-a", "backups"),
    );
    expect(fakeWriter.applied[0].preconditions).toEqual({
      "ObjectStorageConfig/team-a/backups": "41",
    });
  });

  it("writes a Management Identity contract without namespace", async () => {
    const change = update("ManagementAuthConfig", undefined, "keycloak-auth");
    expect((await applyContractChange(change)).ok).toBe(true);
    expect(fakeWriter.applied).toEqual([change]);
  });

  it("refuses a contract in a namespace the cluster does not have, and writes nothing", async () => {
    await refusedWith(
      update("ObjectStorageConfig", "team-z", "backups"),
      "Invalid",
    );
  });

  it("refuses another kind, such as a CamundaCluster, and writes nothing", async () => {
    await refusedWith(update("CamundaCluster", "team-a", "orders"), "Invalid");
  });

  it("refuses a delete without precondition", async () => {
    const change = remove("DatabaseConfig", "team-a", "orders-db");
    delete change.preconditions;
    await refusedWith(change, "Invalid");
  });

  it("refuses a create", async () => {
    const change = update("ObjectStorageConfig", "team-a", "new-bucket");
    delete change.preconditions;
    change.create = ["ObjectStorageConfig/team-a/new-bucket"];
    await refusedWith(change, "Invalid");
  });

  it("refuses every namespaced contract when the namespaces cannot be read", async () => {
    cache.failing = ["Namespace"];
    await refusedWith(
      update("ObjectStorageConfig", "team-a", "backups"),
      "Invalid",
    );
  });

  it.each([
    [
      "SecondaryStorageConfig",
      "team-a",
      "orders-es",
      "ElasticsearchCluster orders-es",
    ],
    ["ObjectStorageConfig", "team-a", "orders-backup", "Registration aws-s3"],
    [
      "ManagementAuthConfig",
      undefined,
      "plane-a-optimize",
      "Management plane platform/plane-a",
    ],
  ])(
    "refuses a change of a %s that %s/%s has a writer for",
    async (kind, namespace, name, writer) => {
      const messages = await refusedWith(
        update(kind, namespace, name),
        "Forbidden",
      );
      expect(messages).toEqual([
        `${kind} ${namespace ? `${namespace}/` : ""}${name} is managed by ${writer}. Change it there.`,
      ]);
    },
  );

  it("refuses a delete of a contract that a registration writes", async () => {
    await refusedWith(
      remove("ObjectStorageConfig", "team-a", "orders-backup"),
      "Forbidden",
    );
  });

  it("names the contract that it refuses", async () => {
    const result = await applyContractChange(
      update("ObjectStorageConfig", "team-a", "orders-backup"),
    );
    expect(!result.ok && result.errors[0].ref).toEqual({
      apiVersion: CORE,
      kind: "ObjectStorageConfig",
      namespace: "team-a",
      name: "orders-backup",
    });
  });

  it("writes a contract kept from a registration", async () => {
    expect(
      (
        await applyContractChange(
          update("ObjectStorageConfig", "team-a", "old-backup"),
        )
      ).ok,
    ).toBe(true);
  });

  it("refuses a contract kept from a registration while the requests cannot be read", async () => {
    cache.failing = ["PluginRequest"];
    await refusedWith(
      update("ObjectStorageConfig", "team-a", "old-backup"),
      "Forbidden",
    );
  });

  it("refuses a change while the contracts of the kind cannot be read", async () => {
    cache.failing = ["ObjectStorageConfig"];
    const messages = await refusedWith(
      update("ObjectStorageConfig", "team-a", "backups"),
      "Forbidden",
    );
    expect(messages[0]).toContain("ObjectStorageConfig");
  });

  it("deletes an unused contract created by hand", async () => {
    const change = remove("DatabaseConfig", "team-a", "orders-db");
    expect((await applyContractChange(change)).ok).toBe(true);
    expect(fakeWriter.applied).toEqual([change]);
  });

  it("refuses a delete of a contract in use, and names its users", async () => {
    const messages = await refusedWith(
      remove("ObjectStorageConfig", "team-a", "backups"),
      "Forbidden",
    );
    expect(messages).toEqual([
      "Delete is possible when no resource uses ObjectStorageConfig team-a/backups. Used by CamundaCluster team-a/orders.",
    ]);
  });

  it("refuses a delete while a kind that can use the contract cannot be read", async () => {
    cache.failing = ["CamundaManagementCluster"];
    const messages = await refusedWith(
      remove("DatabaseConfig", "team-a", "orders-db"),
      "Forbidden",
    );
    expect(messages).toEqual([
      "Delete is possible when the fleet manager can read CamundaManagementCluster.",
    ]);
  });

  it("refuses a change while a kind that can use the contract cannot be read", async () => {
    cache.failing = ["CamundaManagementCluster"];
    const messages = await refusedWith(
      update("DatabaseConfig", "team-a", "orders-db"),
      "Forbidden",
    );
    expect(messages).toEqual([
      "A change is possible when the fleet manager can read CamundaManagementCluster.",
    ]);
  });

  it.each([
    ["change", update("ObjectStorageConfig", "team-a", "old-backup")],
    ["delete", remove("ObjectStorageConfig", "team-a", "old-backup")],
  ])(
    "refuses with Stale a %s whose precondition is not the copy that the server read",
    async (_, change) => {
      const cached = (
        cache.lists.ObjectStorageConfig as { metadata: { name: string } }[]
      ).find((c) => c.metadata.name === "old-backup")!;
      cached.metadata = { ...cached.metadata, resourceVersion: "40" };
      const messages = await refusedWith(change, "Stale");
      expect(messages).toEqual([
        "The ObjectStorageConfig old-backup changed since you opened it.",
      ]);
    },
  );

  it("refuses with Stale a change of a contract that the server has not read", async () => {
    await refusedWith(
      update("ObjectStorageConfig", "team-a", "new-bucket"),
      "Stale",
    );
  });

  it("refuses a delete of a contract that a claim selects", async () => {
    cache.lists.CamundaClusterClaim = [
      {
        kind: "CamundaClusterClaim",
        metadata: { namespace: "camunda-system", name: "orders" },
        spec: {
          plugins: [
            {
              section: "backup-storage",
              registration: "existing-object-storage",
              parameters: { name: "archive" },
            },
          ],
        },
      },
    ];
    await refusedWith(
      remove("ObjectStorageConfig", "camunda-system", "archive"),
      "Forbidden",
    );
  });
});

describe("previewContractChange", () => {
  it("runs the dry run of a contract created by hand", async () => {
    const change = update("ObjectStorageConfig", "team-a", "backups");
    expect(await previewContractChange(change)).toEqual({
      objects: [],
      errors: [],
    });
    expect(fakeWriter.previewed).toEqual([change]);
  });

  it("refuses under the same rules, and previews nothing", async () => {
    const preview = await previewContractChange(
      update("ObjectStorageConfig", "team-a", "orders-backup"),
    );
    expect(preview.objects).toEqual([]);
    expect(preview.errors.map((e) => e.code)).toEqual(["Forbidden"]);
    const refused = await previewContractChange(
      update("ObjectStorageConfig", "team-z", "backups"),
    );
    expect(refused.errors[0].code).toBe("Invalid");
    expect(fakeWriter.previewed).toEqual([]);
  });
});
