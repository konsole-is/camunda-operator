import "server-only";
import { KubernetesRequestError, getJSON, kubeConfig } from "@/lib/k8s/client";
import { inOrder } from "@/lib/serial";
import type { KubeObject, WriteResult } from "@/lib/write/types";
import { WRITE_TIMEOUT_MS, getWriter, refKey } from "@/lib/write/writer";
import {
  SETTINGS_CONFIGMAP,
  type Settings,
  appNamespace,
  checkPatch,
  seedFromEnv,
  settingsFromData,
} from "./keys";

type ConfigMap = KubeObject & { data?: Record<string, string> };

/**
 * getSettings returns the settings from the ConfigMap fleet-manager-settings
 * in FLEET_NAMESPACE. When the ConfigMap is missing, it creates it from the
 * FLEET_SETTINGS_* variables first. It never changes a ConfigMap that
 * exists. It throws when the API server refuses the read or the create.
 */
export async function getSettings(): Promise<Settings> {
  const live =
    (await read()) ?? (await inOrder(SETTINGS_CONFIGMAP, readOrCreate));
  return settingsFromData(live.data);
}

/**
 * updateSettings sets the keys of patch and keeps every other key of the
 * ConfigMap, also a key that the fleet manager does not know. An empty
 * string unsets a setting. It returns Invalid without a request for a patch
 * that checkPatch refuses, and Stale when the ConfigMap changed between its
 * read and its write. It never retries.
 */
export async function updateSettings(
  patch: Partial<Settings>,
): Promise<WriteResult> {
  const errors = checkPatch(patch);
  if (errors.length) return { ok: false, errors };
  return inOrder(SETTINGS_CONFIGMAP, () => writePatch(patch));
}

async function writePatch(patch: Partial<Settings>): Promise<WriteResult> {
  let live: ConfigMap;
  try {
    live = await readOrCreate();
  } catch (error) {
    return {
      ok: false,
      errors: [{ code: "Unavailable", message: messageOf(error) }],
    };
  }
  const object = settingsObject({ ...live.data, ...patch });
  const writer = await getWriter();
  return writer.apply({
    intent: `Change the settings ${Object.keys(patch).join(", ")}`,
    // force takes over a key that another manager, such as kubectl edit,
    // set. The resourceVersion of the read still refuses a stale write.
    force: true,
    apply: [object],
    preconditions: {
      [refKey({ kind: object.kind, ...object.metadata })]:
        live.metadata.resourceVersion ?? "",
    },
  });
}

// An apply cannot create only a missing ConfigMap, so callers hold
// inOrder: a late first create would undo an update.
async function readOrCreate(): Promise<ConfigMap> {
  const live = await read();
  if (live) return live;
  const result = await (
    await getWriter()
  ).apply({
    intent: "Create the settings from the chart values",
    apply: [settingsObject(seedFromEnv(process.env))],
  });
  if (!result.ok) {
    throw new Error(result.errors.map((e) => e.message).join(" "));
  }
  const created = await read();
  if (!created) throw new Error(`The ConfigMap ${SETTINGS_CONFIGMAP} is gone`);
  return created;
}

async function read(): Promise<ConfigMap | undefined> {
  const path = `/api/v1/namespaces/${encodeURIComponent(appNamespace())}/configmaps/${SETTINGS_CONFIGMAP}`;
  try {
    return await getJSON<ConfigMap>(kubeConfig(), path, WRITE_TIMEOUT_MS);
  } catch (error) {
    if (error instanceof KubernetesRequestError && error.statusCode === 404) {
      return undefined;
    }
    throw error;
  }
}

function settingsObject(data: Record<string, string>): ConfigMap {
  return {
    apiVersion: "v1",
    kind: "ConfigMap",
    metadata: { name: SETTINGS_CONFIGMAP, namespace: appNamespace() },
    data,
  };
}

function messageOf(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
