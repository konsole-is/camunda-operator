import { precheckOf } from "@/lib/session";
import { StatusChip } from "./status-chip";

/**
 * PrecheckLine shows why the claim controller refused the values of a
 * claim: the precheck reason of its Ready condition and the message. It
 * shows nothing when precheckOf finds no such condition. A flow without a
 * Claim tab, such as the create progress, shows it after a write.
 */
export function PrecheckLine({
  claim,
}: {
  claim: Parameters<typeof precheckOf>[0];
}) {
  const condition = precheckOf(claim);
  if (!condition) return null;
  return (
    <p role="status" className="flex flex-wrap items-center gap-2 text-sm">
      <StatusChip condition={condition} />
      <span className="text-text-2">{condition.message}</span>
    </p>
  );
}
