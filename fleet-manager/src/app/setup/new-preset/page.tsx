import { PresetCreateLive } from "@/components/setup/presets/preset-create";
import { writeMode } from "@/lib/write/writer";
import { applyPresetChange, previewPresetChange } from "../presets/actions";
import { loadPresets } from "../presets/load";

// The page reads the cache of the server. A static prerender would call the
// API server at build time.
export const dynamic = "force-dynamic";

export default async function NewPresetPage() {
  const [initial, mode] = await Promise.all([
    loadPresets(),
    writeMode().catch(() => undefined),
  ]);
  return (
    <PresetCreateLive
      initial={initial}
      mode={mode}
      actions={{
        previewChange: previewPresetChange,
        applyChange: applyPresetChange,
      }}
    />
  );
}
