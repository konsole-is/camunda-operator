import type { DatabaseConfig } from "@/generated/crds/databaseconfig";
import type { DatabaseServerPreset } from "@/generated/crds/databaseserverpreset";
import type { ElasticsearchClusterPreset } from "@/generated/crds/elasticsearchclusterpreset";
import type { Environment } from "@/generated/crds/environment";
import type { ObjectStorageConfig } from "@/generated/crds/objectstorageconfig";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { SecondaryStorageConfig } from "@/generated/crds/secondarystorageconfig";
import { BUILTIN_CONTROLLER } from "@/lib/labels";
import { byCodeUnit, own } from "./json";
import { resolve } from "./resolve";
import type { OptionView, ParameterRule, PluginSelection } from "./types";

/** BuiltinSource is a kind whose objects a parameter of a built-in registration names. */
export type BuiltinSource =
  | "ElasticsearchClusterPreset"
  | "DatabaseServerPreset"
  | "SecondaryStorageConfig"
  | "ObjectStorageConfig"
  | "DatabaseConfig";

/**
 * BuiltinOption is the entry of one built-in registration. lists maps a
 * parameter to the kind of object it names. hidden are the parameters that
 * the claim controller sets, which no form shows. bucket names the input
 * that gives the snapshot or archive bucket, and the words of its note.
 * storageType is the secondary storage type that the registration makes.
 */
export type BuiltinOption = {
  lists: Readonly<Record<string, BuiltinSource>>;
  hidden: readonly string[];
  bucket?: { input: string; label: string; none: string };
  storageType?: "elasticsearch" | "rdbms";
};

/** BUILTIN_OPTIONS holds the entry of each built-in registration, by registration name. */
export const BUILTIN_OPTIONS: Readonly<Record<string, BuiltinOption>> = {
  elasticsearch: {
    lists: { preset: "ElasticsearchClusterPreset" },
    hidden: ["releaseRef"],
    bucket: {
      input: "objectStorageConfig",
      label: "Snapshots",
      none: "No snapshots",
    },
    storageType: "elasticsearch",
  },
  "database-server": {
    lists: { preset: "DatabaseServerPreset" },
    hidden: ["releaseRef"],
    bucket: {
      input: "objectStorageConfig",
      label: "Archive",
      none: "No archive",
    },
    storageType: "rdbms",
  },
  "existing-secondary-storage": {
    lists: { name: "SecondaryStorageConfig" },
    hidden: [],
  },
  "existing-object-storage": {
    lists: { name: "ObjectStorageConfig" },
    hidden: [],
  },
  "management-database-server": {
    lists: { preset: "DatabaseServerPreset" },
    hidden: ["databases"],
  },
  "existing-database-configs": {
    lists: {
      keycloak: "DatabaseConfig",
      identity: "DatabaseConfig",
      webModeler: "DatabaseConfig",
    },
    hidden: [],
  },
};

/** BuiltinObjects holds the objects of each source kind, in every namespace. A missing kind counts as no objects. */
export type BuiltinObjects = Partial<{
  ElasticsearchClusterPreset: readonly ElasticsearchClusterPreset[];
  DatabaseServerPreset: readonly DatabaseServerPreset[];
  SecondaryStorageConfig: readonly SecondaryStorageConfig[];
  ObjectStorageConfig: readonly ObjectStorageConfig[];
  DatabaseConfig: readonly DatabaseConfig[];
}>;

/**
 * BuiltinContext is what builtinOption reads. namespace is the namespace of
 * the claim. Without it, a parameter that names a config object gets no
 * list, for a page that has no claim namespace, such as the Environment
 * editor.
 */
export type BuiltinContext = {
  environment: Environment | undefined;
  namespace?: string;
  objects: BuiltinObjects;
};

type Fact = { label: string; value: string };

/** builtinEntry returns the entry of registration when BUILTIN_CONTROLLER serves it and BUILTIN_OPTIONS has its name, else undefined. */
export function builtinEntry(
  registration: PluginRegistration,
): BuiltinOption | undefined {
  if (registration.spec.controller?.name !== BUILTIN_CONTROLLER) {
    return undefined;
  }
  return own(BUILTIN_OPTIONS, registration.metadata.name ?? "");
}

/**
 * builtinOption returns option with the data of its built-in entry, and
 * option itself for any other registration. It drops the hidden parameters.
 * A claim-editable preset parameter without an allowed list gets choices:
 * the presets that the Environment lists for its kind
 * (spec.presets.elasticsearchCluster or .databaseServer), in that order,
 * else every preset by name, each with its facts as specs. A claim-editable
 * config parameter without an allowed list gets allowed: the objects of its
 * kind in namespace, by name, with the name as the label. An empty list
 * disables the option: "No <kind> in <namespace>", or for presets
 * "No <kind> is offered". An option that is disabled already keeps its
 * reason. A locked parameter, or one with an allowed list from the
 * Environment, does not change.
 */
