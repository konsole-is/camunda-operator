import "server-only";
import { getJSON, kubeConfig, unlessNotFound } from "@/lib/k8s/client";
import { KindMap, objectPath } from "@/lib/k8s/kinds";
import type { KubeObject } from "@/lib/write/types";
import { WRITE_TIMEOUT_MS } from "@/lib/write/writer";

/**
 * readSecretValues returns the decoded values of a Secret, or undefined
 * when it does not exist. Only modules under src/lib/ may import it, and no
 * caller may return its result or a part of it.
 */
export async function readSecretValues(
  namespace: string,
  name: string,
): Promise<Record<string, string> | undefined> {
  const secret = await readSecret(namespace, name);
  if (!secret) return undefined;
  return Object.fromEntries(
    Object.entries(secret.data ?? {}).map(([key, value]) => [
      key,
      Buffer.from(value, "base64").toString("utf8"),
    ]),
  );
}

/** RawSecret is a Secret as the API server returns it, with base64 data. */
export type RawSecret = KubeObject & { data?: Record<string, string> };

/** readSecret returns the Secret with its base64 data, or undefined when it does not exist. */
export function readSecret(
  namespace: string,
  name: string,
): Promise<RawSecret | undefined> {
  return unlessNotFound(
    getJSON<RawSecret>(
      kubeConfig(),
      objectPath(KindMap.Secret, namespace, name),
      WRITE_TIMEOUT_MS,
    ),
  );
}
