import { describe, expect, it } from "vitest";
import type { CamundaManagementCluster } from "@/generated/crds/camundamanagementcluster";
import type { CamundaManagementClusterClaim } from "@/generated/crds/camundamanagementclusterclaim";
import type { DatabaseServerConfig } from "@/generated/crds/databaseserverconfig";
import type { ObjectStorageConfig } from "@/generated/crds/objectstorageconfig";
import {
  LAST_APPLIED_ANNOTATION,
  REGISTRATION_LABEL,
  REQUEST_LABEL,
} from "@/lib/labels";
import { type ContractKind, contractWriter } from "@/lib/usage";
import { foreignFields } from "@/lib/write/ownership";
import type { KubeObject } from "@/lib/write/types";
import {
  type ContractDraft,
  changeCount,
  copyDraft,
  createChange,
  deleteChange,
  draftErrors,
  draftOf,
  nameErrors,
  namespaceErrors,
  newSpec,
  planeContractHolder,
  updateChange,
  withAuthType,
  withStorageType,
} from "./draft";
import type { ContractObject } from "./facts";
import { connectionFixtures } from "./fixtures";

const sources = connectionFixtures();
const backups = sources.objectStorageConfigs.find(
  (c) => c.metadata.name === "backups",
)!;
const oldBackup = sources.objectStorageConfigs.find(
  (c) => c.metadata.name === "old-backup",
)!;
const sharedEs = sources.secondaryStorageConfigs.find(
  (c) => c.metadata.name === "shared-es",
)!;
const postgres = sources.databaseServerConfigs[0];
const ordersDb = sources.databaseConfigs[0];
const planeAuth = sources.managementAuthConfigs[0];

function withMeta(
  contract: ObjectStorageConfig,
  meta: Partial<ObjectStorageConfig["metadata"]>,
): ObjectStorageConfig {
  return { ...contract, metadata: { ...contract.metadata, ...meta } };
}

function edited<T extends ContractObject>(
  contract: T,
  edit: (spec: T["spec"]) => void,
): ContractDraft {
  const draft = draftOf(contract);
  edit(draft.spec as T["spec"]);
  return draft;
}

