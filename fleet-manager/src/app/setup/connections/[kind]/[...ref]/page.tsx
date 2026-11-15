import { notFound } from "next/navigation";
import { ConnectionDetailLive } from "@/components/setup/connections/connection-detail";
import { contractRefOf } from "@/components/setup/connections/tabs";
import { get } from "@/lib/k8s/read";
import { loadConnections } from "../../load";

// The page reads the cache of the server. A static prerender would call the
// API server at build time.
export const dynamic = "force-dynamic";

export default async function ConnectionPage({
  params,
}: PageProps<"/setup/connections/[kind]/[...ref]">) {
  const { kind, ref } = await params;
  const target = contractRefOf(kind, ref);
  if (!target) notFound();
  const [found, sources] = await Promise.all([
    get(target.kind, target.namespace, target.name),
    loadConnections(),
  ]);
  if (!found) notFound();
  return <ConnectionDetailLive initial={sources} target={target} />;
}
