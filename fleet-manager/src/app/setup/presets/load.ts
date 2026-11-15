import "server-only";
import { PRESET_SOURCE_KINDS } from "@/components/setup/presets/source-kinds";
import type { PresetSources } from "@/components/setup/presets/sources";
import { listOrEmpty } from "@/lib/k8s/list-or-empty";

/**
 * loadPresets reads the first snapshot of the preset pages from the cache
 * of the server, in every namespace. A failed read of a kind gives no
 * objects of that kind.
 */
export async function loadPresets(): Promise<PresetSources> {
  const lists = await Promise.all(
    Object.entries(PRESET_SOURCE_KINDS).map(
      async ([key, kind]) => [key, await listOrEmpty(kind)] as const,
    ),
  );
  return Object.fromEntries(lists) as PresetSources;
}
