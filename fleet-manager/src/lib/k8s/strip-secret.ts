import "server-only";
import type { V1ObjectMeta } from "@kubernetes/client-node";

/**
 * SecretSummary is a Secret as the cache keeps it: its name, labels, owners,
 * and the names of its keys. It holds no value and no annotation.
 */
export interface SecretSummary {
  apiVersion: "v1";
  kind: "Secret";
  metadata: Pick<
    V1ObjectMeta,
    | "name"
    | "namespace"
    | "uid"
    | "resourceVersion"
    | "creationTimestamp"
    | "deletionTimestamp"
    | "labels"
    | "ownerReferences"
  >;
  type?: string;
  immutable?: boolean;
  /** keys are the sorted names of the keys of data and stringData. */
  keys: string[];
}

type RawSecret = {
  apiVersion?: string;
  kind?: string;
  metadata?: V1ObjectMeta;
  type?: string;
  immutable?: boolean;
  data?: Record<string, string>;
  stringData?: Record<string, string>;
};

/**
 * stripSecret returns the summary of a Secret. It drops every field that
 * SecretSummary does not name, so no value and no annotation reaches the
 * cache. An annotation can hold the values, as
 * kubectl.kubernetes.io/last-applied-configuration does.
 */
export function stripSecret(secret: RawSecret): SecretSummary {
  const m = secret.metadata ?? {};
  const metadata: SecretSummary["metadata"] = {
    name: m.name,
    namespace: m.namespace,
    uid: m.uid,
    resourceVersion: m.resourceVersion,
    creationTimestamp: m.creationTimestamp,
    deletionTimestamp: m.deletionTimestamp,
    labels: m.labels,
    ownerReferences: m.ownerReferences,
  };
  const keys = new Set([
    ...Object.keys(secret.data ?? {}),
    ...Object.keys(secret.stringData ?? {}),
  ]);
  // The round trip through JSON drops the fields that the Secret does not set.
  return JSON.parse(
    JSON.stringify({
      apiVersion: "v1",
      kind: "Secret",
      metadata,
      type: secret.type,
      immutable: secret.immutable,
      keys: [...keys].sort(),
    }),
  ) as SecretSummary;
}
