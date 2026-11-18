import type { DatabaseConfig } from "@/generated/crds/databaseconfig";
import type { DatabaseServerConfig } from "@/generated/crds/databaseserverconfig";
import type { ManagementAuthConfig } from "@/generated/crds/managementauthconfig";
import type { ObjectStorageConfig } from "@/generated/crds/objectstorageconfig";
import type { SecondaryStorageConfig } from "@/generated/crds/secondarystorageconfig";
import { LAST_APPLIED_ANNOTATION } from "@/lib/labels";
import { child } from "@/lib/merge";
import { equalJSON, isObject } from "@/lib/policy";
import type { ValidationError } from "@/lib/schema";
import type { ContractKind } from "@/lib/usage";
import { deleteChangeOf } from "@/lib/write/delete";
import { keepUntouchedEmpty } from "@/lib/write/ownership";
import { refKey } from "@/lib/write/refs";
import type { Change, KubeObject } from "@/lib/write/types";
import type { ContractObject } from "./facts";

const API_VERSION = "core.camunda.io/v1";

/** ContractDraft is the spec of one contract as the admin edits it. Its fields can be empty or incomplete. */
export type ContractDraft = { spec: ContractObject["spec"] };

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
  base: ContractObject | undefined,
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
 * and the change keeps them. An empty value of another tool that the edit
 * did not touch stays, and the review does not offer it, as
 * keepUntouchedEmpty says.
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
  return keepUntouchedEmpty(
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

function kindWords(kind: ContractKind): string {
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
