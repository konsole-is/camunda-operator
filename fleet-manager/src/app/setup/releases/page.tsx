import { ReleaseListLive } from "@/components/setup/releases/release-list";
import { writeMode } from "@/lib/write/writer";
import {
  applyReleaseChange,
  applyReleaseMove,
  previewReleaseChange,
  previewReleaseMove,
} from "./actions";
import { loadReleases } from "./load";

// The page reads the cache of the server. A static prerender would call the
// API server at build time.
export const dynamic = "force-dynamic";

export default async function ReleasesPage() {
  const [initial, mode] = await Promise.all([
    loadReleases(),
    writeMode().catch(() => undefined),
  ]);
  return (
    <ReleaseListLive
      initial={initial}
      mode={mode}
      actions={{
        previewChange: previewReleaseChange,
        applyChange: applyReleaseChange,
        previewMove: previewReleaseMove,
        applyMove: applyReleaseMove,
      }}
    />
  );
}
