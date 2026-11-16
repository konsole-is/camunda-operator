import type { V1Namespace } from "@kubernetes/client-node";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import { byCodeUnit, environmentOf, isObject } from "@/lib/policy";
import { allowedIn, presetUsers } from "@/lib/usage";
import type { PresetObject } from "./kinds";
import type { PresetSources } from "./sources";

/**
 * secretNamespaces returns the namespaces whose Secrets the body of preset
 * can name, each once and sorted: the namespaces of the users of the
 * preset, or, when nothing uses it, the namespaces of the Environments that
 * allow it. It is empty when nothing uses the preset and no Environment
 * allows it.
 */
export function secretNamespaces(
  preset: PresetObject,
  sources: PresetSources,
): string[] {
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

/** MissingSecret is a Secret key that the body names and that a namespace lacks. env says that an extraEnv entry names it. */
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
  const visit = (value: unknown, env: boolean) => {
    if (Array.isArray(value)) {
      for (const item of value) visit(item, env);
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
        visit(child, env || field === "extraEnv");
        continue;
      }
      const id = `${ref.name}/${ref.key}`;
      const namespaces = [...missingIn(ref.name, ref.key)];
      if (!found.has(id) && namespaces.length > 0)
        found.set(id, { ...ref, namespaces, env });
    }
  };
  visit(body, false);
  return [...found.values()];
}
