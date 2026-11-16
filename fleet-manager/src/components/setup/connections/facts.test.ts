import { describe, expect, it } from "vitest";
import type { DatabaseConfig } from "@/generated/crds/databaseconfig";
import type { DatabaseServerConfig } from "@/generated/crds/databaseserverconfig";
import type { ManagementAuthConfig } from "@/generated/crds/managementauthconfig";
import type { ObjectStorageConfig } from "@/generated/crds/objectstorageconfig";
import type { SecondaryStorageConfig } from "@/generated/crds/secondarystorageconfig";
import {
  contractFacts,
  type Fact,
  KEY_FACT_LABELS,
  keyFacts,
  NODE_COUNT_HINT,
} from "./facts";

const meta = { namespace: "team-a", name: "c" };

function objectStorage(spec: ObjectStorageConfig["spec"]): ObjectStorageConfig {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "ObjectStorageConfig",
    metadata: meta,
    spec,
  };
}

function secondary(
  spec: SecondaryStorageConfig["spec"],
): SecondaryStorageConfig {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "SecondaryStorageConfig",
    metadata: meta,
    spec,
  };
}

const server: DatabaseServerConfig = {
  apiVersion: "core.camunda.io/v1",
  kind: "DatabaseServerConfig",
  metadata: meta,
  spec: {
    engine: "postgres",
    host: "pg.example.com",
    port: 5432,
    adminCredentialsSecretRef: { name: "pg-admin" },
  },
};

const database: DatabaseConfig = {
  apiVersion: "core.camunda.io/v1",
  kind: "DatabaseConfig",
  metadata: meta,
  spec: {
    databaseName: "orders",
    serverRef: "postgres",
    credentialsSecretRef: { name: "orders-db", usernameKey: "user" },
    backupCredentialsSecretRef: { name: "orders-backup", passwordKey: "pw" },
  },
};

const auth: ManagementAuthConfig = {
  apiVersion: "core.camunda.io/v1",
  kind: "ManagementAuthConfig",
  metadata: { name: "plane-a-optimize" },
  spec: {
    issuerUrl: "https://id.example.com/realms/camunda",
    authUrl: "https://id.example.com/auth",
    tokenUrl: "https://id.example.com/token",
    jwksUrl: "https://id.example.com/certs",
    baseUrl: "https://id.example.com",
    clientId: "optimize",
    audience: "optimize-api",
    clientSecretRef: {
      namespace: "platform",
      name: "optimize-client",
      key: "secret",
    },
  },
};

const fact = (facts: Fact[], label: string) =>
  facts.find((f) => f.label === label);

