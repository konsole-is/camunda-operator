import "server-only";
import { CONNECTION_SOURCE_KINDS } from "@/components/setup/connections/source-kinds";
import type { ConnectionPageData } from "@/components/setup/connections/sources";
import { loadSnapshot } from "@/lib/k8s/snapshot";

/**
 * loadConnections reads the first snapshot of the Connections pages, as
 * loadSnapshot does.
 */
export async function loadConnections(): Promise<ConnectionPageData> {
  // A Resource is plain JSON, so it differs from its model type only in its
  // timestamps, which the Connections pages do not read.
  return (await loadSnapshot(
    CONNECTION_SOURCE_KINDS,
  )) as unknown as ConnectionPageData;
}
