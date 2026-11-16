import { PresetListLive } from "@/components/setup/presets/preset-list";
import { writeMode } from "@/lib/write/writer";
import { applyPresetChange, previewPresetChange } from "./actions";
import { loadPresets } from "./load";

// The page reads the cache of the server. A static prerender would call the
// API server at build time.
export const dynamic = "force-dynamic";

export default async function PresetsPage() {
  const [initial, mode] = await Promise.all([
    loadPresets(),
    writeMode().catch(() => undefined),
  ]);
  return (
    <PresetListLive
      initial={initial}
      mode={mode}
      actions={{
        previewChange: previewPresetChange,
        applyChange: applyPresetChange,
      }}
    />
  );
}
