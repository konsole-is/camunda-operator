import { refKey, refOf } from "./refs";
import type { Change, KubeObject } from "./types";

/** NewNamespace is a namespace that a create change makes, with all its labels. */
export type NewNamespace = { name: string; labels: Record<string, string> };

/**
 * createChangeOf returns the create change of claim: the Namespace first
 * when namespace is given, then the claim, and create lists both keys.
 */
export function createChangeOf(
  intent: string,
  claim: KubeObject,
  namespace?: NewNamespace,
): Change {
  const apply: KubeObject[] =
    namespace === undefined
      ? [claim]
      : [
          {
            apiVersion: "v1",
            kind: "Namespace",
            metadata: { name: namespace.name, labels: namespace.labels },
          },
          claim,
        ];
  return { intent, apply, create: apply.map((o) => refKey(refOf(o))) };
}
