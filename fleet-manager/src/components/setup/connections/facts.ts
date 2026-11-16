import type { DatabaseConfig } from "@/generated/crds/databaseconfig";
import type { DatabaseServerConfig } from "@/generated/crds/databaseserverconfig";
import type { ManagementAuthConfig } from "@/generated/crds/managementauthconfig";
import type { ObjectStorageConfig } from "@/generated/crds/objectstorageconfig";
import type { SecondaryStorageConfig } from "@/generated/crds/secondarystorageconfig";
import type { ContractKind } from "@/lib/usage";

/** ContractObject is a contract of any kind of CONTRACT_KINDS. */
export type ContractObject =
  | ObjectStorageConfig
  | SecondaryStorageConfig
  | DatabaseServerConfig
  | DatabaseConfig
  | ManagementAuthConfig;

/** NODE_COUNT_HINT says what nodeCount of an Elasticsearch contract changes. */
export const NODE_COUNT_HINT =
  "Sets the index replicas when the cluster sets no replica count.";

/**
 * SecretView is one Secret reference: the Secret, and each key that the
 * contract reads from it with its label. It never holds a value.
 */
export type SecretView = {
  namespace?: string;
  name: string;
  keys: readonly (readonly [label: string, key: string])[];
};

/** Fact is one setting of a contract: a value or a Secret reference, with an optional note. */
export type Fact = {
  label: string;
  value?: string;
  secret?: SecretView;
  note?: string;
};

/** KeyFacts are the kind columns of one row of the Connections list. */
export type KeyFacts = { label: string; value: string }[];

/** KEY_FACT_LABELS are the labels of the KeyFacts of each kind, in order. */
export const KEY_FACT_LABELS: Record<ContractKind, readonly string[]> = {
  ObjectStorageConfig: ["Type", "Bucket"],
  SecondaryStorageConfig: ["Type", "Endpoint"],
  DatabaseServerConfig: ["Host"],
  DatabaseConfig: ["Database", "Server"],
  ManagementAuthConfig: ["Issuer", "Client ID"],
};

const STORAGE_TYPE: Record<ObjectStorageConfig["spec"]["type"], string> = {
  S3: "S3",
  GCS: "GCS",
  AzureBlob: "Azure Blob",
};

const SECONDARY_TYPE: Record<SecondaryStorageConfig["spec"]["type"], string> = {
  elasticsearch: "Elasticsearch",
  rdbms: "RDBMS",
};

const ENGINE: Record<DatabaseServerConfig["spec"]["engine"], string> = {
  postgres: "PostgreSQL",
};

const AUTH_TYPE = {
  workloadIdentity: "Workload identity",
  credentials: "Credentials",
} as const;

/**
 * contractFacts returns the settings of contract in reading order. An
 * unset optional field gives no fact. A username or password key that the
 * contract leaves unset shows its default, username or password.
 */
export function contractFacts(contract: ContractObject): Fact[] {
  switch (contract.kind) {
    case "ObjectStorageConfig":
      return objectStorageFacts(contract.spec);
    case "SecondaryStorageConfig":
      return secondaryStorageFacts(contract.spec);
    case "DatabaseServerConfig":
      return compact([
        value("Engine", ENGINE[contract.spec.engine] ?? contract.spec.engine),
        value("Host", contract.spec.host),
        value("Port", String(contract.spec.port)),
        {
          label: "Admin credentials",
          secret: userPassword(contract.spec.adminCredentialsSecretRef),
        },
        ...pitrFacts(contract.spec.pitr),
      ]);
    case "DatabaseConfig":
      return compact([
        value("Database name", contract.spec.databaseName),
        value("Server", contract.spec.serverRef),
        {
          label: "Credentials",
          secret: userPassword(contract.spec.credentialsSecretRef),
        },
        contract.spec.backupCredentialsSecretRef && {
          label: "Backup credentials",
          secret: userPassword(contract.spec.backupCredentialsSecretRef),
        },
      ]);
    case "ManagementAuthConfig": {
      const { spec } = contract;
      return compact([
        value("Issuer URL", spec.issuerUrl),
        value("Issuer backend URL", spec.issuerBackendUrl),
        value("Auth URL", spec.authUrl),
        value("Token URL", spec.tokenUrl),
        value("JWKS URL", spec.jwksUrl),
        value("Base URL", spec.baseUrl),
        value("Client ID", spec.clientId),
        value("Audience", spec.audience),
        {
          label: "Client secret",
          secret: {
            namespace: spec.clientSecretRef.namespace,
            name: spec.clientSecretRef.name,
            keys: [["Key", spec.clientSecretRef.key]],
          },
        },
      ]);
    }
  }
}

/** keyFacts returns the kind columns of contract, with the labels of KEY_FACT_LABELS. A missing value is "". */
export function keyFacts(contract: ContractObject): KeyFacts {
  const labels = KEY_FACT_LABELS[contract.kind];
  return keyValues(contract).map((v, i) => ({ label: labels[i], value: v }));
}

