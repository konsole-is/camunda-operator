import { notFound } from "next/navigation";
import { ConnectionDetailLive } from "@/components/setup/connections/connection-detail";
import { contractRefOf } from "@/components/setup/connections/tabs";
import { get } from "@/lib/k8s/read";
import { writeMode } from "@/lib/write/writer";
import { applyContractChange, previewContractChange } from "../../actions";
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
  // A failed read gives null: the page shows the read error, because the
  // contract is not known to be missing.
  const [found, sources, mode] = await Promise.all([
    get(target.kind, target.namespace, target.name).catch(() => null),
    loadConnections(),
    writeMode().catch(() => undefined),
  ]);
  if (found === undefined) notFound();
  return (
    <ConnectionDetailLive
      initial={sources}
      target={target}
      mode={mode}
      actions={{
        previewChange: previewContractChange,
        applyChange: applyContractChange,
      }}
    />
  );
}
