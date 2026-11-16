import { notFound } from "next/navigation";
import { writeMode } from "@/lib/write/writer";
import { DeletePage } from "./delete-page";
import { loadDeleteClusterPage } from "./load";

export default async function DeleteClusterPage({
  params,
}: PageProps<"/fleet/clusters/[namespace]/[name]/delete">) {
  const { namespace, name } = await params;
  const data = await loadDeleteClusterPage(namespace, name);
  if (!data) notFound();
  const mode = await writeMode().catch(() => undefined);
  return <DeletePage initial={data} writeMode={mode} />;
}
