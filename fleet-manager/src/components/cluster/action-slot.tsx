import type { ActionSlotName, ActionSlots, ClusterSlotContext } from "./slots";

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
