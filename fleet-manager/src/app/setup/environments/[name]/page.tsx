import { notFound } from "next/navigation";
import { EnvironmentEditorLive } from "@/components/setup/environments/environment-editor";
import { get } from "@/lib/k8s/read";
import { writeMode } from "@/lib/write/writer";
import { loadEnvironments } from "../load";

// The page reads the cache of the server. A static prerender would call the
// API server at build time.
export const dynamic = "force-dynamic";

export default async function EnvironmentPage({
  params,
}: PageProps<"/setup/environments/[name]">) {
  const { name } = await params;
  const [found, sources, mode] = await Promise.all([
    get("Environment", undefined, name),
    loadEnvironments(),
    writeMode().catch(() => "direct" as const),
  ]);
  if (!found) notFound();
  return <EnvironmentEditorLive initial={sources} name={name} mode={mode} />;
}
