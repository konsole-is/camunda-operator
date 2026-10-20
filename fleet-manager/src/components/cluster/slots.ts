import type { ReactNode } from "react";

/** ACTION_SLOTS names the places where the write flows mount their actions. */
export const ACTION_SLOTS = [
  "cluster.header.actions",
  "cluster.topology.actions",
  "cluster.backups.actions",
  "cluster.claim.editor",
  "component.settings.editor",
] as const;

export type ActionSlotName = (typeof ACTION_SLOTS)[number];

/** ActionSlots holds the node of each filled slot. */
export type ActionSlots = Partial<Record<ActionSlotName, ReactNode>>;
