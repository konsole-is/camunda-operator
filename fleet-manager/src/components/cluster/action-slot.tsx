import type { ReactNode } from "react";
import type { BackupRow } from "./backups";
import type {
  ActionSlotName,
  ActionSlots,
  ClusterSlotContext,
  RowSlots,
} from "./slots";

/**
 * ActionSlot is the mount point of one action slot: an element with
 * data-action-slot set to name, holding the fill of slots for that name, or
 * nothing. A render function gets context, and renders nothing without it.
 * For a cluster that is not managed, it renders nothing, also when the slot
 * is filled.
 */
export function ActionSlot({
  name,
  slots,
  managed,
  context,
}: {
  name: ActionSlotName;
  slots?: ActionSlots;
  managed: boolean;
  context?: ClusterSlotContext;
}) {
  if (!managed) return null;
  const fill = slots?.[name];
  return (
    <div data-action-slot={name}>
      {typeof fill === "function" ? (context ? fill(context) : null) : fill}
    </div>
  );
}

/**
 * rowActionsOf returns the actions of each backup row: the node of the
 * cluster.backups.row slot in an element with data-action-slot set. It
 * returns undefined for a cluster that is not managed or a slot that is
 * not filled.
 */
export function rowActionsOf(
  slots: RowSlots | undefined,
  managed: boolean,
  context: ClusterSlotContext,
): ((row: BackupRow) => ReactNode) | undefined {
  const fill = slots?.["cluster.backups.row"];
  if (!managed || fill === undefined) return undefined;
  return function rowActions(row: BackupRow) {
    return (
      <div data-action-slot="cluster.backups.row">{fill(context, row)}</div>
    );
  };
}
