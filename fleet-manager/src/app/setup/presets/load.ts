import "server-only";
import { PRESET_SOURCE_KINDS } from "@/components/setup/presets/source-kinds";
import type { PresetSnapshot } from "@/components/setup/presets/sources";
import { loadSnapshot } from "@/lib/k8s/snapshot";

/** loadPresets reads the first snapshot of the preset pages, as loadSnapshot does. */
export function loadPresets(): Promise<PresetSnapshot> {
  return loadSnapshot(PRESET_SOURCE_KINDS);
}
