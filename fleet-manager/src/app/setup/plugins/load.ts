import "server-only";
import type { CatalogRead } from "@/components/setup/plugins/catalog";
import type { RegistrationDetailData } from "@/components/setup/plugins/detail-sources";
import { REGISTRATION_SOURCE_KINDS } from "@/components/setup/plugins/source-kinds";
import type { RegistrationSnapshot } from "@/components/setup/plugins/sources";
import type { Grant } from "@/generated/crds/grant";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import { getBundledData } from "@/lib/bundled";
import { listOrEmpty } from "@/lib/k8s/list-or-empty";
import { get } from "@/lib/k8s/read";
import { loadSnapshot } from "@/lib/k8s/snapshot";
import { REGISTRATION_LABEL } from "@/lib/labels";
import { events } from "@/lib/observe/kube";

/**
 * loadRegistrations reads the first snapshot of the Plugins pages, as
 * loadSnapshot does.
 */
export async function loadRegistrations(): Promise<RegistrationSnapshot> {
  // A Resource is plain JSON. It differs from its model type only in the
  // timestamps of a V1Namespace, which the Plugins pages do not read.
  return (await loadSnapshot(
    REGISTRATION_SOURCE_KINDS,
  )) as unknown as RegistrationSnapshot;
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
 * name, or gives undefined when no registration has that name. The lists
 * of loadRegistrations carry their read errors. A failed read of the
 * Grants gives no Grants.
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
