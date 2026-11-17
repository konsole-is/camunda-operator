import type { V1Namespace } from "@kubernetes/client-node";
import type { ReadError } from "@/lib/k8s/unread";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import { byCodeUnit, environmentOf, isObject } from "@/lib/policy";
import type { KindName } from "@/lib/k8s/kinds";
import { allowedIn, type PresetKind, presetUsers } from "@/lib/usage";
import { ENV_SCOPES } from "@/components/setup/releases/env";
import type { PresetObject } from "./kinds";
import { GUARD_KINDS } from "./rows";
import type { PresetSources } from "./sources";

/**
 * secretsUnread returns each list in errors that secretNamespaces reads for
 * a preset of kind.
 */
export function secretsUnread(
  kind: PresetKind,
  errors: readonly ReadError[],
): KindName[] {
  const kinds: readonly KindName[] = GUARD_KINDS[kind].includes("Namespace")
    ? GUARD_KINDS[kind]
    : [...GUARD_KINDS[kind], "Namespace"];
  return kinds.filter((k) => errors.some((e) => e.kind === k));
}

/**
 * secretNamespaces returns the namespaces whose Secrets the body of preset
 * can name, each once and sorted: the namespaces of the users of the
 * preset, or, when nothing uses it, the namespaces of the Environments that
 * allow it. It is empty when nothing uses the preset and no Environment
 * allows it, and while secretsUnread of errors is not empty.
 */
export function secretNamespaces(
  preset: PresetObject,
  sources: PresetSources,
  errors: readonly ReadError[] = [],
): string[] {
  if (secretsUnread(preset.kind as PresetKind, errors).length > 0) return [];
  const name = preset.metadata.name ?? "";
  // A Resource is plain JSON, so it differs from its model type only in its
  // timestamps, which presetUsers and allowedIn do not read.
  const environments = sources.environments as unknown as Environment[];
  const namespaces = sources.namespaces as unknown as V1Namespace[];
  let names = presetUsers(preset.kind, name, {
    ...sources,
    environments,
    namespaces,
    registrations: sources.registrations as unknown as PluginRegistration[],
  }).map((user) => user.namespace);
  if (names.length === 0) {
    const allowing = new Set(allowedIn(environments, preset.kind, name));
    names = namespaces
      .filter((namespace) => {
        const environment = environmentOf(namespace, environments).environment;
        return allowing.has(environment?.metadata.name ?? "");
      })
      .map((namespace) => namespace.metadata?.name ?? "");
  }
  return [...new Set(names)].sort(byCodeUnit);
}

/** MissingSecret is a Secret key that the body names and that a namespace lacks. env says that an entry of a list of ENV_SCOPES names it. */
export type MissingSecret = {
  name: string;
  key: string;
  namespaces: string[];
  env: boolean;
};

/**
 * missingSecrets returns each Secret key that body names, once and in the
 * order of the body, whose missingIn is not empty: the valueFrom.secretKeyRef
 * of an env entry, and a field whose name ends in SecretRef, such as
 * auth.clientSecretRef.
 */
export function missingSecrets(
  body: unknown,
  missingIn: (name: string, key: string) => readonly string[],
): MissingSecret[] {
  const found = new Map<string, MissingSecret>();
  const visit = (value: unknown, path: readonly string[], env: boolean) => {
    if (Array.isArray(value)) {
      for (const item of value) visit(item, path, env);
      return;
    }
    if (!isObject(value)) return;
    for (const [field, child] of Object.entries(value)) {
      const ref =
        (field === "secretKeyRef" || field.endsWith("SecretRef")) &&
        isObject(child) &&
        typeof child.name === "string" &&
        typeof child.key === "string"
          ? { name: child.name, key: child.key }
          : undefined;
      if (!ref) {
        visit(child, [...path, field], env || isScopeEnv([...path, field]));
        continue;
      }
      const id = `${ref.name}/${ref.key}`;
      const namespaces = [...missingIn(ref.name, ref.key)];
      const seen = found.get(id);
      if (seen) seen.env = seen.env && env;
      else if (namespaces.length > 0)
        found.set(id, { ...ref, namespaces, env });
    }
  };
  visit(body, [], false);
  return [...found.values()];
}

const SCOPE_BLOCKS: readonly string[] = ENV_SCOPES.map((s) => s.scope);

function isScopeEnv(path: readonly string[]): boolean {
  return (
    path.at(-1) === "extraEnv" &&
    (path.length === 1 || (path.length === 2 && SCOPE_BLOCKS.includes(path[0])))
  );
}
