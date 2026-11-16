import "server-only";
import { PLATFORM_SOURCE_KINDS } from "@/components/setup/platform/source-kinds";
import type { PlatformSources } from "@/components/setup/platform/sources";
import { getBundledData } from "@/lib/bundled";
import { listOrEmpty } from "@/lib/k8s/list-or-empty";
import type { ImagesTable } from "@/startup/bundled-data";

/**
 * loadPlatformSources reads the first snapshot of the Platform pages from
 * the cache of the server, in every namespace. A failed read of a kind
 * gives no objects of that kind.
 */
export async function loadPlatformSources(): Promise<PlatformSources> {
  const lists = await Promise.all(
    Object.entries(PLATFORM_SOURCE_KINDS).map(
      async ([key, kind]) => [key, await listOrEmpty(kind)] as const,
    ),
  );
  // A Resource is plain JSON, so it differs from its model type only in its
  // timestamps, which the Platform pages do not read.
  return Object.fromEntries(lists) as unknown as PlatformSources;
}

/** loadImagesTable is the images table of getBundledData, or an empty table when it cannot be read. */
export async function loadImagesTable(): Promise<ImagesTable> {
  return getBundledData().then(
    (bundled) => bundled.imagesTable,
    () => ({ schemaVersion: 1 as const, minors: {} }),
  );
}
