import { notFound } from "next/navigation";
import { writeMode } from "@/lib/write/writer";
import { DeletePage } from "./delete-page";
import { loadDeletePlanePage } from "./load";

export const dynamic = "force-dynamic";

export default async function DeletePlanePage({
  params,
}: PageProps<"/fleet/planes/[namespace]/[name]/delete">) {
  const { namespace, name } = await params;
  const data = await loadDeletePlanePage(namespace, name);
  if (!data) notFound();
  const mode = await writeMode().catch(() => undefined);
  return <DeletePage initial={data} writeMode={mode} />;
}
