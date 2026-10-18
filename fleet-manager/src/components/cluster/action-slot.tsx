import type { ActionSlotName, ActionSlots } from "./slots";

/**
 * ActionSlot is the mount point of one action slot: an element with
 * data-action-slot set to name, holding the node of slots for that name, or
 * nothing. For a cluster that is not managed, it renders nothing, also when
 * the slot is filled.
 */
export function ActionSlot({
  name,
  slots,
  managed,
}: {
  name: ActionSlotName;
  slots?: ActionSlots;
  managed: boolean;
}) {
  if (!managed) return null;
  return <div data-action-slot={name}>{slots?.[name]}</div>;
}
