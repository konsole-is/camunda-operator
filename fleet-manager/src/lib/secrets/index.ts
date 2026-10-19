import "server-only";
import { KubernetesRequestError } from "@/lib/k8s/client";
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
const LABEL = "[a-z0-9]([-a-z0-9]*[a-z0-9])?";
const SECRET_NAME = new RegExp(`^${LABEL}(\\.${LABEL})*$`);
const NAMESPACE = new RegExp(`^${LABEL}$`);

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
 * request for a name, a namespace, or a key that a Secret cannot have, and
 * Stale when the
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
  const refusal = refusalOf(ref);
  if (refusal) return { ok: false, errors: [{ code: "Invalid", ...refusal }] };
  const writer = await getWriter();
  if (writer.mode === "git") return writeToGit(writer, ref);
  // An apply of a missing Secret holds only the new key, so a second apply
  // at the same time would remove the key of the first.
  return inOrder(`${ref.namespace}/${ref.name}`, () =>
    writeDirect(writer, ref, value),
  );
}

function refusalOf(
  ref: SecretRef,
): { path: string; message: string } | undefined {
  if (!SECRET_NAME.test(ref.name) || ref.name.length > 253) {
    return { path: "name", message: `${ref.name} is not a valid Secret name` };
  }
  if (!NAMESPACE.test(ref.namespace) || ref.namespace.length > 63) {
    return {
      path: "namespace",
      message: `${ref.namespace} is not a valid namespace`,
    };
  }
  if (!SECRET_KEY.test(ref.key)) {
    return { path: "key", message: `${ref.key} is not a valid Secret key` };
  }
  return undefined;
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
    // The text of a failed read can quote the Secret.
    const reason =
      error instanceof KubernetesRequestError
        ? `the API server answered HTTP ${error.statusCode}`
        : "the API server did not answer";
    return {
      ok: false,
      errors: [
        {
          code: "Unavailable",
          message: `Could not read the Secret ${ref.namespace}/${ref.name}: ${reason}`,
        },
      ],
    };
  }
  const data = {
    ...live?.data,
    [ref.key]: Buffer.from(value, "utf8").toString("base64"),
  };
  // A field that the apply leaves out and that only the fleet manager set
  // would be removed.
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
