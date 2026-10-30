import { CreateClusterFlow } from "@/components/create/create-cluster-flow";
import { loadCreateCluster } from "./load";

export const dynamic = "force-dynamic";

export default async function NewClusterPage() {
  const initial = await loadCreateCluster();
  return (
    <div className="flex flex-col gap-6">
      <h1 className="font-heading text-3xl font-bold tracking-[-0.02em]">
        New cluster
      </h1>
      <CreateClusterFlow initial={initial} />
    </div>
  );
}
