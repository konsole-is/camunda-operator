import { PROCESS_LABEL } from "@/components/cluster/topology";
import {
  authRules,
  child,
  clusterRules,
  databaseServerRules,
  elasticsearchRules,
  type FieldPath,
  type Rule,
  type Rules,
} from "@/lib/merge";
import type { PresetKind } from "@/lib/usage";

/** RuleLabel says, in the words of the preset pages, how a field of a preset merges with the field of a resource. */
export type RuleLabel =
  | "A resource value replaces it"
  | "Replaces whole"
  | "Merged by key"
  | "Merged by name"
  | "Resource entries follow"
  | "Merged by field"
  | "Replaced with the client";

/** PresetSetting is one field that a preset sets. path is relative to the preset body. */
export type PresetSetting = {
  path: FieldPath;
  value: unknown;
  rule: RuleLabel;
};

/** PresetPart is a titled group of the settings of a preset. */
export type PresetPart = { title: string; settings: PresetSetting[] };

const LABELS: Record<Extract<Rule, string>, RuleLabel> = {
  value: "A resource value replaces it",
  pointer: "A resource value replaces it",
  block: "Replaces whole",
  collection: "Replaces whole",
  byKey: "Merged by key",
  byName: "Merged by name",
  concat: "Resource entries follow",
};

// A layer that sets auth.clientId replaces the whole client of the layers
// before it, so these three fields go together.
const CLIENT_FIELDS = new Set(["clientId", "audience", "clientSecretRef"]);

const PART_TITLES: Record<string, string> = {
  ...PROCESS_LABEL,
  auth: "Authentication",
  backup: "Backup",
};

/** rulesOf returns the merge rules of the resource kind that kind is the preset of. */
export function rulesOf(kind: PresetKind): Rules {
  switch (kind) {
    case "CamundaClusterPreset":
      return clusterRules;
    case "ElasticsearchClusterPreset":
      return elasticsearchRules;
    case "DatabaseServerPreset":
      return databaseServerRules;
  }
}

/**
 * presetParts returns each field that body sets with its merge rule, in rule
 * order. A block of a process, auth, and backup is a part of its own; every
 * other field is in the part "General", which comes first. A field that a
 * preset cannot set is left out, and so is a part without a setting.
 */
export function presetParts(
  kind: PresetKind,
  body: Record<string, unknown> | undefined,
): PresetPart[] {
  if (body === undefined) return [];
  const general: PresetPart = { title: "General", settings: [] };
  const parts = [general];
  for (const [name, rule] of Object.entries(rulesOf(kind))) {
    if (body[name] === undefined) continue;
    if (typeof rule === "object" && !("instance" in rule)) {
      const part = { title: PART_TITLES[name] ?? name, settings: [] };
      walk({ [name]: rule }, body, "", part.settings);
      parts.push(part);
    } else {
      walk({ [name]: rule }, body, "", general.settings);
    }
  }
  return parts.filter((p) => p.settings.length > 0);
}

/**
 * cannotSet returns the fields, relative to the preset body, that each
 * resource sets itself and the CRD of kind refuses in a preset: the
 * instance-bound fields, the version, and for a cluster preset the version
 * and the ServiceAccount name of the Connectors.
 */
export function cannotSet(kind: PresetKind): string[] {
  const paths: string[] = [];
  instancePaths(rulesOf(kind), "", paths);
  paths.push("version");
  if (kind === "CamundaClusterPreset")
    paths.push("connectors.version", "connectors.serviceAccount.name");
  return paths;
}

function instancePaths(rules: Rules, path: FieldPath, out: string[]): void {
  for (const [name, rule] of Object.entries(rules)) {
    if (typeof rule !== "object") continue;
    if ("instance" in rule) out.push(child(path, name));
    else if ("fields" in rule)
      instancePaths(rule.fields, child(path, name), out);
  }
}

function walk(
  rules: Rules,
  body: Record<string, unknown>,
  path: FieldPath,
  out: PresetSetting[],
  labelOf: (name: string, rule: Rule) => RuleLabel | undefined = ruleLabel,
): void {
  for (const [name, rule] of Object.entries(rules)) {
    const value = body[name];
    if (value === undefined) continue;
    const at = child(path, name);
    if (typeof rule === "object" && !("instance" in rule)) {
      if (!isFilled(value)) {
        out.push({ path: at, value, rule: "Merged by field" });
      } else if ("fields" in rule) {
        walk(rule.fields, value, at, out);
      } else {
        walk(authRules, value, at, out, authLabel);
      }
      continue;
    }
    const label = labelOf(name, rule);
    if (label !== undefined) out.push({ path: at, value, rule: label });
  }
}

function ruleLabel(_name: string, rule: Rule): RuleLabel | undefined {
  return typeof rule === "string" ? LABELS[rule] : undefined;
}

function authLabel(name: string, rule: Rule): RuleLabel | undefined {
  return CLIENT_FIELDS.has(name)
    ? "Replaced with the client"
    : ruleLabel(name, rule);
}

function isFilled(value: unknown): value is Record<string, unknown> {
  return (
    typeof value === "object" &&
    value !== null &&
    !Array.isArray(value) &&
    Object.keys(value).length > 0
  );
}
