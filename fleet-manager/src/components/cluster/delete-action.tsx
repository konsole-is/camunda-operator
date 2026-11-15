import Link from "next/link";
import type { ReactNode } from "react";
import { buttonVariants } from "@/components/ui/button";
import type { ClusterSlotContext } from "./slots";

/**
 * DeleteAction links the header of a cluster to its delete page. A cluster
 * without its claim gets nothing, because the delete removes the claim.
 */
export function DeleteAction({
  context,
}: {
  context: ClusterSlotContext;
}): ReactNode {
  if (!context.claim) return null;
  const { namespace, name } = context.cluster.metadata ?? {};
  return (
    <Link
      href={`/fleet/clusters/${namespace}/${name}/delete`}
      className={buttonVariants({ variant: "outline", size: "sm" })}
    >
      Delete
    </Link>
  );
}
