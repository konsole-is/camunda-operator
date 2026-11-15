import type { CamundaPlatformConfigSpec } from "@/generated/crds/camundaplatformconfig";
import type { ImagesTable } from "@/startup/bundled-data";

/** ImageRef is the default repository and tag of one image. An image that another operator picks has no repository. */
export type ImageRef = { repository?: string; tag: string };

/** ImageSet holds the default image of each image name of the table. */
export type ImageSet = Record<string, ImageRef>;

/** minorOf returns the "<major>.<minor>" of version, or undefined when version has no major.minor. */
export function minorOf(version: string): string | undefined {
  const match = /^(\d+)\.(\d+)(?:\.|$)/.exec(version);
  return match === null ? undefined : `${match[1]}.${match[2]}`;
}

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
  const minor = minorOf(version);
  if (minor === undefined) return undefined;
  const images = table.minors[minor];
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

/** ImageField is a field of spec.images of a CamundaPlatformConfig. */
export type ImageField = keyof NonNullable<CamundaPlatformConfigSpec["images"]>;

/**
 * IMAGE_FIELDS gives the name in the images table of each field of
 * spec.images, in the order of the fields in ImagesSpec. The table also
 * holds images that no field can change, such as elasticsearch.
 */
export const IMAGE_FIELDS: Readonly<Record<ImageField, string>> = {
  camunda: "camunda",
  connectors: "connectors",
  optimize: "optimize",
  identity: "identity",
  console: "console",
  webModelerRestapi: "web-modeler-restapi",
  webModelerWebsockets: "web-modeler-websockets",
  keycloak: "keycloak",
  postgres: "postgres",
};

/** RepositoryDefault is the default repository of an image for the minors from to to, both included. */
export type RepositoryDefault = {
  repository: string;
  from: string;
  to: string;
};

/** DefaultRepositories are the default repositories of each image field for a set of Camunda minors. */
export type DefaultRepositories = {
  /** minors are the minors whose defaults show, sorted by number. */
  minors: string[];
  /** missing are the minors of the input that the table does not hold, sorted by number. */
  missing: string[];
  /** fromUsers is false when the input had no minor and the newest minor of the table stands in. */
  fromUsers: boolean;
  /**
   * fields holds the defaults of each field. Neighbor minors with the same
   * repository share one entry. A field whose image a minor lacks has no
   * entry for that minor.
   */
  fields: Record<ImageField, RepositoryDefault[]>;
};

/**
 * defaultRepositories returns the default repository of each image field
 * for minors, each a "<major>.<minor>". minors can hold duplicates and can
 * come in any order. With no minor, the newest minor of table stands in.
 */
export function defaultRepositories(
  table: ImagesTable,
  minors: readonly string[],
): DefaultRepositories {
  const unique = [...new Set(minors)].sort(byMinor);
  const held = unique.filter((m) => Object.hasOwn(table.minors, m));
  const missing = unique.filter((m) => !Object.hasOwn(table.minors, m));
  const newest = Object.keys(table.minors).sort(byMinor).at(-1);
  const fromUsers = unique.length > 0 || newest === undefined;
  const shown = fromUsers ? held : [newest];
  const fields = Object.fromEntries(
    (Object.keys(IMAGE_FIELDS) as ImageField[]).map((field) => [
      field,
      groups(table, IMAGE_FIELDS[field], shown),
    ]),
  ) as Record<ImageField, RepositoryDefault[]>;
  return { minors: shown, missing, fromUsers, fields };
}

// groups joins the neighbor minors whose entry for image has the same
// repository.
function groups(
  table: ImagesTable,
  image: string,
  minors: readonly string[],
): RepositoryDefault[] {
  const out: RepositoryDefault[] = [];
  let open: RepositoryDefault | undefined;
  for (const minor of minors) {
    const repository = table.minors[minor]?.[image]?.repository;
    if (repository === undefined) {
      open = undefined;
    } else if (open?.repository === repository) {
      open.to = minor;
    } else {
      open = { repository, from: minor, to: minor };
      out.push(open);
    }
  }
  return out;
}

/** byMinor orders two "<major>.<minor>" strings by major, then minor, as numbers. */
export function byMinor(a: string, b: string): number {
  const [aMajor, aMinor] = a.split(".").map(Number);
  const [bMajor, bMinor] = b.split(".").map(Number);
  return aMajor - bMajor || aMinor - bMinor;
}
