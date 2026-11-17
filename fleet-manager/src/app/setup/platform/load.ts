import "server-only";
import { PLATFORM_SOURCE_KINDS } from "@/components/setup/platform/source-kinds";
import type { PlatformSnapshot } from "@/components/setup/platform/sources";
import { getBundledData } from "@/lib/bundled";
import { loadSnapshot } from "@/lib/k8s/snapshot";
import type { ImagesTable } from "@/startup/bundled-data";

/**
 * loadPlatformSources reads the first snapshot of the Platform pages, as
 * loadSnapshot does.
 */
export async function loadPlatformSources(): Promise<PlatformSnapshot> {
  // A Resource is plain JSON, so it differs from its model type only in its
  // timestamps, which the Platform pages do not read.
  return (await loadSnapshot(
    PLATFORM_SOURCE_KINDS,
  )) as unknown as PlatformSnapshot;
}

/** loadImagesTable is the images table of getBundledData, or an empty table when it cannot be read. */
export async function loadImagesTable(): Promise<ImagesTable> {
  return getBundledData().then(
    (bundled) => bundled.imagesTable,
    () => ({ schemaVersion: 1 as const, minors: {} }),
  );
}