describe("updateChange", () => {
  const live = withMeta(backups, { resourceVersion: "41", uid: "u-1" });

  it("sends the edited spec with the precondition and no force", () => {
    const draft = edited(live, (spec) => {
      spec.s3!.region = "eu-central-1";
    });
    const change = updateChange(live, draft);
    expect(change).toEqual({
      intent: "Change the object storage contract backups",
      apply: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "ObjectStorageConfig",
          metadata: { name: "backups", namespace: "team-a" },
          spec: {
            ...live.spec,
            s3: { ...live.spec.s3!, region: "eu-central-1" },
          },
        },
      ],
      preconditions: { "ObjectStorageConfig/team-a/backups": "41" },
    });
  });

  it("leaves out a field that the form cleared", () => {
    const draft = edited(live, (spec) => {
      spec.s3!.region = "";
    });
    const s3 = (
      updateChange(live, draft).apply[0].spec as ObjectStorageConfig["spec"]
    ).s3!;
    expect(Object.hasOwn(s3, "region")).toBe(false);
  });

  it("keeps the labels and annotations, without the last applied file", () => {
    const annotated = withMeta(live, {
      labels: { team: "a" },
      annotations: { [LAST_APPLIED_ANNOTATION]: "{}", owner: "ops" },
    });
    expect(
      updateChange(annotated, draftOf(annotated)).apply[0].metadata,
    ).toEqual({
      name: "backups",
      namespace: "team-a",
      labels: { team: "a" },
      annotations: { owner: "ops" },
    });
  });

  it("keeps the plugin labels of a contract kept from a registration", () => {
    expect(
      updateChange(oldBackup, draftOf(oldBackup)).apply[0].metadata.labels,
    ).toEqual({
      [REGISTRATION_LABEL]: "aws-s3",
      [REQUEST_LABEL]: "old-backup",
    });
  });

  it("puts a namespaced contract in its namespace and a Management Identity contract in none", () => {
    expect(
      updateChange(ordersDb, draftOf(ordersDb)).apply[0].metadata.namespace,
    ).toBe("team-a");
    const auth = updateChange(planeAuth, draftOf(planeAuth));
    expect(Object.hasOwn(auth.apply[0].metadata, "namespace")).toBe(false);
    expect(auth.intent).toBe(
      "Change the Management Identity contract plane-a-optimize",
    );
  });

  it("leaves the recovery request and its answer to the writers that own them", () => {
    const recovering: DatabaseServerConfig = {
      ...postgres,
      spec: {
        ...postgres.spec,
        pitr: {
          enabled: true,
          retentionPeriodDays: 7,
          recovery: "operator",
          lastRecovery: {
            completedAt: "2026-11-10T10:00:00Z",
            requestID: "r1",
            requestedBy: "team-a/restore",
            result: "Completed",
            targetTime: "2026-11-10T09:00:00Z",
          },
        },
        recovery: {
          requestID: "r1",
          requestedBy: "team-a/restore",
          targetTime: "2026-11-10T09:00:00Z",
        },
      },
    };
    const change = updateChange(
      {
        ...recovering,
        metadata: { ...recovering.metadata, resourceVersion: "9" },
      },
      draftOf(recovering),
    );
    const spec = change.apply[0].spec as DatabaseServerConfig["spec"];
    expect(Object.hasOwn(spec, "recovery")).toBe(false);
    expect(spec.pitr).toEqual({
      enabled: true,
      retentionPeriodDays: 7,
      recovery: "operator",
    });
    expect(change.keep).toEqual({
      [Object.keys(change.preconditions!)[0]]: [
        "spec.recovery",
        "spec.pitr.lastRecovery",
      ],
    });
  });

  it("keeps no field of another kind of contract", () => {
    expect(updateChange(live, draftOf(live)).keep).toBeUndefined();
  });
});

describe("deleteChange", () => {
  it("deletes with the precondition and the UID that the page read", () => {
    const live = withMeta(backups, { resourceVersion: "41", uid: "u-1" });
    expect(deleteChange(live)).toEqual({
      intent: "Delete the object storage contract backups",
      apply: [],
      delete: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "ObjectStorageConfig",
          namespace: "team-a",
          name: "backups",
        },
      ],
      preconditions: { "ObjectStorageConfig/team-a/backups": "41" },
      uids: { "ObjectStorageConfig/team-a/backups": "u-1" },
    });
  });

  it("deletes a Management Identity contract without namespace", () => {
    const live = {
      ...planeAuth,
      metadata: { ...planeAuth.metadata, resourceVersion: "7", uid: "u-2" },
    };
    expect(deleteChange(live).delete).toEqual([
      {
        apiVersion: "core.camunda.io/v1",
        kind: "ManagementAuthConfig",
        name: "plane-a-optimize",
      },
    ]);
  });
});

describe("updateChange over the empty values of another tool", () => {
  // live holds an S3 block that kubectl set, with an empty endpoint and an
  // empty base path.
  const live = withMeta(
    {
      ...backups,
      spec: {
        ...backups.spec,
        s3: { ...backups.spec.s3!, endpoint: "", basePath: "" },
      },
    },
    {
      resourceVersion: "41",
      managedFields: [
        {
          manager: "kubectl-client-side-apply",
          operation: "Update",
          fieldsV1: {
            "f:spec": {
              "f:type": {},
              "f:s3": {
                "f:bucketName": {},
                "f:region": {},
                "f:endpoint": {},
                "f:basePath": {},
              },
            },
          },
        },
      ],
    },
  );
  const rows = (draft: ContractDraft) => {
    const change = updateChange(live, draft);
    const { changed, cleared } = foreignFields(
      live as unknown as KubeObject,
      change.apply[0],
      change.keep?.["ObjectStorageConfig/team-a/backups"],
    );
    return {
      changed: changed.map((f) => f.path),
      cleared: cleared.map((f) => f.path),
    };
  };

  it("shows only the changed field", () => {
    const draft = edited(live, (spec) => {
      spec.s3!.region = "eu-central-1";
    });
    expect(rows(draft)).toEqual({
      changed: ["spec.s3.region"],
      cleared: [],
    });
  });

  it("still shows a field that the edit cleared", () => {
    const draft = edited(live, (spec) => {
      spec.s3!.region = "";
    });
    expect(rows(draft).cleared).toEqual(["spec.s3.region"]);
  });
});

