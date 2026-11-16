import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import {
  coreSections,
  fixtureClaim,
  fixtureComponents,
  fixtureCondition,
  fixtureContract,
  fixtureGrant,
  fixtureRegistrations,
  fixtureRequest,
  fixtureRequests,
  fixtureSelections,
} from "./request-fixtures";
import {
  builtinStorageOf,
  readyOf,
  requestDetail,
  storageFacts,
} from "./requests";

type Over = {
  deletionPolicy?: "Delete" | "Orphan";
  requests?: Resource<"PluginRequest">[];
  grants?: Resource<"Grant">[];
  contracts?: Parameters<typeof requestDetail>[0]["contracts"];
  claim?: Resource<"CamundaClusterClaim">;
};

// detail reads the request of section of the fixture claim, with every
// other request of the claim beside it.
function detail(section: string, over: Over = {}) {
  const claim =
    over.claim ?? fixtureClaim(fixtureSelections, over.deletionPolicy);
  const components = fixtureComponents(claim, over.requests ?? fixtureRequests);
  const component = components.find((c) => c.section === section)!;
  const found = requestDetail({
    component,
    claim,
    cluster: "orders",
    sections: coreSections,
    registrations: fixtureRegistrations,
    components,
    grants: over.grants ?? [],
    contracts: over.contracts ?? {},
  });
  if (!found) throw new Error(`no detail for ${section}`);
  return found;
}

// withRequest returns the fixture requests with the request of its section
// replaced by request.
function withRequest(request: Resource<"PluginRequest">) {
  return fixtureRequests.map((r) =>
    r.spec.section === request.spec.section ? request : r,
  );
}

const rejected = [
  fixtureCondition("Applied", "False", "Rejected", "The policy is refused"),
  fixtureCondition("MatchesTemplate", "True", "Healthy"),
  fixtureCondition("Ready", "False", "Rejected", "The policy is refused"),
];

