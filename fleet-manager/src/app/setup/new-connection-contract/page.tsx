import { ContractCreateLive } from "@/components/setup/connections/contract-create";
import { connectionTabOf } from "@/components/setup/connections/tabs";
import { writeMode } from "@/lib/write/writer";
import {
  applyContractChange,
  previewContractChange,
} from "../connections/actions";
import { loadConnections } from "../connections/load";

// The page reads the cache of the server. A static prerender would call the
// API server at build time.
export const dynamic = "force-dynamic";

export default async function NewConnectionContractPage({
  searchParams,
}: PageProps<"/setup/new-connection-contract">) {
  const { tab } = await searchParams;
  const [initial, mode] = await Promise.all([
    loadConnections(),
    writeMode().catch(() => undefined),
  ]);
  return (
    <ContractCreateLive
      initial={initial}
      tab={connectionTabOf(tab)}
      mode={mode}
      actions={{
        previewChange: previewContractChange,
        applyChange: applyContractChange,
      }}
    />
  );
}
