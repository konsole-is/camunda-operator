"use client";

import type { Resource } from "@/lib/k8s/kinds";
import type { Change, Preview, WriteResult } from "@/lib/write/types";

export type ClusterClaim = Resource<"CamundaClusterClaim">;

/** ClaimActions are the two server actions of a claim write. */
export type ClaimActions = {
  preview(change: Change): Promise<Preview>;
  apply(change: Change): Promise<WriteResult>;
};
