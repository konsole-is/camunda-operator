import type { CamundaManagementCluster } from "@/generated/crds/camundamanagementcluster";
import type { CamundaManagementClusterClaim } from "@/generated/crds/camundamanagementclusterclaim";
import type { DatabaseConfig } from "@/generated/crds/databaseconfig";
import type { DatabaseServerConfig } from "@/generated/crds/databaseserverconfig";
import type { ManagementAuthConfig } from "@/generated/crds/managementauthconfig";
import type { ObjectStorageConfig } from "@/generated/crds/objectstorageconfig";
import type { SecondaryStorageConfig } from "@/generated/crds/secondarystorageconfig";
import { LAST_APPLIED_ANNOTATION, referenceNameError } from "@/lib/labels";
import { child } from "@/lib/merge";
import { equalJSON, isObject } from "@/lib/policy";
import type { ValidationError } from "@/lib/schema";
import type { ContractKind } from "@/lib/usage";
import { deleteChangeOf } from "@/lib/write/delete";
import { keepEmpty } from "@/lib/write/ownership";
import { refKey } from "@/lib/write/refs";
import type { Change, KubeObject } from "@/lib/write/types";
import type { ContractObject } from "./facts";

const API_VERSION = "core.camunda.io/v1";

/** ContractDraft is the spec of one contract as the admin edits it. Its fields can be empty or incomplete. */
export type ContractDraft = { spec: ContractObject["spec"] };

/** ContractCreateDraft is the form of a new contract: its name, its namespace, "" for a Management Identity contract, and its spec. */
export type ContractCreateDraft = ContractDraft & {
  name: string;
  namespace: string;
};

const OBJECT_STORAGE_BLOCK = {
  S3: "s3",
  GCS: "gcs",
  AzureBlob: "azureBlob",
} as const;
const SECONDARY_STORAGE_BLOCK = {
  elasticsearch: "elasticsearch",
  rdbms: "rdbms",
} as const;

/** draftOf returns a copy of the spec of contract, or an empty spec without a contract. */
export function draftOf(contract: ContractObject | undefined): ContractDraft {
  return {
    spec: contract
      ? structuredClone(contract.spec)
      : ({} as ContractObject["spec"]),
  };
}

/**
 * draftErrors returns the fields that the CRD of kind requires and draft
 * leaves empty, each with its spec path and a message for the admin. It
 * checks no other rule: the dry run of the review does.
 */
