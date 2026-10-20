import "server-only";
import { getJSON, kubeConfig, unlessNotFound } from "@/lib/k8s/client";
import { objectPath } from "@/lib/k8s/kinds";
import { inOrder } from "@/lib/serial";
import { messageOf } from "@/lib/write/errors";
import type { KubeObject, WriteResult } from "@/lib/write/types";
import {
  WRITE_TIMEOUT_MS,
  WriteOnlyKindMap,
  getWriter,
  refKey,
} from "@/lib/write/writer";
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
 * FLEET_SETTINGS_* variables first, on the API server also in Git mode.
 * It never changes a ConfigMap that exists. It throws when the API server refuses the read or the create.
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
 * read and its write. It never retries. It writes to the API server, also
 * in Git mode.
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
    directOnly: true,
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
    directOnly: true,
    apply: [settingsObject(seedFromEnv(process.env))],
  });
  if (!result.ok) {
    throw new Error(result.errors.map((e) => e.message).join(" "));
  }
  const created = await read();
  if (!created) throw new Error(`The ConfigMap ${SETTINGS_CONFIGMAP} is gone`);
  return created;
}

function read(): Promise<ConfigMap | undefined> {
  const path = objectPath(
    WriteOnlyKindMap.ConfigMap,
    appNamespace(),
    SETTINGS_CONFIGMAP,
  );
  return unlessNotFound(
    getJSON<ConfigMap>(kubeConfig(), path, WRITE_TIMEOUT_MS),
  );
}

function settingsObject(data: Record<string, string>): ConfigMap {
  return {
    apiVersion: "v1",
    kind: "ConfigMap",
    metadata: { name: SETTINGS_CONFIGMAP, namespace: appNamespace() },
    data,
  };
}
