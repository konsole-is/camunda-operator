import { EnvironmentListLive } from "@/components/setup/environments/environment-list";
import { loadEnvironments } from "./load";

// The page reads the cache of the server. A static prerender would call the
// API server at build time.
export const dynamic = "force-dynamic";

export default async function EnvironmentsPage() {
  return <EnvironmentListLive initial={await loadEnvironments()} />;
}