export function draftErrors(
  kind: ContractKind,
  draft: ContractDraft,
): ValidationError[] {
  const errors: ValidationError[] = [];
  const need = (ok: unknown, path: string, message: string) => {
    if (!ok) errors.push({ path: `spec.${path}`, message });
  };
  switch (kind) {
    case "ObjectStorageConfig": {
      const spec = draft.spec as ObjectStorageConfig["spec"];
      if (spec.type === "S3" && spec.s3) {
        const s3 = spec.s3;
        need(s3.bucketName, "s3.bucketName", "Enter the bucket name.");
        need(
          s3.region || s3.endpoint,
          "s3.region",
          "Enter the region, or an endpoint.",
        );
        if (s3.auth?.type === "credentials") {
          const ref = s3.auth.credentials?.secretRef;
          need(
            ref?.name && ref.accessKeyIdKey && ref.secretAccessKeyKey,
            "s3.auth.credentials.secretRef",
            "Pick the Secret and the keys of the access key.",
          );
        }
      }
      if (spec.type === "GCS" && spec.gcs) {
        need(spec.gcs.bucketName, "gcs.bucketName", "Enter the bucket name.");
        if (spec.gcs.auth?.type === "credentials")
          needKey(spec.gcs.auth.credentials?.secretRef, "gcs", need);
      }
      if (spec.type === "AzureBlob" && spec.azureBlob) {
        const azure = spec.azureBlob;
        need(
          azure.accountName,
          "azureBlob.accountName",
          "Enter the account name.",
        );
        need(azure.container, "azureBlob.container", "Enter the container.");
        if (azure.auth?.type === "credentials")
          needKey(azure.auth.credentials?.secretRef, "azureBlob", need);
      }
      break;
    }
    case "SecondaryStorageConfig": {
      const spec = draft.spec as SecondaryStorageConfig["spec"];
      if (spec.type === "elasticsearch" && spec.elasticsearch) {
        const es = spec.elasticsearch;
        need(es.endpoint, "elasticsearch.endpoint", "Enter the endpoint.");
        if (es.nodeCount !== undefined)
          need(
            Number.isInteger(es.nodeCount) && es.nodeCount >= 1,
            "elasticsearch.nodeCount",
            "Enter a node count of 1 or more, or leave it empty.",
          );
        need(
          es.credentialsSecretRef?.name,
          "elasticsearch.credentialsSecretRef",
          "Pick the Secret of the credentials.",
        );
        if (es.caSecretRef)
          need(
            es.caSecretRef.name && es.caSecretRef.key,
            "elasticsearch.caSecretRef",
            "Pick the Secret and the key of the CA certificate.",
          );
      }
      if (spec.type === "rdbms" && spec.rdbms)
        need(
          spec.rdbms.databaseConfigRef,
          "rdbms.databaseConfigRef",
          "Pick the database config.",
        );
      break;
    }
    case "DatabaseServerConfig": {
      const spec = draft.spec as DatabaseServerConfig["spec"];
      need(spec.host, "host", "Enter the host.");
      need(
        Number.isInteger(spec.port) && spec.port >= 1 && spec.port <= 65535,
        "port",
        "Enter a port from 1 to 65535.",
      );
      need(
        spec.adminCredentialsSecretRef?.name,
        "adminCredentialsSecretRef",
        "Pick the Secret of the admin credentials.",
      );
      break;
    }
    case "DatabaseConfig": {
      const spec = draft.spec as DatabaseConfig["spec"];
      need(spec.databaseName, "databaseName", "Enter the database name.");
      need(spec.serverRef, "serverRef", "Pick the database server.");
      need(
        spec.credentialsSecretRef?.name,
        "credentialsSecretRef",
        "Pick the Secret of the credentials.",
      );
      if (spec.backupCredentialsSecretRef)
        need(
          spec.backupCredentialsSecretRef.name,
          "backupCredentialsSecretRef",
          "Pick the Secret of the backup credentials.",
        );
      break;
    }
    case "ManagementAuthConfig": {
      const spec = draft.spec as ManagementAuthConfig["spec"];
      need(spec.issuerUrl, "issuerUrl", "Enter the issuer URL.");
      need(spec.authUrl, "authUrl", "Enter the auth URL.");
      need(spec.tokenUrl, "tokenUrl", "Enter the token URL.");
      need(spec.jwksUrl, "jwksUrl", "Enter the JWKS URL.");
      need(spec.baseUrl, "baseUrl", "Enter the base URL.");
      need(spec.clientId, "clientId", "Enter the client ID.");
      need(spec.audience, "audience", "Enter the audience.");
      const ref = spec.clientSecretRef;
      need(
        ref?.namespace && ref.name && ref.key,
        "clientSecretRef",
        "Pick the namespace, the Secret, and the key of the client secret.",
      );
      break;
    }
  }
  return errors;
}

function needKey(
  ref: { name?: string; key?: string } | undefined,
  block: string,
  need: (ok: unknown, path: string, message: string) => void,
) {
  need(
    ref?.name && ref.key,
    `${block}.auth.credentials.secretRef`,
    "Pick the Secret and the key of the credentials.",
  );
}

/** changeCount counts the fields of the spec that draft changes from the spec of base. An empty string counts as no value. */
export function changeCount(
  base: ContractDraft | undefined,
  draft: ContractDraft,
): number {
  return countChanged(compact(base?.spec ?? {}), compact(draft.spec));
}

function countChanged(a: unknown, b: unknown): number {
  if (isObject(a) && isObject(b)) {
    const keys = new Set([...Object.keys(a), ...Object.keys(b)]);
    return [...keys].reduce((n, key) => n + countChanged(a[key], b[key]), 0);
  }
  return equalJSON(a, b) ? 0 : 1;
}

// RECOVERY_PATHS are the recovery request of a database server contract
// and its answer. The editor does not show them, and their own writers
// keep them.
const RECOVERY_PATHS = [
  child("spec", "recovery"),
  child(child("spec", "pitr"), "lastRecovery"),
];

/**
 * updateChange applies live with the spec of draft, with the
 * resourceVersion of live as its precondition, and without force. It
 * keeps the labels and the annotations of live, without the last applied
 * file of kubectl, so a contract kept from a registration keeps its plugin
 * labels. A field that draft leaves empty is left out. The recovery
 * request of a database server contract and its answer are left out too,
 * and the change keeps them.
 */
