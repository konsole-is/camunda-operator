import "server-only";
import type { KubeObject } from "@/lib/write/types";

/**
 * externalSecretFor returns the ExternalSecret that keeps the Secret
 * <namespace>/<name> from the store. Each property of the store entry
 * <namespace>/<name> becomes a key of the Secret, so a new key needs no
 * change of the object.
 */
export function externalSecretFor(
  ref: { namespace: string; name: string },
  store: { kind: string; name: string },
): KubeObject {
  return {
    apiVersion: "external-secrets.io/v1",
    kind: "ExternalSecret",
    metadata: { name: ref.name, namespace: ref.namespace },
    spec: {
      refreshInterval: "1h",
      secretStoreRef: { kind: store.kind, name: store.name },
      target: { name: ref.name, creationPolicy: "Owner" },
      dataFrom: [{ extract: { key: `${ref.namespace}/${ref.name}` } }],
    },
  };
}