describe("changeCount", () => {
  it("counts no change for an untouched draft", () => {
    expect(changeCount(backups, draftOf(backups))).toBe(0);
  });

  it("counts each changed field", () => {
    const draft = edited(backups, (spec) => {
      spec.s3!.region = "eu-central-1";
      spec.s3!.basePath = "orders";
    });
    expect(changeCount(backups, draft)).toBe(2);
  });

  it("counts a cleared field once", () => {
    const draft = edited(backups, (spec) => {
      spec.s3!.region = "";
    });
    expect(changeCount(backups, draft)).toBe(1);
  });
});

type Case = [ContractKind, ContractObject, (spec: never) => void, string];

describe("draftErrors", () => {
  const s3Credentials: ObjectStorageConfig = {
    ...backups,
    spec: {
      type: "S3",
      s3: {
        bucketName: "acme",
        region: "eu-west-1",
        auth: {
          type: "credentials",
          credentials: {
            secretRef: {
              name: "s3",
              accessKeyIdKey: "id",
              secretAccessKeyKey: "secret",
            },
          },
        },
      },
    },
  };
  const gcs: ObjectStorageConfig = {
    ...backups,
    spec: {
      type: "GCS",
      gcs: {
        bucketName: "acme",
        auth: {
          type: "credentials",
          credentials: { secretRef: { name: "gcs", key: "key.json" } },
        },
      },
    },
  };
  const azure: ObjectStorageConfig = {
    ...backups,
    spec: {
      type: "AzureBlob",
      azureBlob: {
        accountName: "acme",
        container: "backups",
        auth: {
          type: "credentials",
          credentials: { secretRef: { name: "azure", key: "key" } },
        },
      },
    },
  };
  const rdbms = {
    ...sharedEs,
    spec: { type: "rdbms" as const, rdbms: { databaseConfigRef: "orders-db" } },
  };

  it.each<ContractObject>([
    backups,
    s3Credentials,
    gcs,
    azure,
    sharedEs,
    rdbms,
    postgres,
    ordersDb,
    planeAuth,
  ])("finds no error in a complete contract: %#", (contract) => {
    expect(draftErrors(contract.kind, draftOf(contract))).toEqual([]);
  });

  it.each<Case>([
    [
      "ObjectStorageConfig",
      backups,
      (s: ObjectStorageConfig["spec"]) => void (s.s3!.bucketName = ""),
      "spec.s3.bucketName",
    ],
    [
      "ObjectStorageConfig",
      backups,
      (s: ObjectStorageConfig["spec"]) => void (s.s3!.region = ""),
      "spec.s3.region",
    ],
    [
      "ObjectStorageConfig",
      s3Credentials,
      (s: ObjectStorageConfig["spec"]) =>
        void (s.s3!.auth!.credentials = undefined),
      "spec.s3.auth.credentials.secretRef",
    ],
    [
      "ObjectStorageConfig",
      gcs,
      (s: ObjectStorageConfig["spec"]) => void (s.gcs!.bucketName = ""),
      "spec.gcs.bucketName",
    ],
    [
      "ObjectStorageConfig",
      gcs,
      (s: ObjectStorageConfig["spec"]) =>
        void (s.gcs!.auth!.credentials = undefined),
      "spec.gcs.auth.credentials.secretRef",
    ],
    [
      "ObjectStorageConfig",
      azure,
      (s: ObjectStorageConfig["spec"]) => void (s.azureBlob!.accountName = ""),
      "spec.azureBlob.accountName",
    ],
    [
      "ObjectStorageConfig",
      azure,
      (s: ObjectStorageConfig["spec"]) => void (s.azureBlob!.container = ""),
      "spec.azureBlob.container",
    ],
    [
      "ObjectStorageConfig",
      azure,
      (s: ObjectStorageConfig["spec"]) =>
        void (s.azureBlob!.auth!.credentials = undefined),
      "spec.azureBlob.auth.credentials.secretRef",
    ],
    [
      "SecondaryStorageConfig",
      sharedEs,
      (s: typeof sharedEs.spec) => void (s.elasticsearch!.endpoint = ""),
      "spec.elasticsearch.endpoint",
    ],
    [
      "SecondaryStorageConfig",
      sharedEs,
      (s: typeof sharedEs.spec) =>
        void (s.elasticsearch!.credentialsSecretRef.name = ""),
      "spec.elasticsearch.credentialsSecretRef",
    ],
    [
      "SecondaryStorageConfig",
      rdbms,
      (s: typeof rdbms.spec) => void (s.rdbms.databaseConfigRef = ""),
      "spec.rdbms.databaseConfigRef",
    ],
    [
      "DatabaseServerConfig",
      postgres,
      (s: typeof postgres.spec) => void (s.host = ""),
      "spec.host",
    ],
    [
      "DatabaseServerConfig",
      postgres,
      (s: typeof postgres.spec) => void (s.port = 0),
      "spec.port",
    ],
    [
      "DatabaseServerConfig",
      postgres,
      (s: typeof postgres.spec) => void (s.port = 65536),
      "spec.port",
    ],
    [
      "DatabaseServerConfig",
      postgres,
      (s: typeof postgres.spec) => void (s.adminCredentialsSecretRef.name = ""),
      "spec.adminCredentialsSecretRef",
    ],
    [
      "DatabaseConfig",
      ordersDb,
      (s: typeof ordersDb.spec) => void (s.databaseName = ""),
      "spec.databaseName",
    ],
    [
      "DatabaseConfig",
      ordersDb,
      (s: typeof ordersDb.spec) => void (s.serverRef = ""),
      "spec.serverRef",
    ],
    [
      "DatabaseConfig",
      ordersDb,
      (s: typeof ordersDb.spec) => void (s.credentialsSecretRef.name = ""),
      "spec.credentialsSecretRef",
    ],
    [
      "ManagementAuthConfig",
      planeAuth,
      (s: typeof planeAuth.spec) => void (s.issuerUrl = ""),
      "spec.issuerUrl",
    ],
    [
      "ManagementAuthConfig",
      planeAuth,
      (s: typeof planeAuth.spec) => void (s.authUrl = ""),
      "spec.authUrl",
    ],
    [
      "ManagementAuthConfig",
      planeAuth,
      (s: typeof planeAuth.spec) => void (s.tokenUrl = ""),
      "spec.tokenUrl",
    ],
    [
      "ManagementAuthConfig",
      planeAuth,
      (s: typeof planeAuth.spec) => void (s.jwksUrl = ""),
      "spec.jwksUrl",
    ],
    [
      "ManagementAuthConfig",
      planeAuth,
      (s: typeof planeAuth.spec) => void (s.baseUrl = ""),
      "spec.baseUrl",
    ],
    [
      "ManagementAuthConfig",
      planeAuth,
      (s: typeof planeAuth.spec) => void (s.clientId = ""),
      "spec.clientId",
    ],
    [
      "ManagementAuthConfig",
      planeAuth,
      (s: typeof planeAuth.spec) => void (s.audience = ""),
      "spec.audience",
    ],
    [
      "ManagementAuthConfig",
      planeAuth,
      (s: typeof planeAuth.spec) =>
        void (s.clientSecretRef = { namespace: "", name: "", key: "" }),
      "spec.clientSecretRef",
    ],
  ] as unknown as Case[])(
    "asks for a field that the CRD requires: %s %#, %s",
    (kind, contract, edit, path) => {
      const draft = draftOf(contract);
      edit(draft.spec as never);
      const errors = draftErrors(kind, draft);
      expect(errors.map((e) => e.path)).toEqual([path]);
      expect(errors[0].message).toMatch(/^(Enter|Pick) .+\.$/);
    },
  );

  it("asks for no field that the CRD leaves optional", () => {
    const bare: ObjectStorageConfig = {
      ...backups,
      spec: { type: "S3", s3: { bucketName: "acme", region: "eu-west-1" } },
    };
    const es = {
      ...sharedEs,
      spec: {
        type: "elasticsearch" as const,
        elasticsearch: {
          endpoint: "https://es:9200",
          credentialsSecretRef: { name: "es" },
        },
      },
    };
    const db = {
      ...ordersDb,
      spec: {
        databaseName: "orders",
        serverRef: "postgres",
        credentialsSecretRef: { name: "orders" },
      },
    };
    const auth = {
      ...planeAuth,
      spec: { ...planeAuth.spec, issuerBackendUrl: "" },
    };
    for (const contract of [bare, es, db, auth] as ContractObject[]) {
      expect(draftErrors(contract.kind, draftOf(contract))).toEqual([]);
    }
  });

  it("takes an endpoint in place of the region of S3", () => {
    const draft = edited(backups, (spec) => {
      spec.s3!.region = "";
      spec.s3!.endpoint = "https://minio.team-a:9000";
    });
    expect(draftErrors("ObjectStorageConfig", draft)).toEqual([]);
  });

  it("asks for a node count of 1 or more", () => {
    const draft = edited(sharedEs, (spec) => {
      spec.elasticsearch!.nodeCount = Number.NaN;
    });
    expect(
      draftErrors("SecondaryStorageConfig", draft).map((e) => e.path),
    ).toEqual(["spec.elasticsearch.nodeCount"]);
  });

  it("asks for the key of a CA certificate whose Secret is picked", () => {
    const draft = edited(sharedEs, (spec) => {
      spec.elasticsearch!.caSecretRef = { name: "shared-es-ca", key: "" };
    });
    expect(
      draftErrors("SecondaryStorageConfig", draft).map((e) => e.path),
    ).toEqual(["spec.elasticsearch.caSecretRef"]);
  });
});