export function updateChange(
  live: ContractObject,
  draft: ContractDraft,
): Change {
  const { name = "", namespace, labels, annotations } = live.metadata;
  const kept = Object.fromEntries(
    Object.entries(annotations ?? {}).filter(
      ([key]) => key !== LAST_APPLIED_ANNOTATION,
    ),
  );
  const object: KubeObject = {
    apiVersion: API_VERSION,
    kind: live.kind,
    metadata: {
      name,
      ...(namespace !== undefined && { namespace }),
      ...(labels && { labels }),
      ...(Object.keys(kept).length > 0 && { annotations: kept }),
    },
    spec: specOf(live.kind, draft),
  };
  const guarded = preconditionOf(live);
  return keepEmpty(
    {
      intent: `Change the ${kindWords(live.kind)} contract ${name}`,
      apply: [object],
      ...guarded,
      ...(live.kind === "DatabaseServerConfig" &&
        guarded.preconditions && {
          keep: Object.fromEntries(
            Object.keys(guarded.preconditions).map((key) => [
              key,
              RECOVERY_PATHS,
            ]),
          ),
        }),
    },
    live,
    { ...object, spec: draft.spec },
  );
}

/**
 * newSpec returns the spec of a new contract of kind: an S3 object storage
 * with a workload identity, an Elasticsearch secondary storage, a
 * PostgreSQL database server, and an empty spec of the other kinds. Each
 * call gives a new object.
 */
export function newSpec(kind: ContractKind): ContractObject["spec"] {
  const specs: Record<ContractKind, () => Record<string, unknown>> = {
    ObjectStorageConfig: () => ({
      type: "S3",
      s3: { auth: { type: "workloadIdentity" } },
    }),
    SecondaryStorageConfig: () => ({
      type: "elasticsearch",
      elasticsearch: {},
    }),
    DatabaseServerConfig: () => ({ engine: "postgres" }),
    DatabaseConfig: () => ({}),
    ManagementAuthConfig: () => ({}),
  };
  return specs[kind]() as unknown as ContractObject["spec"];
}

/**
 * copyDraft returns current with a copy of the spec of source, or the new
 * spec of kind without a source. It keeps the name of current, and its
 * namespace. While current has no namespace, it takes the namespace of
 * source. A Management Identity contract has none.
 */
export function copyDraft(
  kind: ContractKind,
  source: ContractObject | undefined,
  current: ContractCreateDraft,
): ContractCreateDraft {
  return {
    name: current.name,
    namespace:
      kind === "ManagementAuthConfig"
        ? ""
        : current.namespace || (source?.metadata.namespace ?? ""),
    spec: source ? draftOf(source).spec : newSpec(kind),
  };
}

/**
 * withStorageType returns the draft of a contract of kind with the storage
 * type type and a new block of that type: a workload identity for an
 * object storage type, empty for a secondary storage type. A pick of
 * another type starts from it, so the spec holds only the block of its
 * type.
 */
export function withStorageType(
  kind: "ObjectStorageConfig" | "SecondaryStorageConfig",
  type: string,
): ContractDraft {
  const spec =
    kind === "ObjectStorageConfig"
      ? {
          type,
          [OBJECT_STORAGE_BLOCK[type as keyof typeof OBJECT_STORAGE_BLOCK]]: {
            auth: { type: "workloadIdentity" },
          },
        }
      : {
          type,
          [SECONDARY_STORAGE_BLOCK[
            type as keyof typeof SECONDARY_STORAGE_BLOCK
          ]]: {},
        };
  return { spec: spec as unknown as ContractDraft["spec"] };
}

/**
 * withAuthType returns draft, an object storage contract, with the auth
 * type type. The block of its storage type keeps its other fields, and the
 * fields of the old auth type go.
 */
export function withAuthType(
  draft: ContractDraft,
  type: "workloadIdentity" | "credentials",
): ContractDraft {
  const spec = draft.spec as ObjectStorageConfig["spec"];
  const block = OBJECT_STORAGE_BLOCK[spec.type];
  const fields = (spec as unknown as Record<string, unknown>)[block];
  return {
    spec: {
      ...spec,
      [block]: { ...(isObject(fields) ? fields : {}), auth: { type } },
    } as unknown as ContractDraft["spec"],
  };
}

/**
 * nameErrors returns the error of name for a new contract of kind, with
 * the path "name", or none. taken are the names of the contracts of kind
 * in the namespace of the new contract, or all of them for a Management
 * Identity contract. heldBy is "<namespace>/<name>" of a management plane
 * or a plane claim that writes a Management Identity contract of name.
 */
