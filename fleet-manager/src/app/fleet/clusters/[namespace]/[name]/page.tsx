import { notFound } from "next/navigation";
import { ClusterView } from "@/components/cluster/cluster-view";
import { loadClusterPage } from "./load";

export default async function ClusterPage({
  params,
}: PageProps<"/fleet/clusters/[namespace]/[name]">) {
  const { namespace, name } = await params;
  const data = await loadClusterPage(namespace, name);
  if (!data) notFound();
  return <ClusterView initial={data} now={new Date().toISOString()} />;
}