const NAME_RULE =
  "The name has only lowercase letters, digits, and hyphens. It starts and ends with a letter or a digit, and has at most 253 characters.";

describe("nameErrors", () => {
  it.each([
    ["", "Give the contract a name."],
    ["team.a", NAME_RULE],
    ["Backups", NAME_RULE],
    ["-backups", NAME_RULE],
    ["backups-", NAME_RULE],
    ["a".repeat(254), NAME_RULE],
    ["backups", "A contract with this name exists in this namespace."],
  ])("refuses the object storage name %j", (name, message) => {
    expect(nameErrors("ObjectStorageConfig", name, ["backups"])).toEqual([
      { path: "name", message },
    ]);
  });

  it("accepts a free name of 253 characters", () => {
    expect(
      nameErrors("DatabaseConfig", "a".repeat(253), ["orders-db"]),
    ).toEqual([]);
  });

  it("refuses a Management Identity name that exists or that a management plane writes", () => {
    expect(
      nameErrors("ManagementAuthConfig", "keycloak-auth", ["keycloak-auth"]),
    ).toEqual([{ path: "name", message: "A contract with this name exists." }]);
    expect(
      nameErrors("ManagementAuthConfig", "ops-auth", [], "platform/ops"),
    ).toEqual([
      {
        path: "name",
        message:
          "The management plane platform/ops writes a contract with this name.",
      },
    ]);
    expect(nameErrors("ManagementAuthConfig", "optimize-two", [])).toEqual([]);
  });
});

