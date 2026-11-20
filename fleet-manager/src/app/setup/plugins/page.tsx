import { RegistrationListLive } from "@/components/setup/plugins/registration-list";
import { loadCatalog, loadRegistrations } from "./load";

// The page reads the cache of the server. A static prerender would call the
// API server at build time.
export const dynamic = "force-dynamic";

export default async function PluginsPage() {
  const [initial, catalog] = await Promise.all([
    loadRegistrations(),
    loadCatalog(),
  ]);
  return (
    <RegistrationListLive
      initial={initial}
      catalog={catalog}
      now={new Date().toISOString()}
    />
  );
}
