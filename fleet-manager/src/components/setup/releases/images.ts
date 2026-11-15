import type { CamundaReleaseSpec } from "@/generated/crds/camundarelease";
import { defaultImages } from "@/lib/merge";
import type { ImagesTable } from "@/startup/bundled-data";

/** ReleaseImage is an image that the resources of a release run. */
export type ReleaseImage =
  "camunda" | "optimize" | "connectors" | "elasticsearch" | "postgres";

/**
 * ReleaseImageRow is one image of a release. pinField is the field of
 * spec.images that pins it, set only for the images that a release can pin.
 * repository and tag give the default for the version: repository is unset
 * for an image that another operator picks, and tag is unset for connectors
 * when the release sets no connectors version.
 */
export type ReleaseImageRow = {
  image: ReleaseImage;
  label: string;
  pinField?: "camunda" | "connectors";
  repository?: string;
  tag?: string;
};

const IMAGES: readonly { image: ReleaseImage; label: string }[] = [
  { image: "camunda", label: "Camunda" },
  { image: "optimize", label: "Optimize" },
  { image: "connectors", label: "Connectors" },
  { image: "elasticsearch", label: "Elasticsearch" },
  { image: "postgres", label: "PostgreSQL" },
];

/**
 * releaseImages returns the images of a release with spec, in a fixed
 * order, from the entry of table for the Camunda minor of spec.version.
 * The tag of camunda and optimize is spec.version, and the tag of
 * connectors is spec.connectors.version, because the release sets the
 * version of these images. The other tags are those of the table. It
 * leaves out an image that the table lacks, and returns undefined when the
 * table does not hold the minor.
 */
export function releaseImages(
  spec: CamundaReleaseSpec,
  table: ImagesTable,
): ReleaseImageRow[] | undefined {
  const defaults = defaultImages(table, spec.version);
  if (!defaults) return undefined;
  return IMAGES.flatMap(({ image, label }) => {
    const entry = defaults[image];
    if (!entry) return [];
    const tag =
      image === "camunda" || image === "optimize"
        ? spec.version
        : image === "connectors"
          ? spec.connectors?.version
          : entry.tag;
    const pinField =
      image === "camunda" || image === "connectors" ? image : undefined;
    return [
      {
        image,
        label,
        ...(pinField && { pinField }),
        ...(entry.repository !== undefined && {
          repository: entry.repository,
        }),
        ...(tag && { tag }),
      },
    ];
  });
}

/** minorOf returns the major.minor of version, or undefined when version has none. */
export function minorOf(version: string): string | undefined {
  const match = /^(\d+)\.(\d+)(?:\.|$)/.exec(version);
  return match ? `${match[1]}.${match[2]}` : undefined;
}