describe("contractFacts", () => {
  it("shows the CA certificate as a name and a key", () => {
    const facts = contractFacts(
      secondary({
        type: "elasticsearch",
        elasticsearch: {
          endpoint: "https://es:9200",
          credentialsSecretRef: { name: "es-user" },
          caSecretRef: { name: "es-ca", key: "ca.crt" },
        },
      }),
    );
    expect(fact(facts, "CA certificate")).toEqual({
      label: "CA certificate",
      secret: { name: "es-ca", keys: [["Key", "ca.crt"]] },
    });
  });

  it("shows admin credentials as a name, a username key, and a password key, with the defaults", () => {
    expect(fact(contractFacts(server), "Admin credentials")).toEqual({
      label: "Admin credentials",
      secret: {
        name: "pg-admin",
        keys: [
          ["Username key", "username"],
          ["Password key", "password"],
        ],
      },
    });
  });

  it("shows the retention of point-in-time recovery and who rolls the server back", () => {
    const facts = contractFacts({
      ...server,
      spec: {
        ...server.spec,
        pitr: { enabled: true, retentionPeriodDays: 7, recovery: "operator" },
      },
    });
    expect(fact(facts, "Point-in-time recovery")?.value).toBe(
      "7 days of retention",
    );
    expect(fact(facts, "Rollback")?.value).toBe("By the writer");
  });

  it("shows a manual rollback when the contract leaves recovery unset", () => {
    const facts = contractFacts({
      ...server,
      spec: { ...server.spec, pitr: { enabled: true, retentionPeriodDays: 1 } },
    });
    expect(fact(facts, "Rollback")?.value).toBe("Manual");
  });

  it("shows point-in-time recovery as off when the contract turns it off", () => {
    const facts = contractFacts({
      ...server,
      spec: { ...server.spec, pitr: { enabled: false } },
    });
    expect(fact(facts, "Point-in-time recovery")?.value).toBe("Off");
    expect(fact(facts, "Rollback")).toBeUndefined();
  });

  it("gives no point-in-time recovery fact when the contract has no pitr", () => {
    expect(fact(contractFacts(server), "Point-in-time recovery")).toBe(
      undefined,
    );
  });

  it("shows the credentials and the backup credentials of a database, with the defaults", () => {
    const facts = contractFacts(database);
    expect(fact(facts, "Credentials")?.secret).toEqual({
      name: "orders-db",
      keys: [
        ["Username key", "user"],
        ["Password key", "password"],
      ],
    });
    expect(fact(facts, "Backup credentials")?.secret).toEqual({
      name: "orders-backup",
      keys: [
        ["Username key", "username"],
        ["Password key", "pw"],
      ],
    });
  });

  it("shows the S3 pair", () => {
    const facts = contractFacts(
      objectStorage({
        type: "S3",
        s3: {
          bucketName: "acme",
          auth: {
            type: "credentials",
            credentials: {
              secretRef: {
                name: "s3-keys",
                accessKeyIdKey: "id",
                secretAccessKeyKey: "secret",
              },
            },
          },
        },
      }),
    );
    expect(fact(facts, "Credentials")?.secret).toEqual({
      name: "s3-keys",
      keys: [
        ["Access key ID key", "id"],
        ["Secret access key key", "secret"],
      ],
    });
  });

  it("shows the role ARN of S3 workload identity", () => {
    const facts = contractFacts(
      objectStorage({
        type: "S3",
        s3: {
          bucketName: "acme",
          region: "eu-west-1",
          auth: {
            type: "workloadIdentity",
            workloadIdentity: { roleArn: "arn:aws:iam::1:role/r" },
          },
        },
      }),
    );
    expect(facts).toEqual([
      { label: "Type", value: "S3" },
      { label: "Bucket", value: "acme" },
      { label: "Region", value: "eu-west-1" },
      { label: "Auth", value: "Workload identity" },
      { label: "Role ARN", value: "arn:aws:iam::1:role/r" },
    ]);
  });

  it("shows the GCS and Azure key", () => {
    const gcs = contractFacts(
      objectStorage({
        type: "GCS",
        gcs: {
          bucketName: "acme",
          auth: {
            type: "credentials",
            credentials: { secretRef: { name: "gcs-key", key: "key.json" } },
          },
        },
      }),
    );
    const azure = contractFacts(
      objectStorage({
        type: "AzureBlob",
        azureBlob: {
          accountName: "acme",
          container: "backups",
          auth: {
            type: "credentials",
            credentials: { secretRef: { name: "azure-key", key: "key" } },
          },
        },
      }),
    );
    expect(fact(gcs, "Credentials")?.secret).toEqual({
      name: "gcs-key",
      keys: [["Key", "key.json"]],
    });
    expect(fact(azure, "Credentials")?.secret).toEqual({
      name: "azure-key",
      keys: [["Key", "key"]],
    });
  });

  it("shows the workload identity of GCS and Azure", () => {
    const gcs = contractFacts(
      objectStorage({
        type: "GCS",
        gcs: {
          bucketName: "acme",
          auth: {
            type: "workloadIdentity",
            workloadIdentity: { serviceAccountEmail: "sa@p.iam" },
          },
        },
      }),
    );
    const azure = contractFacts(
      objectStorage({
        type: "AzureBlob",
        azureBlob: {
          accountName: "acme",
          container: "backups",
          auth: {
            type: "workloadIdentity",
            workloadIdentity: { clientId: "1234" },
          },
        },
      }),
    );
    expect(fact(gcs, "Service account email")?.value).toBe("sa@p.iam");
    expect(fact(azure, "Client ID")?.value).toBe("1234");
  });

  it("shows the client secret with its namespace", () => {
    expect(fact(contractFacts(auth), "Client secret")).toEqual({
      label: "Client secret",
      secret: {
        namespace: "platform",
        name: "optimize-client",
        keys: [["Key", "secret"]],
      },
    });
  });

  it("puts the replica hint on the node count", () => {
    expect(NODE_COUNT_HINT).toBe(
      "Sets the index replicas when the cluster sets no replica count.",
    );
    const facts = contractFacts(
      secondary({
        type: "elasticsearch",
        elasticsearch: {
          endpoint: "https://es:9200",
          nodeCount: 3,
          credentialsSecretRef: { name: "es-user" },
        },
      }),
    );
    expect(fact(facts, "Node count")).toEqual({
      label: "Node count",
      value: "3",
      note: NODE_COUNT_HINT,
    });
  });

  it("leaves out an unset optional field", () => {
    const facts = contractFacts(auth);
    expect(fact(facts, "Issuer backend URL")).toBeUndefined();
    const es = contractFacts(
      secondary({
        type: "elasticsearch",
        elasticsearch: {
          endpoint: "https://es:9200",
          credentialsSecretRef: { name: "es-user" },
        },
      }),
    );
    expect(es.map((f) => f.label)).toEqual(["Type", "Endpoint", "Credentials"]);
    expect(
      fact(
        contractFacts({
          ...database,
          spec: { ...database.spec, backupCredentialsSecretRef: undefined },
        }),
        "Backup credentials",
      ),
    ).toBeUndefined();
  });

  it("shows the database config of an RDBMS contract", () => {
    expect(
      contractFacts(
        secondary({ type: "rdbms", rdbms: { databaseConfigRef: "orders-db" } }),
      ),
    ).toEqual([
      { label: "Type", value: "RDBMS" },
      { label: "Database config", value: "orders-db" },
    ]);
  });

  it("shows every field of a Management Identity contract", () => {
    expect(
      contractFacts({
        ...auth,
        spec: {
          ...auth.spec,
          issuerBackendUrl: "http://keycloak:8080/realms/camunda",
        },
      }).map((f) => f.label),
    ).toEqual([
      "Issuer URL",
      "Issuer backend URL",
      "Auth URL",
      "Token URL",
      "JWKS URL",
      "Base URL",
      "Client ID",
      "Audience",
      "Client secret",
    ]);
  });
});