describe("namespaceErrors", () => {
  it("asks for the namespace of a namespaced contract", () => {
    expect(namespaceErrors("DatabaseConfig", "")).toEqual([
      { path: "namespace", message: "Pick the namespace of the contract." },
    ]);
    expect(namespaceErrors("DatabaseConfig", "team-a")).toEqual([]);
  });

  it("asks for no namespace of a Management Identity contract", () => {
    expect(namespaceErrors("ManagementAuthConfig", "")).toEqual([]);
  });
});

describe("planeContractHolder", () => {
  it("finds the plane or plane claim that writes a contract of the name, else its own name", () => {
    const planes = [
      {
        metadata: { namespace: "platform", name: "plane-a" },
        spec: { managementAuthConfigName: "plane-a-optimize" },
      },
    ] as unknown as CamundaManagementCluster[];
    const claims = [
      {
        metadata: { namespace: "platform", name: "ops" },
        spec: { managementAuthConfigName: "ops-auth" },
      },
      { metadata: { namespace: "platform", name: "lab" }, spec: {} },
    ] as unknown as CamundaManagementClusterClaim[];

    expect(planeContractHolder("plane-a-optimize", planes, claims)).toBe(
      "platform/plane-a",
    );
    expect(planeContractHolder("ops-auth", planes, claims)).toBe(
      "platform/ops",
    );
    expect(planeContractHolder("lab", planes, claims)).toBe("platform/lab");
    expect(planeContractHolder("ops", planes, claims)).toBeUndefined();
  });
});

