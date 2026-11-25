import { PlatformCreateLive } from "@/components/setup/platform/platform-create";
import { loadImagesTable } from "@/lib/bundled/images";
import { writeMode } from "@/lib/write/writer";
import {
  applyPlatformConfigChange,
  previewPlatformConfigChange,
} from "../platform/actions";
import { loadPlatformSources } from "../platform/load";

// The page reads the cache of the server. A static prerender would call the
// API server at build time.
export const dynamic = "force-dynamic";

export default async function NewPlatformConfigPage() {
  const [sources, mode, imagesTable] = await Promise.all([
    loadPlatformSources(),
    writeMode().catch(() => undefined),
    loadImagesTable(),
  ]);
  return (
    <PlatformCreateLive
      initial={sources}
      imagesTable={imagesTable}
      mode={mode}
      actions={{
        previewChange: previewPlatformConfigChange,
        applyChange: applyPlatformConfigChange,
      }}
    />
  );
}