describe("keyFacts", () => {
  it("gives the type and the bucket of an object storage contract", () => {
    expect(
      keyFacts(objectStorage({ type: "S3", s3: { bucketName: "acme" } })),
    ).toEqual([
      { label: "Type", value: "S3" },
      { label: "Bucket", value: "acme" },
    ]);
    expect(
      keyFacts(
        objectStorage({
          type: "AzureBlob",
          azureBlob: { accountName: "acme", container: "backups" },
        }),
      ),
    ).toEqual([
      { label: "Type", value: "Azure Blob" },
      { label: "Bucket", value: "acme/backups" },
    ]);
  });

  it("gives the type and the endpoint or database config of a secondary storage contract", () => {
    expect(
      keyFacts(
        secondary({
          type: "elasticsearch",
          elasticsearch: {
            endpoint: "https://es:9200",
            credentialsSecretRef: { name: "u" },
          },
        }),
      ),
    ).toEqual([
      { label: "Type", value: "Elasticsearch" },
      { label: "Endpoint", value: "https://es:9200" },
    ]);
    expect(
      keyFacts(
        secondary({ type: "rdbms", rdbms: { databaseConfigRef: "orders-db" } }),
      ),
    ).toEqual([
      { label: "Type", value: "RDBMS" },
      { label: "Endpoint", value: "Database config orders-db" },
    ]);
  });

  it("gives the host and port of a database server contract", () => {
    expect(keyFacts(server)).toEqual([
      { label: "Host", value: "pg.example.com:5432" },
    ]);
  });

  it("gives the database and the server of a database contract", () => {
    expect(keyFacts(database)).toEqual([
      { label: "Database", value: "orders" },
      { label: "Server", value: "postgres" },
    ]);
  });

  it("gives the issuer and the client ID of a Management Identity contract", () => {
    expect(keyFacts(auth)).toEqual([
      { label: "Issuer", value: "https://id.example.com/realms/camunda" },
      { label: "Client ID", value: "optimize" },
    ]);
  });

  it("has the labels of KEY_FACT_LABELS for each kind", () => {
    expect(keyFacts(server).map((f) => f.label)).toEqual(
      KEY_FACT_LABELS.DatabaseServerConfig,
    );
    expect(keyFacts(auth).map((f) => f.label)).toEqual(
      KEY_FACT_LABELS.ManagementAuthConfig,
    );
  });
});
