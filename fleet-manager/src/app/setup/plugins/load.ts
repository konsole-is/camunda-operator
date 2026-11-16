import "server-only";
import type { CatalogRead } from "@/components/setup/plugins/catalog";
import type { RegistrationDetailData } from "@/components/setup/plugins/detail-sources";
import { REGISTRATION_SOURCE_KINDS } from "@/components/setup/plugins/source-kinds";
import type { RegistrationSources } from "@/components/setup/plugins/sources";
import type { Grant } from "@/generated/crds/grant";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import { getBundledData } from "@/lib/bundled";
import { listOrEmpty } from "@/lib/k8s/list-or-empty";
import { get } from "@/lib/k8s/read";
import { REGISTRATION_LABEL } from "@/lib/labels";
import { events } from "@/lib/observe/kube";

/**
 * loadRegistrations reads the first snapshot of the Plugins page from the
 * cache of the server, in every namespace. A failed read of a kind gives no
 * objects of that kind.
 */
export async function loadRegistrations(): Promise<RegistrationSources> {
  const lists = await Promise.all(
    Object.entries(REGISTRATION_SOURCE_KINDS).map(
      async ([key, kind]) => [key, await listOrEmpty(kind)] as const,
    ),
  );
  // A Resource is plain JSON. It differs from its model type only in the
  // timestamps of a V1Namespace, which the Plugins page does not read.
  return Object.fromEntries(lists) as unknown as RegistrationSources;
}

/** loadCatalog gives the charts of the catalog of the image, or the error of its read. */
export async function loadCatalog(): Promise<CatalogRead> {
  return getBundledData().then(
    (bundled) => ({ charts: bundled.catalog.charts }),
    (error: unknown) => ({
      error: error instanceof Error ? error.message : String(error),
    }),
  );
}

/**
 * loadRegistration reads the first snapshot of the page of the registration
 * name, or gives undefined when no registration has that name. A failed
 * read of a list gives no objects of its kind.
 */
export async function loadRegistration(
  name: string,
): Promise<RegistrationDetailData | undefined> {
  const [registration, sources, grants] = await Promise.all([
    get("PluginRegistration", undefined, name),
    loadRegistrations(),
    listOrEmpty("Grant", { labelSelector: `${REGISTRATION_LABEL}=${name}` }),
  ]);
  if (!registration) return undefined;
  const found = await events({
    kind: "PluginRegistration",
    name,
    uid: registration.metadata.uid,
  }).catch(() => []);
  // A Resource is plain JSON. It differs from its model type only in its
  // timestamps, which the page does not read.
  return {
    name,
    registration: registration as unknown as PluginRegistration,
    sources,
    grants: grants as unknown as Grant[],
    events: found,
  };
}