export function builtinOption(
  option: OptionView,
  registrations: readonly PluginRegistration[],
  context: BuiltinContext,
): OptionView {
  const registration = registrations.find(
    (r) => r.metadata.name === option.registration,
  );
  const entry = registration && builtinEntry(registration);
  if (!entry) return option;

  let choices = option.choices;
  const empty: string[] = [];
  const parameters = option.parameters
    .filter((rule) => !entry.hidden.includes(rule.name))
    .map((rule): ParameterRule => {
      const source = own(entry.lists, rule.name);
      if (!source || !rule.claimEditable || rule.allowed) return rule;
      if (source === "ElasticsearchClusterPreset") {
        const items = presetItems(
          context.objects.ElasticsearchClusterPreset ?? [],
          context.environment?.spec.presets?.elasticsearchCluster,
          context.environment !== undefined,
          elasticsearchFacts,
        );
        if (items.length === 0) empty.push(`No ${source} is offered`);
        choices = { parameter: rule.name, items };
        return rule;
      }
      if (source === "DatabaseServerPreset") {
        const items = presetItems(
          context.objects.DatabaseServerPreset ?? [],
          context.environment?.spec.presets?.databaseServer,
          context.environment !== undefined,
          databaseServerFacts,
        );
        if (items.length === 0) empty.push(`No ${source} is offered`);
        choices = { parameter: rule.name, items };
        return rule;
      }
      const { namespace } = context;
      if (namespace === undefined) return rule;
      const names = (context.objects[source] ?? [])
        .filter((o) => o.metadata?.namespace === namespace)
        .map((o) => o.metadata?.name ?? "")
        .sort(byCodeUnit);
      if (names.length === 0) empty.push(`No ${source} in ${namespace}`);
      return {
        ...rule,
        allowed: names.map((name) => ({ name, label: name })),
      };
    });

  return {
    ...option,
    parameters,
    ...(choices && { choices }),
    ...(!option.disabledReason &&
      empty.length > 0 && {
        disabledReason: empty[0],
      }),
  };
}

// With an Environment, the cards show the preset list of the Environment for
// the kind.
function presetItems<P extends { metadata: { name?: string } }>(
  presets: readonly P[],
  listed: readonly string[] | undefined,
  withEnvironment: boolean,
  facts: (preset: P) => Fact[],
): { name: string; specs: Fact[] }[] {
  const ordered = withEnvironment
    ? (listed ?? []).flatMap((name) =>
        presets.filter((p) => p.metadata.name === name).slice(0, 1),
      )
    : [...presets].sort((a, b) =>
        byCodeUnit(a.metadata.name ?? "", b.metadata.name ?? ""),
      );
  return ordered.map((preset) => ({
    name: preset.metadata.name ?? "",
    specs: facts(preset),
  }));
}

function elasticsearchFacts(preset: ElasticsearchClusterPreset): Fact[] {
  const { cluster } = preset.spec;
  return factsOf([
    ["Nodes", cluster.replicas],
    ["CPU", cluster.resources?.requests?.cpu],
    ["Memory", cluster.resources?.requests?.memory],
    ["Storage", cluster.storageSize],
  ]);
}

function databaseServerFacts(preset: DatabaseServerPreset): Fact[] {
  const { server } = preset.spec;
  return factsOf([
    ["Instances", server.instances],
    ["CPU", server.resources?.requests?.cpu],
    ["Memory", server.resources?.requests?.memory],
    ["Storage", server.storageSize],
  ]);
}

function factsOf(facts: [string, unknown][]): Fact[] {
  return facts
    .filter(([, value]) => value !== undefined && value !== "")
    .map(([label, value]) => ({ label, value: String(value) }));
}

/**
 * bucketNote returns the bucket line of a built-in registration that has a
 * bucket entry: "<label>: From <section label>" when the cluster or server
 * writes to the bucket of the section that its input reads, "<label>: From
 * the preset" when it writes to the bucket of its preset, else "<label>:
 * <none>". The preset is the one that the selection of registration in
 * selections resolves to under context.environment, read from
 * context.objects. It returns undefined for any other registration, and
 * when the line depends on a preset that is not known.
 */
export function bucketNote(
  registration: PluginRegistration,
  selections: readonly PluginSelection[],
  sections: readonly PluginSection[],
  context: BuiltinContext,
): string | undefined {
  const entry = builtinEntry(registration);
  const bucket = entry?.bucket;
  if (!entry || !bucket) return undefined;
  const from = registration.spec.inputs?.find(
    (i) => i.name === bucket.input,
  )?.from;
  const section = from?.split("/")[0];
  const label =
    sections.find((s) => s.metadata.name === section)?.spec.label ?? section;
  const fromSection = `${bucket.label}: From ${label}`;
  const fromPreset = `${bucket.label}: From the preset`;
  const none = `${bucket.label}: ${bucket.none}`;
  const picked =
    section !== undefined && selections.some((s) => s.section === section);
  const name = presetOf(registration, selections, context.environment);

  if (own(entry.lists, "preset") === "ElasticsearchClusterPreset") {
    // The bucket input replaces the snapshot bucket of the preset.
    if (picked) return fromSection;
    const preset = context.objects.ElasticsearchClusterPreset?.find(
      (p) => p.metadata.name === name,
    );
    if (!preset) return undefined;
    return preset.spec.cluster.snapshotStorageRef ? fromPreset : none;
  }
  const preset = context.objects.DatabaseServerPreset?.find(
    (p) => p.metadata.name === name,
  );
  if (!preset) return undefined;
  // The PostgreSQL adapter takes the retention of the archive from the
  // preset, and does not use the bucket input when the preset has none.
  if (!preset.spec.server.archive) return none;
  return picked ? fromSection : fromPreset;
}

function presetOf(
  registration: PluginRegistration,
  selections: readonly PluginSelection[],
  environment: Environment | undefined,
): unknown {
  const name = registration.metadata.name ?? "";
  const selection = selections.find((s) => s.registration === name);
  const result = resolve({
    environment,
    registrations: [registration],
    selections: [
      {
        section: selection?.section ?? registration.spec.sections[0] ?? "",
        registration: name,
        ...(selection?.parameters && { parameters: selection.parameters }),
      },
    ],
  });
  if (!("value" in result)) return undefined;
  return result.value.requests?.[0]?.parameters?.preset;
}
