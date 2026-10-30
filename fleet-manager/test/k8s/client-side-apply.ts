import type {
  KubernetesObject,
  KubernetesObjectApi,
} from "@kubernetes/client-node";
import { LAST_APPLIED_ANNOTATION } from "@/lib/labels";
import type { KubeObject } from "@/lib/write/types";

/**
 * createClientSideApplied creates object as kubectl apply without
 * --server-side does: with object as its last applied file, and an Update
 * entry of the field manager kubectl-client-side-apply.
 */
export function createClientSideApplied(
  api: KubernetesObjectApi,
  object: KubeObject,
): Promise<KubernetesObject> {
  const annotations = (object.metadata.annotations ?? {}) as Record<
    string,
    string
  >;
  return api.create(
    {
      ...object,
      metadata: {
        ...object.metadata,
        annotations: {
          ...annotations,
          [LAST_APPLIED_ANNOTATION]: JSON.stringify(object),
        },
      },
    },
    undefined,
    undefined,
    "kubectl-client-side-apply",
  );
}
