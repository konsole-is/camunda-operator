import { PlatformListLive } from "@/components/setup/platform/platform-list";
import { loadPlatformSources } from "./load";

// The page reads the cache of the server. A static prerender would call the
// API server at build time.
export const dynamic = "force-dynamic";

export default async function PlatformPage() {
  return <PlatformListLive initial={await loadPlatformSources()} />;
}
