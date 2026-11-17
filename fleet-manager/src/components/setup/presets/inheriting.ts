import type { ConditionLike } from "@/components/fleet/status";
import { clusterFields } from "@/lib/fields";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import type { ReadError } from "@/lib/k8s/unread";
import { CLAIM_LABEL } from "@/lib/labels";
import {
  type FieldSourceMap,
  mergeCluster,
  mergeDatabaseServer,
  mergeElasticsearch,
} from "@/lib/merge";
import { byCodeUnit, environmentNameOf } from "@/lib/policy";
import { type PresetKind, presetResources } from "@/lib/usage";
import type { PresetObject } from "./kinds";
import { cannotSet } from "./merge-rules";
import type { PresetSources } from "./sources";

/** OwnField is a field that a resource sets over its preset. filledBy is the label of the section whose slot fills it. */
export type OwnField = { path: string; filledBy?: string };

/**
 * InheritingRow is one resource that inherits a preset. environment is the
 * display name, else the name, of the Environment of its namespace. href
 * is set for a CamundaCluster only.
 */
export type InheritingRow = {
  kind: "CamundaCluster" | "ElasticsearchCluster" | "DatabaseServer";
  namespace: string;
  name: string;
  environment?: string;
  ownFields: OwnField[];
  ready?: ConditionLike;
  href?: string;
};

type Inheriting =
  | Resource<"CamundaCluster">
  | Resource<"ElasticsearchCluster">
  | Resource<"DatabaseServer">;

/**
 * inheritingRows returns the resources whose spec.presetRef names preset,
 * in the order of presetResources. Own fields are the fields that a
 * resource sets over the preset, and for a CamundaCluster also each field
 * that a slot of a section that its claim selects fills. A field that a
 * preset cannot set is not an own field. Paths start below spec and are
 * sorted by code unit.
 */
export function inheritingRows(
  preset: PresetObject,
  sources: PresetSources,
): InheritingRow[] {
  const kind = preset.kind as PresetKind;
  const blocked = cannotSet(kind);
  return presetResources(kind, preset.metadata.name ?? "", sources).flatMap(
    (user) => {
      const object = listOf(user.kind, sources).find(
        (o) =>
          (o.metadata.namespace ?? "") === user.namespace &&
          o.metadata.name === user.name,
      );
      if (object === undefined) return [];
      const own = ownFields(preset, object, sources);
      const ready = object.status?.conditions?.find((c) => c.type === "Ready");
      const namespace = sources.namespaces.find(
        (n) => n.metadata?.name === user.namespace,
      );
      const envName =
        namespace && environmentNameOf(namespace, sources.environments);
      const environment = sources.environments.find(
        (e) => e.metadata.name === envName,
      );
      return [
        {
          kind: user.kind as InheritingRow["kind"],
          namespace: user.namespace,
          name: user.name,
          ...(environment && {
            environment:
              environment.spec.displayName || (environment.metadata.name ?? ""),
          }),
          ownFields: [...own]
            .filter(([path]) => !blocked.some((b) => isAtOrUnder(path, b)))
            .sort(([a], [b]) => byCodeUnit(a, b))
            .map(([path, filledBy]) => ({
              path,
              ...(filledBy !== undefined && { filledBy }),
            })),
          ...(ready && { ready }),
          ...(user.kind === "CamundaCluster" && {
            href: `/fleet/clusters/${encodeURIComponent(user.namespace)}/${encodeURIComponent(user.name)}`,
          }),
        },
      ];
    },
  );
}

const INHERITING_KIND: Record<PresetKind, InheritingRow["kind"]> = {
  CamundaClusterPreset: "CamundaCluster",
  ElasticsearchClusterPreset: "ElasticsearchCluster",
  DatabaseServerPreset: "DatabaseServer",
};

/**
 * inheritingUnread returns the kind of the resources that inherit a preset
 * of kind when it is in errors (not read) or pending (not current), else
 * an empty list. inheritingRows does not know the users then.
 */
export function inheritingUnread(
  kind: PresetKind,
  errors: readonly ReadError[],
  pending: readonly KindName[],
): KindName[] {
  const inheriting = INHERITING_KIND[kind];
  return pending.includes(inheriting) ||
    errors.some((e) => e.kind === inheriting)
    ? [inheriting]
    : [];
}

function listOf(kind: string, sources: PresetSources): readonly Inheriting[] {
  switch (kind) {
    case "CamundaCluster":
      return sources.clusters;
    case "ElasticsearchCluster":
      return sources.elasticsearchClusters;
    default:
      return sources.databaseServers;
  }
}

// ownFields maps each own field path, below spec, to the label of the
// section that fills it, or undefined.
function ownFields(
  preset: PresetObject,
  object: Inheriting,
  sources: PresetSources,
): Map<string, string | undefined> {
  const own = new Map<string, string | undefined>();
  const add = (source: FieldSourceMap) => {
    for (const [path, layer] of Object.entries(source))
      if (layer === "overrides") own.set(belowSpec(path), undefined);
  };
  if (object.kind === "CamundaCluster") {
    const cluster = object as Resource<"CamundaCluster">;
    const merged = mergeCluster(
      (preset as Resource<"CamundaClusterPreset">).spec,
      undefined,
      cluster.spec,
    );
    add(merged.source);
    const claimName = cluster.metadata.labels?.[CLAIM_LABEL];
    const claim = sources.clusterClaims.find(
      (c) =>
        c.metadata.namespace === cluster.metadata.namespace &&
        c.metadata.name === claimName,
    );
    for (const field of clusterFields({
      cluster,
      merged,
      claim,
      sections: sources.sections,
      registrations: sources.registrations,
    })) {
      if (
        field.state.kind === "filled" &&
        field.target.kind === "CamundaCluster"
      ) {
        const path = belowSpec(field.path);
        for (const key of own.keys())
          if (isAtOrUnder(key, path)) own.delete(key);
        own.set(path, field.state.sectionLabel);
      }
    }
  } else if (object.kind === "ElasticsearchCluster") {
    add(
      mergeElasticsearch(
        (preset as Resource<"ElasticsearchClusterPreset">).spec,
        (object as Resource<"ElasticsearchCluster">).spec,
      ).source,
    );
  } else {
    add(
      mergeDatabaseServer(
        (preset as Resource<"DatabaseServerPreset">).spec,
        (object as Resource<"DatabaseServer">).spec,
      ).source,
    );
  }
  return own;
}

function belowSpec(path: string): string {
  return path.startsWith("spec.") ? path.slice("spec.".length) : path;
}

function isAtOrUnder(path: string, parent: string): boolean {
  return (
    path === parent ||
    path.startsWith(`${parent}.`) ||
    path.startsWith(`${parent}[`)
  );
}