export function nameErrors(
  kind: ContractKind,
  name: string,
  taken: readonly string[],
  heldBy?: string,
): ValidationError[] {
  const error = (message: string) => [{ path: "name", message }];
  if (name === "") return error("Give the contract a name.");
  // The create flow of a management plane uses the rule of
  // spec.managementAuthConfigName of a CamundaManagementClusterClaim for
  // every contract name.
  const invalid = referenceNameError(name);
  if (invalid) return error(invalid);
  const clusterScoped = kind === "ManagementAuthConfig";
  if (taken.includes(name))
    return error(
      clusterScoped
        ? "A contract with this name exists."
        : "A contract with this name exists in this namespace.",
    );
  if (clusterScoped && heldBy !== undefined)
    return error(
      `The management plane ${heldBy} writes a contract with this name.`,
    );
  return [];
}

/** namespaceErrors returns the error of namespace for a new contract of kind, with the path "namespace", or none. A Management Identity contract has no namespace. */
export function namespaceErrors(
  kind: ContractKind,
  namespace: string,
): ValidationError[] {
  return kind !== "ManagementAuthConfig" && namespace === ""
    ? [{ path: "namespace", message: "Pick the namespace of the contract." }]
    : [];
}

/**
 * planeContractHolder returns "<namespace>/<name>" of the first plane of
 * planes, then of planeClaims, that writes the Management Identity
 * contract name, or undefined. A plane writes the contract that
 * spec.managementAuthConfigName names, or the contract of its own name,
 * as ContractName of the management cluster component does.
 */
export function planeContractHolder(
  name: string,
  planes: readonly CamundaManagementCluster[],
  planeClaims: readonly CamundaManagementClusterClaim[],
): string | undefined {
  const holder = [...planes, ...planeClaims].find(
    (p) => (p.spec?.managementAuthConfigName || p.metadata.name) === name,
  );
  return holder && `${holder.metadata.namespace ?? ""}/${holder.metadata.name}`;
}

/**
 * createChange creates the contract name of kind in namespace with the
 * spec of draft, with no labels and no annotations, so the contract shows
 * as created by hand. A Management Identity contract has no namespace.
 * The write fails with Stale when the contract exists.
 */
export function createChange(
  kind: ContractKind,
  namespace: string,
  name: string,
  draft: ContractDraft,
): Change {
  const ref = {
    kind,
    ...(kind !== "ManagementAuthConfig" && { namespace }),
    name,
  };
  return {
    intent: `Create the ${kindWords(kind)} contract ${name}`,
    apply: [
      {
        apiVersion: API_VERSION,
        kind,
        metadata: {
          name,
          ...(ref.namespace !== undefined && { namespace: ref.namespace }),
        },
        spec: specOf(kind, draft),
      },
    ],
    create: [refKey(ref)],
  };
}

/** deleteChange deletes live when it still has its resourceVersion and its UID. */
export function deleteChange(live: ContractObject): Change {
  return deleteChangeOf(
    [{ apiVersion: API_VERSION, kind: live.kind, metadata: live.metadata }],
    `Delete the ${kindWords(live.kind)} contract ${live.metadata.name ?? ""}`,
  );
}

function specOf(
  kind: ContractKind,
  draft: ContractDraft,
): Record<string, unknown> {
  const spec = compact(draft.spec) as Record<string, unknown>;
  if (kind === "DatabaseServerConfig") {
    delete spec.recovery;
    if (isObject(spec.pitr)) delete spec.pitr.lastRecovery;
  }
  return spec;
}

const KIND_WORDS: Record<ContractKind, string> = {
  ObjectStorageConfig: "object storage",
  SecondaryStorageConfig: "secondary storage",
  DatabaseServerConfig: "database server",
  DatabaseConfig: "database",
  ManagementAuthConfig: "Management Identity",
};

/** kindWords returns the words of kind in a sentence, such as object storage. */
export function kindWords(kind: ContractKind): string {
  return KIND_WORDS[kind];
}

function preconditionOf(live: ContractObject): Pick<Change, "preconditions"> {
  const { name = "", namespace, resourceVersion } = live.metadata;
  return resourceVersion === undefined
    ? {}
    : {
        preconditions: {
          [refKey({ kind: live.kind, namespace, name })]: resourceVersion,
        },
      };
}

// compact returns a copy of value without the empty strings and the
// undefined values of its objects, at any depth.
function compact(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(compact);
  if (!isObject(value)) return value;
  return Object.fromEntries(
    Object.entries(value).flatMap(([key, v]) =>
      v === undefined || v === "" ? [] : [[key, compact(v)]],
    ),
  );
}
