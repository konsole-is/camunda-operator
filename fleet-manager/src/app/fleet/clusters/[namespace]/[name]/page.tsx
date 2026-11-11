import { notFound } from "next/navigation";
import { writeMode } from "@/lib/write/writer";
import { ClusterPage as ClusterClient } from "./cluster-page";
import { loadClusterPage } from "./load";

export default async function ClusterPage({
  params,
}: PageProps<"/fleet/clusters/[namespace]/[name]">) {
  const { namespace, name } = await params;
  const data = await loadClusterPage(namespace, name);
  if (!data) notFound();
  const mode = await writeMode().catch(() => undefined);
  return (
    <ClusterClient
      initial={data}
      now={new Date().toISOString()}
      writeMode={mode}
    />
  );
}
