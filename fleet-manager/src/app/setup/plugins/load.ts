import "server-only";
import type { CatalogRead } from "@/components/setup/plugins/catalog";
import { REGISTRATION_SOURCE_KINDS } from "@/components/setup/plugins/source-kinds";
import type { RegistrationSources } from "@/components/setup/plugins/sources";
import { getBundledData } from "@/lib/bundled";
import { listOrEmpty } from "@/lib/k8s/list-or-empty";

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