describe("requestDetail", () => {
  it("keeps the summary items in order with their severity", () => {
    expect(detail("encryption-key").summary).toEqual([
      { label: "Key", value: "alias/orders", severity: "Info" },
      { label: "State", value: "Enabled", severity: "Info" },
      { label: "Rotation", value: "Off", severity: "Warning" },
    ]);
  });

  it("gives the time of the summary", () => {
    expect(detail("encryption-key").summaryTime).toBe("2026-11-16T11:40:00Z");
  });

  it("shows a summary item of the wrong shape as an error of its row", () => {
    const request = fixtureRequest("encryption-key", {
      summary: [
        { label: "Key", value: "alias/orders" },
        { label: "State", value: 3 },
      ] as never,
    });
    expect(
      detail("encryption-key", { requests: withRequest(request) }).summary,
    ).toEqual([
      { label: "Key", value: "alias/orders", severity: "Info" },
      { error: "Summary item 2 is not a valid summary item." },
    ]);
  });

  it("names the field that each output fills", () => {
    expect(detail("volume-encryption").outputs).toEqual([
      {
        key: "storageClassName",
        value: "orders-encrypted",
        fills: ["CamundaCluster spec.zeebe.storageClassName"],
      },
    ]);
  });

  it("gives an output that fills no field no fills, in the order of the keys", () => {
    expect(detail("connectors-permissions").outputs).toEqual([
      { key: "attachedPolicies", value: "orders-connectors-s3", fills: [] },
      {
        key: "principal",
        value: "arn:aws:iam::1:role/orders-connectors",
        fills: [],
      },
      {
        key: "serviceAccountAnnotations",
        value:
          '{"eks.amazonaws.com/role-arn":"arn:aws:iam::1:role/orders-connectors"}',
        fills: ["CamundaCluster spec.connectors.serviceAccount.annotations"],
      },
    ]);
  });

  it("names the section and output of each input, and its value once it is resolved", () => {
    expect(detail("volume-encryption").inputs).toEqual([
      {
        name: "keyRef",
        section: "encryption-key",
        sectionLabel: "Encryption key",
        output: "keyRef",
        value: "arn:aws:kms:eu-west-1:1:key/1234",
      },
    ]);
    const waiting = fixtureRequest("volume-encryption", {});
    expect(
      detail("volume-encryption", { requests: withRequest(waiting) }).inputs,
    ).toEqual([
      {
        name: "keyRef",
        section: "encryption-key",
        sectionLabel: "Encryption key",
        output: "keyRef",
      },
    ]);
  });

  it("gives the links of the request", () => {
    expect(detail("access").links).toEqual([
      { name: "Operate", url: "https://orders.example.com/operate" },
    ]);
  });

  it("gives each resource its fate under the policy of the claim", () => {
    const keep = detail("backup-storage", { deletionPolicy: "Orphan" });
    expect(keep.policy).toBe("Orphan");
    expect(keep.resources.map((r) => "fate" in r && r.fate)).toEqual([
      "Kept",
      "Deleted",
    ]);
    const remove = detail("backup-storage", { deletionPolicy: "Delete" });
    expect(remove.policy).toBe("Delete");
    expect(remove.resources.map((r) => "fate" in r && r.fate)).toEqual([
      "Deleted",
      "Deleted",
    ]);
  });

  it("gives the resources of a request that is being deleted the fate under its own policy", () => {
    const going = fixtureRequest("backup-storage", {
      resources: [{ kind: "Bucket", name: "orders-backups", onOrphan: "Kept" }],
    });
    going.metadata!.deletionTimestamp = "2026-11-16T11:59:00Z";
    going.spec.deletionPolicy = "Orphan";
    const found = detail("backup-storage", {
      deletionPolicy: "Delete",
      requests: withRequest(going),
    });
    expect(found.policy).toBe("Orphan");
    expect(found.resources).toEqual([
      {
        kind: "Bucket",
        name: "orders-backups",
        onOrphan: "Kept",
        fate: "Kept",
      },
    ]);
  });

  it("gives no fate to the resources of a request that the claim does not control", () => {
    const other = fixtureRequest("backup-storage", {
      resources: [{ kind: "Bucket", name: "orders-backups", onOrphan: "Kept" }],
    });
    other.metadata!.ownerReferences = [
      {
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "CamundaClusterClaim",
        name: "orders",
        uid: "uid-of-an-older-claim",
        controller: true,
      },
    ];
    const found = detail("backup-storage", { requests: withRequest(other) });
    expect(found.controlled).toBe(false);
    expect(found.resources).toEqual([
      { kind: "Bucket", name: "orders-backups", onOrphan: "Kept" },
    ]);
  });

  it("shows a resource row of the wrong shape as an error of its row", () => {
    const request = fixtureRequest("backup-storage", {
      resources: [{ kind: "Bucket", onOrphan: "Kept" }] as never,
    });
    expect(
      detail("backup-storage", { requests: withRequest(request) }).resources,
    ).toEqual([{ error: "Resource 1 is not a valid resource." }]);
  });

  it("shows a status list that is not a list as one error", () => {
    const request = fixtureRequest("backup-storage", {
      resources: "orders-backups" as never,
      outputs: ["orders-backups"] as never,
    });
    const found = detail("backup-storage", { requests: withRequest(request) });
    expect(found.resources).toEqual([
      { error: "The resources are not a list." },
    ]);
    expect(found.outputs).toEqual([
      { error: "The outputs are not an object." },
    ]);
  });

  it("lists the grants that the request owns, with the statement as JSON", () => {
    const owned = fixtureGrant("orders-read", "backup-storage", [
      fixtureCondition("Applied", "True", "Healthy"),
      fixtureCondition("MatchesTemplate", "True", "Healthy"),
      fixtureCondition("Ready", "True", "Healthy"),
    ]);
    const ofAnother = fixtureGrant("orders-key", "encryption-key");
    const [grant, ...rest] = detail("backup-storage", {
      grants: [ofAnother, owned],
    }).grants;
    expect(rest).toEqual([]);
    expect(grant).toMatchObject({
      name: "orders-read",
      template: "read-objects",
      state: "ready",
      ready: { type: "Ready", status: "True", reason: "Healthy" },
    });
    expect(grant.request).toBeUndefined();
    expect(JSON.parse(grant.statement!)).toEqual({
      Effect: "Allow",
      Action: ["s3:GetObject"],
      Resource: "arn:aws:s3:::orders-backups/*",
    });
    expect(grant.statement).toContain("\n  ");
    expect(grant.conditions.map((c) => c.type)).toEqual([
      "Applied",
      "MatchesTemplate",
      "Ready",
    ]);
  });

  it("lists every grant of the cluster for the cloud identity, by name, with its request", () => {
    const backups = fixtureGrant("orders-read", "backup-storage");
    const key = fixtureGrant("orders-key", "encryption-key");
    const elsewhere = fixtureGrant("payments-read", "backup-storage");
    elsewhere.spec.cluster = { name: "payments" };
    expect(
      detail("cloud-identity", {
        grants: [backups, elsewhere, key],
      }).grants.map((g) => [g.name, g.request]),
    ).toEqual([
      ["orders-key", "orders-encryption-key"],
      ["orders-read", "orders-backup-storage"],
    ]);
  });

  it("shows a grant without a status with its statement and no conditions", () => {
    const [grant] = detail("backup-storage", {
      grants: [fixtureGrant("orders-read", "backup-storage")],
    }).grants;
    expect(grant.conditions).toEqual([]);
    expect(grant.ready).toBeUndefined();
    expect(grant.state).toBe("waiting");
    expect(grant.statement).toBeDefined();
  });

  it("keeps the Ready of a grant whose template changed as the identity extension wrote it, applied with the old template", () => {
    const changed = fixtureGrant("orders-read", "backup-storage", [
      fixtureCondition("Applied", "True", "Healthy"),
      fixtureCondition(
        "MatchesTemplate",
        "False",
        "TemplateChanged",
        "The fragment no longer matches its template",
      ),
      fixtureCondition(
        "Ready",
        "False",
        "TemplateChanged",
        "The fragment no longer matches its template",
      ),
    ]);
    const [grant] = detail("backup-storage", { grants: [changed] }).grants;
    expect(grant.ready).toMatchObject({
      status: "False",
      reason: "TemplateChanged",
    });
    expect(grant.state).toBe("oldTemplate");
  });

  it("marks a grant whose template its registration no longer has", () => {
    const kept = fixtureGrant("orders-read", "backup-storage");
    const gone = fixtureGrant("orders-write", "backup-storage");
    gone.spec.template = "write-objects";
    const unknown = fixtureGrant("orders-list", "backup-storage");
    unknown.metadata!.labels!["plugins.camunda.io/registration"] = "aws-s3-old";
    expect(
      detail("backup-storage", { grants: [kept, gone, unknown] }).grants.map(
        (g) => [g.name, g.templateGone],
      ),
    ).toEqual([
      ["orders-list", false],
      ["orders-read", false],
      ["orders-write", true],
    ]);
  });

  it("calls a grant refused while it is not Ready for any other reason", () => {
    const [grant] = detail("backup-storage", {
      grants: [fixtureGrant("orders-read", "backup-storage", rejected)],
    }).grants;
    expect(grant.state).toBe("refused");
    expect(grant.ready).toMatchObject({ status: "False", reason: "Rejected" });
  });

  it("shows a grant whose values have the wrong type with its errors", () => {
    const broken = fixtureGrant("orders-read", "backup-storage");
    broken.spec.fragment = "s3:GetObject" as never;
    broken.status = {
      conditions: [{ type: "Ready", status: "Maybe" }] as never,
    };
    const [grant] = detail("backup-storage", { grants: [broken] }).grants;
    expect(grant.statement).toBeUndefined();
    expect(grant.errors).toEqual([
      "The statement is not a JSON object.",
      "Condition 1 is not a valid condition.",
    ]);
    expect(grant.state).toBe("waiting");
  });

  it("shows an existing contract as connected, with its Ready, and no resources", () => {
    const found = detail("secondary-storage", {
      contracts: {
        SecondaryStorageConfig: [fixtureContract("orders-storage")],
      },
    });
    expect(found.connected).toEqual([
      {
        kind: "SecondaryStorageConfig",
        name: "orders-storage",
        found: true,
        ready: expect.objectContaining({ type: "Ready", status: "True" }),
      },
    ]);
    expect(found.resources).toEqual([]);
  });

  it("says that a connected contract is not in the namespace", () => {
    const elsewhere = fixtureContract("orders-storage");
    elsewhere.metadata!.namespace = "team-b";
    expect(
      detail("secondary-storage", {
        contracts: { SecondaryStorageConfig: [elsewhere] },
      }).connected,
    ).toEqual([
      { kind: "SecondaryStorageConfig", name: "orders-storage", found: false },
    ]);
  });

  it("connects each database config that an existing-database-configs selection names", () => {
    const claim = fixtureClaim([
      {
        section: "secondary-storage",
        registration: "existing-database-configs",
        parameters: { keycloak: "kc-db", identity: "id-db", webModeler: 4 },
      },
    ]);
    const components = fixtureComponents(claim, [
      fixtureRequest("secondary-storage"),
    ]);
    const found = requestDetail({
      component: components[0],
      claim,
      cluster: "orders",
      sections: coreSections,
      registrations: fixtureRegistrations,
      components,
      grants: [],
      contracts: {},
    });
    expect(found?.connected?.map((c) => [c.kind, c.name])).toEqual([
      ["DatabaseConfig", "kc-db"],
      ["DatabaseConfig", "id-db"],
    ]);
  });

  it("connects nothing for an option that makes its resources", () => {
    expect(detail("backup-storage").connected).toBeUndefined();
  });

  it("gives the principal and the ServiceAccount of the Connectors permissions", () => {
    expect(detail("connectors-permissions").connectors).toEqual({
      principal: "arn:aws:iam::1:role/orders-connectors",
      serviceAccount: "orders-connectors",
    });
    expect(detail("cloud-identity").connectors).toBeUndefined();
  });

  it("lists the sections that take an output of this section as an input", () => {
    expect(detail("encryption-key").usedBy).toEqual([
      "Volume encryption",
      "Backup storage",
    ]);
    expect(detail("backup-storage").usedBy).toEqual([]);
  });

  it("gives no detail for a section without a request", () => {
    const claim = fixtureClaim();
    const components = fixtureComponents(claim, []);
    expect(
      requestDetail({
        component: components[0],
        claim,
        cluster: "orders",
        sections: coreSections,
        registrations: fixtureRegistrations,
        components,
        grants: [],
        contracts: {},
      }),
    ).toBeUndefined();
  });
});

