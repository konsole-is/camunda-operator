import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { CatalogChart } from "@/startup/bundled-data";

/**
 * missingCharts returns the charts of the catalog that no registration
 * names as its controller, in catalog order. A chart whose controller runs
 * under another name counts as missing.
 */
export function missingCharts(
  charts: readonly CatalogChart[],
  registrations: readonly PluginRegistration[],
): CatalogChart[] {
  const controllers = new Set(
    registrations.flatMap((r) => r.spec.controller?.name ?? []),
  );
  return charts.filter((c) => !controllers.has(c.name));
}
