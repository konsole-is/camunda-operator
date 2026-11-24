import { beforeEach, describe, expect, it, vi } from "vitest";
import { CONNECTION_SOURCE_KINDS } from "@/components/setup/connections/source-kinds";
import { createChange } from "@/components/setup/connections/draft";
import { connectionFixtures } from "@/components/setup/connections/fixtures";
import type { ConnectionSources } from "@/components/setup/connections/sources";
import { REGISTRATION_LABEL } from "@/lib/labels";
import type { ContractKind } from "@/lib/usage";
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
// resourceVersion of the changes below. spare is a second unused contract
// in camunda-system.
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
  sources.objectStorageConfigs.push({
    ...sources.objectStorageConfigs[0],
    metadata: { namespace: "camunda-system", name: "spare" },
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

function create(kind: ContractKind, namespace: string, name: string): Change {
  return createChange(kind, namespace, name, { spec: {} as never });
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

  it("refuses a change that writes more than one contract, and writes nothing", async () => {
    const change = update("SecondaryStorageConfig", "team-a", "shared-es");
    const removed = remove("DatabaseConfig", "team-a", "orders-db");
    change.delete = removed.delete;
    change.preconditions = {
      ...change.preconditions,
      ...removed.preconditions,
    };
    await refusedWith(change, "Invalid");
  });

  it("refuses a delete without precondition", async () => {
    const change = remove("DatabaseConfig", "team-a", "orders-db");
    delete change.preconditions;
    await refusedWith(change, "Invalid");
  });

  it("writes a create of a new contract by hand", async () => {
    const change = create("ObjectStorageConfig", "team-a", "new-bucket");
    expect(await applyContractChange(change)).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([change]);
  });

  it("writes a create of a Management Identity contract without namespace", async () => {
    const change = create("ManagementAuthConfig", "", "optimize-two");
    expect((await applyContractChange(change)).ok).toBe(true);
    expect(fakeWriter.applied).toEqual([change]);
  });

  it("writes a create of a name that another namespace holds", async () => {
    expect(
      (await applyContractChange(create("ObjectStorageConfig", "team-a", "archive")))
        .ok,
    ).toBe(true);
  });

  it.each([
    ["ObjectStorageConfig", "team-a", "backups"],
    ["ObjectStorageConfig", "team-a", "orders-backup"],
    ["ManagementAuthConfig", "", "plane-a-optimize"],
  ] as const)(
    "refuses with Stale a create of %s %s/%s that the server lists",
    async (kind, namespace, name) => {
      expect(
        await refusedWith(create(kind, namespace, name), "Stale"),
      ).toEqual([`The ${kind} ${name} exists already. Pick another name.`]);
    },
  );

  it("refuses a create with labels, and writes nothing", async () => {
    const change = create("ObjectStorageConfig", "team-a", "new-bucket");
    change.apply[0].metadata.labels = { [REGISTRATION_LABEL]: "aws-s3" };
    expect(await refusedWith(change, "Invalid")).toEqual([
      expect.stringContaining(
        "A create must write one contract with only its name and namespace.",
      ),
    ]);
  });

  it("refuses a create with annotations, and writes nothing", async () => {
    const change = create("ObjectStorageConfig", "team-a", "new-bucket");
    change.apply[0].metadata.annotations = { note: "x" };
    await refusedWith(change, "Invalid");
  });

  it("refuses a create whose key is not its object, and writes nothing", async () => {
    const change = create("ObjectStorageConfig", "team-a", "new-bucket");
    change.create = ["ObjectStorageConfig/team-a/backups"];
    await refusedWith(change, "Invalid");
  });

  it("refuses a create into a namespace that the cluster does not have", async () => {
    await refusedWith(
      create("ObjectStorageConfig", "team-z", "new-bucket"),
      "Invalid",
    );
  });

  it("refuses a create while the contracts of the kind cannot be read", async () => {
    cache.failing = ["DatabaseConfig"];
    expect(
      await refusedWith(create("DatabaseConfig", "team-a", "orders-two"), "Forbidden"),
    ).toEqual([
      "The fleet manager cannot read DatabaseConfig, so DatabaseConfig team-a/orders-two is not created.",
    ]);
  });

  it("refuses a Management Identity contract that a plane claim writes", async () => {
    cache.lists.CamundaManagementClusterClaim = [
      {
        metadata: { namespace: "platform", name: "ops" },
        spec: { managementAuthConfigName: "ops-auth" },
      },
    ];
    expect(
      await refusedWith(create("ManagementAuthConfig", "", "ops-auth"), "Forbidden"),
    ).toEqual([
      "The management plane platform/ops writes ManagementAuthConfig ops-auth. Pick another name.",
    ]);
  });

  it("refuses a Management Identity contract while the plane claims cannot be read", async () => {
    cache.failing = ["CamundaManagementClusterClaim"];
    expect(
      await refusedWith(
        create("ManagementAuthConfig", "", "optimize-two"),
        "Forbidden",
      ),
    ).toEqual([
      "The fleet manager cannot read CamundaManagementClusterClaim, so ManagementAuthConfig optimize-two is not created.",
    ]);
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
        cache.lists.ObjectStorageConfig as {
          metadata: { name: string; resourceVersion?: string };
        }[]
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

  describe("a custom registration whose request output names the contract", () => {
    beforeEach(() => {
      cache.lists.PluginSection = [
        {
          kind: "PluginSection",
          metadata: { name: "backup-storage" },
          spec: { selection: "One", slot: "backupStorage" },
        },
      ];
      cache.lists.PluginRequest = [
        {
          kind: "PluginRequest",
          metadata: { namespace: "camunda-system", name: "orders-backups" },
          spec: {
            section: "backup-storage",
            registration: "acme-s3",
            parameters: {},
          },
          status: {
            outputs: { objectStorageConfig: "archive" },
            conditions: [
              { type: "OutputsReady", status: "True", observedGeneration: 0 },
            ],
          },
        },
      ];
    });

    it("refuses a delete of the contract and names the request", async () => {
      const messages = await refusedWith(
        remove("ObjectStorageConfig", "camunda-system", "archive"),
        "Forbidden",
      );
      expect(messages).toEqual([
        "Delete is possible when no resource uses ObjectStorageConfig camunda-system/archive. Used by PluginRequest camunda-system/orders-backups.",
      ]);
    });

    it.each([
      [
        "has no outputs for its spec yet",
        { status: {} },
        "PluginRequest camunda-system/orders-backups has not given its outputs for its current spec yet.",
      ],
      [
        "names a section that does not exist",
        { section: "gone" },
        "The section gone of PluginRequest camunda-system/orders-backups does not exist.",
      ],
    ])(
      "refuses a delete of an unused contract while a request %s, and names the request",
      async (_, patch, cause) => {
        const [request] = cache.lists.PluginRequest as {
          spec: { section: string };
          status: unknown;
        }[];
        if ("status" in patch) request.status = patch.status;
        else request.spec.section = patch.section;
        const messages = await refusedWith(
          remove("ObjectStorageConfig", "camunda-system", "spare"),
          "Forbidden",
        );
        expect(messages).toEqual([
          `Delete is possible when the fleet manager knows the users. ${cause}`,
        ]);
      },
    );

    it("writes a change of a contract while a request has no outputs for its spec yet", async () => {
      (cache.lists.PluginRequest as { status: unknown }[])[0].status = {};
      const change = update("ObjectStorageConfig", "camunda-system", "spare");
      expect((await applyContractChange(change)).ok).toBe(true);
      expect(fakeWriter.applied).toEqual([change]);
    });

    it("deletes while a request that is being deleted has no output", async () => {
      const [request] = cache.lists.PluginRequest as {
        metadata: Record<string, string>;
        status: unknown;
      }[];
      request.metadata.deletionTimestamp = "2026-11-18T10:00:00Z";
      request.status = {};
      const change = remove("ObjectStorageConfig", "camunda-system", "spare");
      expect((await applyContractChange(change)).ok).toBe(true);
    });

    it.each(["PluginSection", "PluginRegistration"])(
      "refuses a delete while %s cannot be read, and names it",
      async (kind) => {
        cache.failing = [kind];
        const messages = await refusedWith(
          remove("ObjectStorageConfig", "camunda-system", "archive"),
          "Forbidden",
        );
        expect(messages).toEqual([
          `Delete is possible when the fleet manager can read ${kind}.`,
        ]);
      },
    );

    it("deletes a contract that no output names", async () => {
      const change = remove("ObjectStorageConfig", "camunda-system", "spare");
      expect((await applyContractChange(change)).ok).toBe(true);
      expect(fakeWriter.applied).toEqual([change]);
    });
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

  it("previews a create of a new contract", async () => {
    const change = create("DatabaseConfig", "team-a", "orders-two");
    await previewContractChange(change);
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
