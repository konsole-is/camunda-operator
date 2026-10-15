import type { ImagesTable } from "@/startup/bundled-data";

/** ImageRef is the default repository and tag of one image. An image that another operator picks has no repository. */
export type ImageRef = { repository?: string; tag: string };

/** ImageSet holds the default image of each image name of the table. */
export type ImageSet = Record<string, ImageRef>;

/**
 * defaultImages returns the default image of each image of the Camunda minor
 * of version, from table. Each image has its own version line, so the tag is
 * the version of the table, after the tag prefix of the image. It gives the
 * tested version of each image, not the version that a cluster runs. It
 * returns undefined when version has no major.minor or the table does not
 * hold its minor.
 */
export function defaultImages(
  table: ImagesTable,
  version: string,
): ImageSet | undefined {
  const match = /^(\d+)\.(\d+)(?:\.|$)/.exec(version);
  if (match === null) return undefined;
  const images = table.minors[`${match[1]}.${match[2]}`];
  if (images === undefined) return undefined;
  return Object.fromEntries(
    Object.entries(images).map(([name, entry]) => [
      name,
      {
        ...(entry.repository === undefined
          ? {}
          : { repository: entry.repository }),
        tag: (entry.tagPrefix ?? "") + entry.version,
      },
    ]),
  );
}
