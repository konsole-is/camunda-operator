import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import {
  SecretRefsReaderContext,
  staticSecretRefs,
} from "@/components/fleet/namespaced-secret-ref-picker";
import type { ObjectStorageConfig } from "@/generated/crds/objectstorageconfig";
import { ContractForm } from "./contract-form";
import { draftOf } from "./draft";
import type { ContractObject } from "./facts";
import { connectionFixtures } from "./fixtures";

// A story must not read live Secrets, so the pickers read fixed lists.
const SECRETS = staticSecretRefs({
  "team-a": [
    { name: "shared-es-user", keys: ["user", "pass"] },
    { name: "shared-es-ca", keys: ["ca.crt"] },
    { name: "s3-keys", keys: ["access-key-id", "secret-access-key"] },
    { name: "gcs-key", keys: ["key.json"] },
    { name: "postgres-admin", keys: ["username", "password"] },
    { name: "orders-db", keys: ["user", "pass"] },
    { name: "orders-db-backup", keys: ["user", "pass"] },
  ],
  platform: [{ name: "plane-a-optimize-client", keys: ["client-secret"] }],
});

const sources = connectionFixtures();
const backups = sources.objectStorageConfigs[0];

function objectStorage(spec: ObjectStorageConfig["spec"]): ContractObject {
  return { ...backups, spec };
}

function argsOf(contract: ContractObject) {
  return {
    kind: contract.kind,
    namespace: contract.metadata.namespace,
    draft: draftOf(contract),
  };
}

const meta = {
  title: "setup/connections/ContractForm",
  component: ContractForm,
  args: {
    ...argsOf(backups),
    errors: [],
    namespaces: ["camunda-system", "platform", "team-a"],
    databaseConfigs: ["orders-db", "billing-db"],
    databaseServerConfigs: ["postgres"],
    onChange: fn(),
  },
  decorators: [
    (Story) => (
      <SecretRefsReaderContext.Provider value={SECRETS}>
        <div className="max-w-3xl">
          <Story />
        </div>
      </SecretRefsReaderContext.Provider>
    ),
  ],
} satisfies Meta<typeof ContractForm>;
export default meta;
type Story = StoryObj<typeof meta>;

export const S3WithWorkloadIdentity: Story = {
  name: "S3 with workload identity",
};

export const S3WithAKeyPair: Story = {
  name: "S3 with a key pair",
  args: argsOf(
    objectStorage({
      type: "S3",
      s3: {
        bucketName: "acme-backups",
        endpoint: "https://minio.team-a:9000",
        forcePathStyle: true,
        auth: {
          type: "credentials",
          credentials: {
            secretRef: {
              name: "s3-keys",
              accessKeyIdKey: "access-key-id",
              secretAccessKeyKey: "secret-access-key",
            },
          },
        },
      },
    }),
  ),
};

export const GcsWithAKey: Story = {
  name: "GCS with a key",
  args: argsOf(
    objectStorage({
      type: "GCS",
      gcs: {
        bucketName: "acme-backups",
        basePath: "orders",
        auth: {
          type: "credentials",
          credentials: { secretRef: { name: "gcs-key", key: "key.json" } },
        },
      },
    }),
  ),
};

export const AzureWithWorkloadIdentity: Story = {
  name: "Azure with workload identity",
  args: argsOf(
    objectStorage({
      type: "AzureBlob",
      azureBlob: {
        accountName: "acmebackups",
        container: "orders",
        auth: {
          type: "workloadIdentity",
          workloadIdentity: {
            clientId: "8f1c2a90-3c41-4f4e-9d1b-1f0b3e7c2d55",
          },
        },
      },
    }),
  ),
};

export const ElasticsearchWithACaCertificate: Story = {
  name: "Elasticsearch with a CA certificate",
  args: argsOf(sources.secondaryStorageConfigs[1]),
};

export const Rdbms: Story = {
  name: "RDBMS",
  args: argsOf({
    ...sources.secondaryStorageConfigs[1],
    spec: { type: "rdbms", rdbms: { databaseConfigRef: "orders-db" } },
  }),
};

export const DatabaseServer: Story = {
  name: "Database server",
  args: argsOf(sources.databaseServerConfigs[0]),
};

export const DatabaseWithBackupCredentials: Story = {
  name: "Database with backup credentials",
  args: argsOf({
    ...sources.databaseConfigs[0],
    spec: {
      ...sources.databaseConfigs[0].spec,
      backupCredentialsSecretRef: { name: "orders-db-backup" },
    },
  }),
};

export const ManagementIdentity: Story = {
  name: "Management Identity",
  args: argsOf(sources.managementAuthConfigs[0]),
};

export const DraftErrors: Story = {
  name: "Draft errors",
  args: {
    ...argsOf(objectStorage({ type: "S3", s3: { bucketName: "" } })),
    errors: [
      { path: "spec.s3.bucketName", message: "Enter the bucket name." },
      { path: "spec.s3.region", message: "Enter the region, or an endpoint." },
    ],
  },
};
