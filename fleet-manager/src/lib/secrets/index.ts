import "server-only";
import { list } from "@/lib/k8s/read";
import type { WriteResult } from "@/lib/write/types";
import { getWriter, refKey } from "@/lib/write/writer";
import { readSecret } from "./values";

/** SecretRef names one key of one Secret. */
export type SecretRef = { namespace: string; name: string; key: string };

const SECRET_KEY = /^[-._a-zA-Z0-9]+$/;

/**
 * listSecretRefs returns the Secrets of namespace by name, each with the
 * sorted names of its keys, from the cache. It returns no value.
 */
export async function listSecretRefs(
  namespace: string,
): Promise<{ name: string; keys: string[] }[]> {
  const secrets = await list("Secret", { namespace });
  return secrets.map((s) => ({ name: s.metadata.name ?? "", keys: s.keys }));
}

/**
 * writeSecretValue sets one key of a Secret and keeps its other keys. It
 * creates the Secret when it is missing. It returns Invalid without a
 * request for a key name that a Secret cannot hold, and Stale when the
 * Secret changed between its read and its write. The result never holds
 * the value.
 */
export async function writeSecretValue(
  ref: SecretRef,
  value: string,
): Promise<WriteResult> {
  if (!SECRET_KEY.test(ref.key)) {
    return {
      ok: false,
      errors: [
        {
          code: "Invalid",
          path: "key",
          message: `${ref.key} is not a valid Secret key`,
        },
      ],
    };
  }
  const writer = await getWriter();
  if (writer.mode !== "direct") {
    return {
      ok: false,
      errors: [
        {
          code: "Unavailable",
          message: "Secrets in Git mode come with the Git writer",
        },
      ],
    };
  }
  let live;
  try {
    live = await readSecret(ref.namespace, ref.name);
  } catch (error) {
    return {
      ok: false,
      errors: [
        {
          code: "Unavailable",
          message: error instanceof Error ? error.message : String(error),
        },
      ],
    };
  }
  const object = {
    apiVersion: "v1",
    kind: "Secret",
    metadata: { name: ref.name, namespace: ref.namespace },
    // The apply holds every key of the live Secret. A key that it leaves
    // out and that only the fleet manager set would be removed.
    data: {
      ...live?.data,
      [ref.key]: Buffer.from(value, "utf8").toString("base64"),
    },
  };
  const resourceVersion = live?.metadata.resourceVersion;
  return writer.apply({
    intent: `Set the key ${ref.key} of the Secret ${ref.namespace}/${ref.name}`,
    // force takes over a key that another manager, such as kubectl edit,
    // set. The resourceVersion of the read still refuses a stale write.
    force: true,
    apply: [object],
    ...(resourceVersion && {
      preconditions: {
        [refKey({ kind: "Secret", ...object.metadata })]: resourceVersion,
      },
    }),
  });
}
