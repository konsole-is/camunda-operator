import { ReleaseCreateLive } from "@/components/setup/releases/release-create";
import { loadImagesTable } from "@/lib/bundled/images";
import { writeMode } from "@/lib/write/writer";
import {
  applyReleaseChange,
  applyReleaseMove,
  previewReleaseChange,
  previewReleaseMove,
} from "../releases/actions";
import { loadReleases } from "../releases/load";

// The page reads the cache of the server. A static prerender would call the
// API server at build time.
export const dynamic = "force-dynamic";

export default async function NewReleasePage({
  searchParams,
}: PageProps<"/setup/new-release">) {
  const { from } = await searchParams;
  const [sources, mode, imagesTable] = await Promise.all([
    loadReleases(),
    writeMode().catch(() => undefined),
    loadImagesTable(),
  ]);
  return (
    <ReleaseCreateLive
      data={{
        sources,
        imagesTable,
        from: Array.isArray(from) ? from[0] : from,
      }}
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
