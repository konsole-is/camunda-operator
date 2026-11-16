import { refKey } from "@/lib/write/refs";
import type { Change, RefKey } from "@/lib/write/types";
import type { PresetObject } from "./kinds";

/**
 * presetDeleteChange deletes presets in one change. Each delete carries the
 * resourceVersion and the UID that the page read, so it fails with Stale
 * when the preset changed or was created again.
 */
export function presetDeleteChange(presets: readonly PresetObject[]): Change {
  const preconditions: Record<RefKey, string> = {};
  const uids: Record<RefKey, string> = {};
  for (const preset of presets) {
    const key = refKey({ kind: preset.kind, name: preset.metadata.name ?? "" });
    if (preset.metadata.resourceVersion !== undefined)
      preconditions[key] = preset.metadata.resourceVersion;
    if (preset.metadata.uid !== undefined) uids[key] = preset.metadata.uid;
  }
  return {
    intent:
      presets.length === 1
        ? `Delete the preset ${presets[0].metadata.name ?? ""}`
        : `Delete ${presets.length} presets`,
    apply: [],
    delete: presets.map((p) => ({
      apiVersion: p.apiVersion,
      kind: p.kind,
      name: p.metadata.name ?? "",
    })),
    preconditions,
    uids,
  };
}
