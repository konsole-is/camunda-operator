import { ConnectionListLive } from "@/components/setup/connections/connection-list";
import { loadConnections } from "./load";

// The page reads the cache of the server. A static prerender would call the
// API server at build time.
export const dynamic = "force-dynamic";

export default async function ConnectionsPage() {
  return <ConnectionListLive initial={await loadConnections()} />;
}
