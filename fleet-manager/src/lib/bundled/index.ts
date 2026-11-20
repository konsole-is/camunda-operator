import "server-only";
import { type BundledData, loadBundledData } from "@/startup/bundled-data";

let cached: Promise<BundledData> | undefined;

/**
 * getBundledData returns the images table and the catalog that the image
 * ships. It reads them once and keeps the result for the life of the
 * process. A failed read is not kept: the next call reads again.
 */
export function getBundledData(): Promise<BundledData> {
  if (cached) return cached;
  const read = loadBundledData();
  cached = read;
  read.catch(() => {
    if (cached === read) cached = undefined;
  });
  return read;
}
