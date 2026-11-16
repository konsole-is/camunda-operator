import { ReleaseCreate } from "@/components/setup/releases/release-create";
import { getBundledData } from "@/lib/bundled";
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
    // The server checked the table at start. A table that cannot be read
    // now gives no default image.
    getBundledData().then(
      (bundled) => bundled.imagesTable,
      () => ({ schemaVersion: 1 as const, minors: {} }),
    ),
  ]);
  return (
    <ReleaseCreate
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
