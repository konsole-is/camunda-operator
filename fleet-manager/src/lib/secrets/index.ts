import "server-only";
import { list } from "@/lib/k8s/read";
import { inOrder } from "@/lib/serial";
import { getSettings } from "@/lib/settings";
import type { KubeObject, WriteResult, Writer } from "@/lib/write/types";
import { getWriter, refKey } from "@/lib/write/writer";
import { externalSecretFor } from "./external-secret";
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
 *
 * In Git mode it commits an ExternalSecret that reads the Secret from the
 * store of the settings, and value is not used: the admin puts it in the
 * store entry <namespace>/<name>. It returns Invalid when the settings
 * name no store.
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
  if (writer.mode === "git") return writeToGit(writer, ref);
  // An apply of a missing Secret holds only the new key, so a second apply
  // at the same time would remove the key of the first. The writes of one
  // Secret in this process run one at a time.
  return inOrder(`${ref.namespace}/${ref.name}`, () =>
    writeDirect(writer, ref, value),
  );
}

async function writeDirect(
  writer: Writer,
  ref: SecretRef,
  value: string,
): Promise<WriteResult> {
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
  const data = {
    ...live?.data,
    [ref.key]: Buffer.from(value, "utf8").toString("base64"),
  };
  // The apply holds every field of the live Secret. A field that it leaves
  // out and that only the fleet manager set would be removed.
  const object: KubeObject = live
    ? { ...live, data }
    : {
        apiVersion: "v1",
        kind: "Secret",
        metadata: { name: ref.name, namespace: ref.namespace },
        data,
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

async function writeToGit(
  writer: Writer,
  ref: SecretRef,
): Promise<WriteResult> {
  const settings = await getSettings();
  const store = {
    kind: settings["secretStore.kind"],
    name: settings["secretStore.name"],
  };
  if (!store.kind || !store.name) {
    return {
      ok: false,
      errors: [
        {
          code: "Invalid",
          path: "secretStore",
          message:
            "Set secretStore.kind and secretStore.name in the settings before you write a Secret in Git mode",
        },
      ],
    };
  }
  return writer.apply({
    intent: `Set the Secret ${ref.namespace}/${ref.name} from the store`,
    apply: [externalSecretFor(ref, store)],
  });
}
