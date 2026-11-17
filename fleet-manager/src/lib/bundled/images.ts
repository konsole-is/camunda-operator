import "server-only";
import type { ImagesTable } from "@/startup/bundled-data";
import { getBundledData } from "./index";

/**
 * loadImagesTable is the images table of getBundledData, or an empty table
 * when it cannot be read. The server checked the table at start, so a page
 * that gets the empty table prefills no version and no default image.
 */
export function loadImagesTable(): Promise<ImagesTable> {
  return getBundledData().then(
    (bundled) => bundled.imagesTable,
    () => ({ schemaVersion: 1 as const, minors: {} }),
  );
}
