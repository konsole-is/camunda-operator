import { deleteChangeOf } from "@/lib/write/delete";
import type { Change } from "@/lib/write/types";
import type { PresetObject } from "./kinds";

/** presetDeleteChange deletes presets in one change, as deleteChangeOf does. */
export function presetDeleteChange(presets: readonly PresetObject[]): Change {
  return deleteChangeOf(
    presets,
    presets.length === 1
      ? `Delete the preset ${presets[0].metadata.name ?? ""}`
      : `Delete ${presets.length} presets`,
  );
}
