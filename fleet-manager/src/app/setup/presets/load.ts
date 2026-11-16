import "server-only";
import type { ReadError } from "@/components/planes/planes-table";
import { PRESET_SOURCE_KINDS } from "@/components/setup/presets/source-kinds";
import type { PresetSnapshot } from "@/components/setup/presets/sources";
import { list } from "@/lib/k8s/read";

/**
 * loadPresets reads the first snapshot of the preset pages from the cache
 * of the server, in every namespace. A failed read of a kind gives no
 * objects of that kind and an entry in readErrors.
 */
export async function loadPresets(): Promise<PresetSnapshot> {
  const readErrors: ReadError[] = [];
  const lists = await Promise.all(
    Object.entries(PRESET_SOURCE_KINDS).map(async ([key, kind]) => {
      const items = await list(kind).catch((err: unknown) => {
        readErrors.push({
          kind,
          message: err instanceof Error ? err.message : String(err),
        });
        return [];
      });
      return [key, items] as const;
    }),
  );
  return { ...Object.fromEntries(lists), readErrors } as PresetSnapshot;
}
