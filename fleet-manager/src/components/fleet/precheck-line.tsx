import Link from "next/link";
import { precheckOf } from "@/lib/session";
import { StatusChip } from "./status-chip";

/**
 * PrecheckLine shows why the claim controller refused the values of a
 * claim: the precheck reason of its Ready condition and the message. It
 * shows nothing when precheckOf finds no such condition. A flow without a
 * Claim tab, such as the create progress, shows it after a write. With
 * href, the line ends with the link "Open the Claim tab" to href.
 */
export function PrecheckLine({
  claim,
  href,
}: {
  claim: Parameters<typeof precheckOf>[0];
  href?: string;
}) {
  const condition = precheckOf(claim);
  if (!condition) return null;
  return (
    <p role="status" className="flex flex-wrap items-center gap-2 text-sm">
      <StatusChip condition={condition} />
      <span className="text-text-2">{condition.message}</span>
      {href !== undefined && (
        <Link
          href={href}
          className="relative text-sm text-info underline-offset-4 hit-area hover:underline"
        >
          Open the Claim tab
        </Link>
      )}
    </p>
  );
}