const esRequest = fixtureRequest("secondary-storage", {
  resources: [
    { kind: "Secret", name: "orders-es-credentials", onOrphan: "Deleted" },
    { kind: "ElasticsearchCluster", name: "orders-es", onOrphan: "Deleted" },
  ],
});

function elasticsearch(name: string, namespace = "team-a") {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "ElasticsearchCluster",
    metadata: { namespace, name },
    spec: {},
  } as unknown as Resource<"ElasticsearchCluster">;
}

function databaseServer(name: string) {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "DatabaseServer",
    metadata: { namespace: "team-a", name },
    spec: {},
  } as unknown as Resource<"DatabaseServer">;
}

describe("builtinStorageOf", () => {
  it("finds the ElasticsearchCluster by the resource row of the request", () => {
    const es = elasticsearch("orders-es");
    expect(
      builtinStorageOf(esRequest, [elasticsearch("other-es"), es], []),
    ).toEqual({ kind: "ElasticsearchCluster", name: "orders-es", object: es });
  });

  it("finds the DatabaseServer by the resource row of the request", () => {
    const db = databaseServer("orders-db");
    const request = fixtureRequest("secondary-storage", {
      resources: [
        { kind: "DatabaseServer", name: "orders-db", onOrphan: "Deleted" },
      ],
    });
    expect(builtinStorageOf(request, [], [db])).toEqual({
      kind: "DatabaseServer",
      name: "orders-db",
      object: db,
    });
  });

  it("names the object that the row names when it is not in the namespace", () => {
    expect(
      builtinStorageOf(esRequest, [elasticsearch("orders-es", "team-b")], []),
    ).toEqual({ kind: "ElasticsearchCluster", name: "orders-es" });
  });

  it("gives nothing for a request without a storage row", () => {
    expect(
      builtinStorageOf(fixtureRequest("backup-storage", {}), [], []),
    ).toBeUndefined();
  });
});

