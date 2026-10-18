import { notFound } from "next/navigation";
import { ClusterView } from "@/components/cluster/cluster-view";
import { clusterTabOf } from "@/components/cluster/tabs";
import { loadClusterPage } from "./load";

export default async function ClusterPage({
  params,
  searchParams,
}: PageProps<"/fleet/clusters/[namespace]/[name]">) {
  const { namespace, name } = await params;
  const { tab } = await searchParams;
  const data = await loadClusterPage(namespace, name);
  if (!data) notFound();
  return (
    <ClusterView
      initial={data}
      tab={clusterTabOf(tab)}
      now={new Date().toISOString()}
    />
  );
}