describe("createChange", () => {
  it("creates an object storage contract with its name and namespace only", () => {
    expect(
      createChange("ObjectStorageConfig", "team-a", "archive", {
        spec: {
          type: "S3",
          s3: {
            bucketName: "acme-archive",
            region: "eu-west-1",
            endpoint: "",
            auth: { type: "workloadIdentity" },
          },
        },
      }),
    ).toEqual({
      intent: "Create the object storage contract archive",
      apply: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "ObjectStorageConfig",
          metadata: { name: "archive", namespace: "team-a" },
          spec: {
            type: "S3",
            s3: {
              bucketName: "acme-archive",
              region: "eu-west-1",
              auth: { type: "workloadIdentity" },
            },
          },
        },
      ],
      create: ["ObjectStorageConfig/team-a/archive"],
    });
  });

  it("creates a Management Identity contract without a namespace", () => {
    const change = createChange("ManagementAuthConfig", "", "optimize-two", {
      spec: {} as ContractDraft["spec"],
    });
    expect(change.apply[0].metadata).toEqual({ name: "optimize-two" });
    expect(change.create).toEqual(["ManagementAuthConfig//optimize-two"]);
  });

  it("makes a copy of a contract of a registration a contract created by hand", () => {
    const source = sources.objectStorageConfigs.find(
      (c) => c.metadata.name === "orders-backup",
    )!;
    const draft = copyDraft("ObjectStorageConfig", source, {
      name: "orders-copy",
      namespace: "",
      spec: newSpec("ObjectStorageConfig"),
    });
    const object = createChange(
      "ObjectStorageConfig",
      draft.namespace,
      draft.name,
      draft,
    ).apply[0];

    expect(object.metadata).toEqual({
      name: "orders-copy",
      namespace: "team-a",
    });
    expect(object.spec).toEqual(source.spec);
    expect(contractWriter(object as ContractObject, [], [])).toEqual({
      by: "hand",
    });
  });

  it("leaves out the recovery request of a copied database server contract", () => {
    const spec = createChange("DatabaseServerConfig", "team-a", "pg-two", {
      spec: {
        engine: "postgres",
        host: "pg.team-a",
        port: 5432,
        adminCredentialsSecretRef: { name: "postgres-admin" },
        recovery: {
          requestID: "0c5f7e9a-1d2b-4c3d-8e4f-5a6b7c8d9e0f",
          requestedBy: "team-a/orders",
          targetTime: "2026-11-20T10:00:00Z",
        },
      } as unknown as ContractDraft["spec"],
    }).apply[0].spec;
    expect(spec).not.toHaveProperty("recovery");
    expect(spec).toEqual({
      engine: "postgres",
      host: "pg.team-a",
      port: 5432,
      adminCredentialsSecretRef: { name: "postgres-admin" },
    });
  });
});