describe("readyOf", () => {
  it("gives the Ready condition, and nothing for conditions of the wrong type", () => {
    const ready = fixtureCondition("Ready", "False", "Unreachable");
    expect(readyOf([fixtureCondition("Other", "True", "Healthy"), ready])).toBe(
      ready,
    );
    expect(readyOf("Ready")).toBeUndefined();
  });
});

describe("storageFacts", () => {
  it("gives the version, snapshot repository, volumes, and conditions of an ElasticsearchCluster", () => {
    const object = elasticsearch("orders-es");
    object.status = {
      version: "8.19.2",
      snapshotRepository: "camunda",
      volumes: [{ name: "data-orders-es-0", capacity: "32Gi" }],
      conditions: [fixtureCondition("Ready", "True", "Healthy")],
    };
    expect(
      storageFacts({ kind: "ElasticsearchCluster", name: "orders-es", object }),
    ).toEqual({
      version: "8.19.2",
      snapshotRepository: "camunda",
      volumes: [{ name: "data-orders-es-0", capacity: "32Gi" }],
      conditions: [fixtureCondition("Ready", "True", "Healthy")],
      errors: [],
    });
  });

  it("gives the version, archive, recovery, and volumes of a DatabaseServer", () => {
    const object = databaseServer("orders-db");
    object.status = {
      version: "17.5",
      archive: {
        reachableFrom: "2026-11-10T09:58:00Z",
        boundary: { at: "2026-11-15T08:00:00Z", location: "s3://orders/db" },
      },
      recovery: {
        requestID: "3f2b1c4d",
        targetTime: "2026-11-15T07:30:00Z",
        startedAt: "2026-11-15T07:31:00Z",
        contract: "orders-db",
        requestedBy: "orders",
      },
      volumes: [{ name: "orders-db-1", capacity: 10737418240 }],
    };
    expect(
      storageFacts({ kind: "DatabaseServer", name: "orders-db", object }),
    ).toEqual({
      version: "17.5",
      archive: {
        reachableFrom: "2026-11-10T09:58:00Z",
        boundary: "2026-11-15T08:00:00Z, s3://orders/db",
      },
      recovery: {
        requestID: "3f2b1c4d",
        targetTime: "2026-11-15T07:30:00Z",
        startedAt: "2026-11-15T07:31:00Z",
      },
      volumes: [{ name: "orders-db-1", capacity: "10737418240" }],
      conditions: [],
      errors: [],
    });
  });

  it("leaves out each field that the status does not hold", () => {
    expect(
      storageFacts({
        kind: "DatabaseServer",
        name: "orders-db",
        object: databaseServer("orders-db"),
      }),
    ).toEqual({ volumes: [], conditions: [], errors: [] });
  });

  it("names each value of the wrong type", () => {
    const object = elasticsearch("orders-es");
    object.status = {
      version: 8,
      volumes: [{ name: "data-0" }],
      conditions: "Ready",
    } as never;
    expect(
      storageFacts({ kind: "ElasticsearchCluster", name: "orders-es", object }),
    ).toEqual({
      volumes: [{ error: "Volume 1 is not a valid volume." }],
      conditions: [],
      errors: ["The version is not a text.", "The conditions are not a list."],
    });
  });
});