function keyValues(contract: ContractObject): string[] {
  switch (contract.kind) {
    case "ObjectStorageConfig": {
      const { spec } = contract;
      const bucket =
        spec.type === "AzureBlob"
          ? spec.azureBlob &&
            `${spec.azureBlob.accountName}/${spec.azureBlob.container}`
          : spec.type === "GCS"
            ? spec.gcs?.bucketName
            : spec.s3?.bucketName;
      return [STORAGE_TYPE[spec.type] ?? spec.type, bucket ?? ""];
    }
    case "SecondaryStorageConfig": {
      const { spec } = contract;
      const target =
        spec.type === "rdbms"
          ? spec.rdbms && `Database config ${spec.rdbms.databaseConfigRef}`
          : spec.elasticsearch?.endpoint;
      return [SECONDARY_TYPE[spec.type] ?? spec.type, target ?? ""];
    }
    case "DatabaseServerConfig":
      return [`${contract.spec.host}:${contract.spec.port}`];
    case "DatabaseConfig":
      return [contract.spec.databaseName, contract.spec.serverRef];
    case "ManagementAuthConfig":
      return [contract.spec.issuerUrl, contract.spec.clientId];
  }
}

function objectStorageFacts(spec: ObjectStorageConfig["spec"]): Fact[] {
  const type: Fact = {
    label: "Type",
    value: STORAGE_TYPE[spec.type] ?? spec.type,
  };
  switch (spec.type) {
    case "S3": {
      const s3 = spec.s3;
      if (!s3) return [type];
      const secret = s3.auth?.credentials?.secretRef;
      return compact([
        type,
        value("Bucket", s3.bucketName),
        value("Region", s3.region),
        value("Endpoint", s3.endpoint),
        value("Base path", s3.basePath),
        s3.forcePathStyle !== undefined &&
          value("Path-style access", s3.forcePathStyle ? "Yes" : "No"),
        s3.auth?.type && value("Auth", AUTH_TYPE[s3.auth.type]),
        secret && {
          label: "Credentials",
          secret: {
            name: secret.name,
            keys: [
              ["Access key ID key", secret.accessKeyIdKey],
              ["Secret access key key", secret.secretAccessKeyKey],
            ],
          },
        },
        value("Role ARN", s3.auth?.workloadIdentity?.roleArn),
      ]);
    }
    case "GCS": {
      const gcs = spec.gcs;
      if (!gcs) return [type];
      return compact([
        type,
        value("Bucket", gcs.bucketName),
        value("Base path", gcs.basePath),
        gcs.auth?.type && value("Auth", AUTH_TYPE[gcs.auth.type]),
        keyFact(gcs.auth?.credentials?.secretRef),
        value(
          "Service account email",
          gcs.auth?.workloadIdentity?.serviceAccountEmail,
        ),
      ]);
    }
    case "AzureBlob": {
      const azure = spec.azureBlob;
      if (!azure) return [type];
      return compact([
        type,
        value("Account", azure.accountName),
        value("Container", azure.container),
        value("Endpoint", azure.endpoint),
        value("Base path", azure.basePath),
        azure.auth?.type && value("Auth", AUTH_TYPE[azure.auth.type]),
        keyFact(azure.auth?.credentials?.secretRef),
        value("Client ID", azure.auth?.workloadIdentity?.clientId),
      ]);
    }
  }
}

function secondaryStorageFacts(spec: SecondaryStorageConfig["spec"]): Fact[] {
  const type: Fact = {
    label: "Type",
    value: SECONDARY_TYPE[spec.type] ?? spec.type,
  };
  if (spec.type === "rdbms") {
    return compact([
      type,
      value("Database config", spec.rdbms?.databaseConfigRef),
    ]);
  }
  const es = spec.elasticsearch;
  if (!es) return [type];
  return compact([
    type,
    value("Endpoint", es.endpoint),
    es.nodeCount !== undefined && {
      label: "Node count",
      value: String(es.nodeCount),
      note: NODE_COUNT_HINT,
    },
    value("Snapshot repository", es.snapshotRepository),
    { label: "Credentials", secret: userPassword(es.credentialsSecretRef) },
    es.caSecretRef && {
      label: "CA certificate",
      secret: {
        name: es.caSecretRef.name,
        keys: [["Key", es.caSecretRef.key]],
      },
    },
  ]);
}

// The defaults of LocalCredentialsSecretRef in api/v1/common_types.go.
function userPassword(ref: {
  name: string;
  usernameKey?: string;
  passwordKey?: string;
}): SecretView {
  return {
    name: ref.name,
    keys: [
      ["Username key", ref.usernameKey ?? "username"],
      ["Password key", ref.passwordKey ?? "password"],
    ],
  };
}

function keyFact(
  ref: { name: string; key: string } | undefined,
): Fact | undefined {
  return (
    ref && {
      label: "Credentials",
      secret: { name: ref.name, keys: [["Key", ref.key]] },
    }
  );
}

function pitrFacts(pitr: DatabaseServerConfig["spec"]["pitr"]): Fact[] {
  if (!pitr) return [];
  if (!pitr.enabled) return [{ label: "Point-in-time recovery", value: "Off" }];
  return [
    {
      label: "Point-in-time recovery",
      value: `${pitr.retentionPeriodDays} days of retention`,
    },
    {
      label: "Rollback",
      value: pitr.recovery === "operator" ? "By the writer" : "Manual",
    },
  ];
}

function value(label: string, v: string | undefined): Fact | undefined {
  return v ? { label, value: v } : undefined;
}

function compact(facts: (Fact | undefined | false | "" | 0)[]): Fact[] {
  return facts.filter((f): f is Fact => Boolean(f));
}
