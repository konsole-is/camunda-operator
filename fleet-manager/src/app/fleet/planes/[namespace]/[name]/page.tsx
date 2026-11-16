import { notFound } from "next/navigation";
import { PlaneView } from "@/components/planes/plane-view";
import { writeMode } from "@/lib/write/writer";
import { loadPlanePage } from "./load";

export const dynamic = "force-dynamic";

export default async function PlanePage({
  params,
}: PageProps<"/fleet/planes/[namespace]/[name]">) {
  const { namespace, name } = await params;
  const data = await loadPlanePage(namespace, name);
  if (!data) notFound();
  const mode = await writeMode().catch(() => undefined);
  return <PlaneView data={data} writeMode={mode} />;
}
