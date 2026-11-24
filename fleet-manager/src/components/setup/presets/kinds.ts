import type { Resource } from "@/lib/k8s/kinds";
import type { PresetKind } from "@/lib/usage";
import type { PresetSources } from "./sources";

/** PresetObject is a preset of any of the three kinds. */
export type PresetObject =
  | Resource<"CamundaClusterPreset">
  | Resource<"ElasticsearchClusterPreset">
  | Resource<"DatabaseServerPreset">;

/** PRESET_KINDS holds each preset kind with its URL slug and its label, in the order of the pages. */
export const PRESET_KINDS: readonly {
  kind: PresetKind;
  slug: string;
  label: string;
}[] = [
  { kind: "CamundaClusterPreset", slug: "cluster", label: "Cluster" },
  {
    kind: "ElasticsearchClusterPreset",
    slug: "elasticsearch",
    label: "Elasticsearch",
  },
  { kind: "DatabaseServerPreset", slug: "postgresql", label: "PostgreSQL" },
];

/** PRESETS_HREF is the path of the Presets page. */
export const PRESETS_HREF = "/setup/presets";

/** presetKindOf returns the preset kind of a URL slug, or undefined for an unknown slug. */
export function presetKindOf(slug: string): PresetKind | undefined {
  return PRESET_KINDS.find((k) => k.slug === slug)?.kind;
}

/** presetLabel returns the label of kind. */
export function presetLabel(kind: PresetKind): string {
  return PRESET_KINDS.find((k) => k.kind === kind)?.label ?? kind;
}

/** presetHref returns the path of the page of the preset name of kind. */
export function presetHref(kind: PresetKind, name: string): string {
  const slug = PRESET_KINDS.find((k) => k.kind === kind)?.slug ?? "";
  return `${PRESETS_HREF}/${slug}/${encodeURIComponent(name)}`;
}

/** presetBody returns spec.cluster of a cluster or Elasticsearch preset and spec.server of a PostgreSQL preset. */
export function presetBody(
  preset: PresetObject,
): Record<string, unknown> | undefined {
  const spec = preset.spec as {
    cluster?: Record<string, unknown>;
    server?: Record<string, unknown>;
  };
  return preset.kind === "DatabaseServerPreset" ? spec.server : spec.cluster;
}

/** presetsOf returns the list of sources that holds the presets of kind. */
export function presetsOf(
  kind: PresetKind,
  sources: PresetSources,
): PresetObject[] {
  switch (kind) {
    case "CamundaClusterPreset":
      return sources.clusterPresets;
    case "ElasticsearchClusterPreset":
      return sources.elasticsearchPresets;
    case "DatabaseServerPreset":
      return sources.databasePresets;
  }
}