describe("newSpec", () => {
  it.each<[ContractKind, unknown]>([
    [
      "ObjectStorageConfig",
      { type: "S3", s3: { auth: { type: "workloadIdentity" } } },
    ],
    ["SecondaryStorageConfig", { type: "elasticsearch", elasticsearch: {} }],
    ["DatabaseServerConfig", { engine: "postgres" }],
    ["DatabaseConfig", {}],
    ["ManagementAuthConfig", {}],
  ])("starts a new %s contract", (kind, spec) => {
    expect(newSpec(kind)).toEqual(spec);
  });

  it("gives a new object on each call", () => {
    expect(newSpec("ObjectStorageConfig")).not.toBe(
      newSpec("ObjectStorageConfig"),
    );
  });
});

describe("copyDraft", () => {
  it("keeps the typed name and namespace, and takes the namespace of the source only while none is picked", () => {
    expect(
      copyDraft("ObjectStorageConfig", backups, {
        name: "next",
        namespace: "team-b",
        spec: newSpec("ObjectStorageConfig"),
      }),
    ).toEqual({ name: "next", namespace: "team-b", spec: backups.spec });
    expect(
      copyDraft("ObjectStorageConfig", backups, {
        name: "next",
        namespace: "",
        spec: newSpec("ObjectStorageConfig"),
      }).namespace,
    ).toBe("team-a");
  });

  it("starts the new spec again without a source", () => {
    expect(
      copyDraft("ObjectStorageConfig", undefined, {
        name: "next",
        namespace: "team-a",
        spec: backups.spec,
      }),
    ).toEqual({
      name: "next",
      namespace: "team-a",
      spec: newSpec("ObjectStorageConfig"),
    });
  });

  it("gives a copy that a change of the draft does not write into the source", () => {
    const copy = copyDraft("ObjectStorageConfig", backups, {
      name: "next",
      namespace: "",
      spec: newSpec("ObjectStorageConfig"),
    });
    (copy.spec as ObjectStorageConfig["spec"]).s3!.bucketName = "changed";
    expect(backups.spec.s3?.bucketName).toBe("acme-backups");
  });
});

describe("withStorageType and withAuthType", () => {
  it("starts the block of the picked storage type empty", () => {
    expect(withStorageType("ObjectStorageConfig", "GCS")).toEqual({
      spec: { type: "GCS", gcs: { auth: { type: "workloadIdentity" } } },
    });
    expect(withStorageType("ObjectStorageConfig", "AzureBlob")).toEqual({
      spec: {
        type: "AzureBlob",
        azureBlob: { auth: { type: "workloadIdentity" } },
      },
    });
    expect(withStorageType("SecondaryStorageConfig", "rdbms")).toEqual({
      spec: { type: "rdbms", rdbms: {} },
    });
  });

  it("keeps the fields of the block and drops the fields of the old auth", () => {
    const draft = {
      spec: {
        type: "S3",
        s3: {
          bucketName: "acme",
          auth: {
            type: "workloadIdentity",
            workloadIdentity: { roleArn: "arn:aws:iam::1:role/x" },
          },
        },
      },
    } as ContractDraft;
    expect(withAuthType(draft, "credentials")).toEqual({
      spec: {
        type: "S3",
        s3: { bucketName: "acme", auth: { type: "credentials" } },
      },
    });
  });
});

describe("changeCount of a new contract", () => {
  it("counts nothing on the new spec and each field set after", () => {
    const base = { spec: newSpec("ObjectStorageConfig") };
    expect(changeCount(base, { spec: newSpec("ObjectStorageConfig") })).toBe(0);
    expect(
      changeCount(base, {
        spec: {
          type: "S3",
          s3: {
            bucketName: "acme",
            region: "eu-west-1",
            auth: { type: "workloadIdentity" },
          },
        },
      }),
    ).toBe(2);
  });
});
