import { EnvironmentEditorLive } from "@/components/setup/environments/environment-editor";
import { writeMode } from "@/lib/write/writer";
import { loadEnvironments } from "../load";

// The page reads the cache of the server. A static prerender would call the
// API server at build time.
export const dynamic = "force-dynamic";

export default async function NewEnvironmentPage() {
  const [sources, mode] = await Promise.all([
    loadEnvironments(),
    writeMode().catch(() => "direct" as const),
  ]);
  return <EnvironmentEditorLive initial={sources} mode={mode} />;
}
