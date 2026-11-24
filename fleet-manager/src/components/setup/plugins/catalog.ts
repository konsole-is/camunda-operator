import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { CatalogChart } from "@/startup/bundled-data";

/** CatalogRead is the catalog of the image, or the error of its read. */
export type CatalogRead = { charts: CatalogChart[] } | { error: string };

/**
 * missingCharts returns the charts of the catalog of which no registration
 * names a controller, in catalog order. A chart whose controller runs under
 * another name counts as missing.
 */
export function missingCharts(
  charts: readonly CatalogChart[],
  registrations: readonly PluginRegistration[],
): CatalogChart[] {
  const named = new Set(
    registrations.flatMap((r) => r.spec.controller?.name ?? []),
  );
  return charts.filter((c) => !c.controllers.some((n) => named.has(n)));
}
