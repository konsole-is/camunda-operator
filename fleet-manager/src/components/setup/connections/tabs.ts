import { tabOf } from "@/lib/tabs";
import type { ContractKind, ContractRef } from "@/lib/usage";
import type { ContractObject } from "./facts";
import type { ConnectionSources } from "./sources";

/** ConnectionTab is the URL value of one tab of the Connections page. */
export type ConnectionTab =
  | "object-storage"
  | "secondary-storage"
  | "database-servers"
  | "databases"
  | "management-identity";

/** CONNECTION_TABS are the tabs of the Connections page, in order, with the contract kind of each. */
export const CONNECTION_TABS: readonly {
  value: ConnectionTab;
  label: string;
  kind: ContractKind;
}[] = [
  {
    value: "object-storage",
    label: "Object storage",
    kind: "ObjectStorageConfig",
  },
  {
    value: "secondary-storage",
    label: "Secondary storage",
    kind: "SecondaryStorageConfig",
  },
  {
    value: "database-servers",
    label: "Database servers",
    kind: "DatabaseServerConfig",
  },
  { value: "databases", label: "Databases", kind: "DatabaseConfig" },
  {
    value: "management-identity",
    label: "Management Identity",
    kind: "ManagementAuthConfig",
  },
];

/**
 * connectionTabOf returns the tab that the tab query parameter names, as
 * tabOf does. The fallback is object-storage.
 */
export function connectionTabOf(
  value: string | string[] | undefined,
): ConnectionTab {
  return tabOf(CONNECTION_TABS, value, "object-storage");
}

/** contractHref returns the path of the page of a contract, which contractRefOf reads back. */
export function contractHref(ref: ContractRef): string {
  const tab = CONNECTION_TABS.find((t) => t.kind === ref.kind)!.value;
  const segments =
    ref.namespace === undefined ? [ref.name] : [ref.namespace, ref.name];
  return `/setup/connections/${tab}/${segments.map(encodeURIComponent).join("/")}`;
}

/**
 * contractRefOf reads a contract from the tab and the decoded segments of
 * its path: a namespace and a name for a namespaced kind, a name alone for
 * ManagementAuthConfig. Another count or an unknown tab gives undefined.
 */
export function contractRefOf(
  tab: string,
  segments: readonly string[],
): ContractRef | undefined {
  const kind = CONNECTION_TABS.find((t) => t.value === tab)?.kind;
  if (kind === undefined) return undefined;
  if (kind === "ManagementAuthConfig") {
    return segments.length === 1 ? { kind, name: segments[0] } : undefined;
  }
  return segments.length === 2
    ? { kind, namespace: segments[0], name: segments[1] }
    : undefined;
}

/** contractsOf returns the contracts of kind in sources. */
export function contractsOf(
  sources: ConnectionSources,
  kind: ContractKind,
): readonly ContractObject[] {
  switch (kind) {
    case "ObjectStorageConfig":
      return sources.objectStorageConfigs;
    case "SecondaryStorageConfig":
      return sources.secondaryStorageConfigs;
    case "DatabaseServerConfig":
      return sources.databaseServerConfigs;
    case "DatabaseConfig":
      return sources.databaseConfigs;
    case "ManagementAuthConfig":
      return sources.managementAuthConfigs;
  }
}
