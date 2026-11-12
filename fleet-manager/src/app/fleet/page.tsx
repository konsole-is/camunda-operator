import { FleetViewLive } from "@/components/fleet-view/fleet-view";
import { loadFleetPage } from "./load";

// The page reads the cache of the server. A static prerender would call the
// API server at build time.
export const dynamic = "force-dynamic";

export default async function FleetPage() {
  return (
    <FleetViewLive
      initial={await loadFleetPage()}
      now={new Date().toISOString()}
    />
  );
}
