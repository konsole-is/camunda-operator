"use client";

import { type ReactNode, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

/**
 * InlineConfirm asks for a confirm in place, not in a dialog. Closed, it is
 * one button with triggerLabel. Open, it lists what goes and what stays,
 * then children, with the confirm and Cancel. An empty list does not show.
 * The trigger opens it and calls onOpen. The confirm is off while
 * confirmDisabled. Cancel closes it, calls
 * onCancel, and puts the focus back on the trigger. danger, true by
 * default, marks the confirm as destructive.
 */
export function InlineConfirm({
  triggerLabel,
  confirmLabel,
  goes,
  stays = [],
  defaultOpen = false,
  danger = true,
  confirmDisabled = false,
  children,
  onConfirm,
  onCancel,
  onOpen,
}: {
  triggerLabel: string;
  confirmLabel: string;
  goes: ReactNode[];
  stays?: ReactNode[];
  defaultOpen?: boolean;
  danger?: boolean;
  confirmDisabled?: boolean;
  children?: ReactNode;
  onConfirm?: () => void;
  onCancel?: () => void;
  onOpen?: () => void;
}) {
  const [open, setOpen] = useState(defaultOpen);
  const trigger = useRef<HTMLButtonElement>(null);
  const cancel = useRef<HTMLButtonElement>(null);
  const [returnFocus, setReturnFocus] = useState(false);

  useEffect(() => {
    if (open) cancel.current?.focus();
    else if (returnFocus) trigger.current?.focus();
  }, [open, returnFocus]);

  if (!open) {
    return (
      <Button
        ref={trigger}
        variant="outline"
        onClick={() => {
          setOpen(true);
          onOpen?.();
        }}
      >
        {triggerLabel}
      </Button>
    );
  }
  return (
    <div
      role="group"
      aria-label={triggerLabel}
      className={cn(
        "flex max-w-xl flex-col gap-3 rounded-md border p-3 text-sm",
        danger ? "border-bad/40 bg-bad/5" : "border-border bg-panel",
      )}
    >
      {goes.length > 0 && <List title="What goes" items={goes} />}
      {stays.length > 0 && <List title="What stays" items={stays} />}
      {children}
      <div className="flex flex-wrap gap-2">
        <Button
          variant={danger ? "destructive" : "default"}
          disabled={confirmDisabled}
          onClick={() => onConfirm?.()}
        >
          {confirmLabel}
        </Button>
        <Button
          ref={cancel}
          variant="ghost"
          onClick={() => {
            setReturnFocus(true);
            setOpen(false);
            onCancel?.();
          }}
        >
          Cancel
        </Button>
      </div>
    </div>
  );
}

function List({ title, items }: { title: string; items: ReactNode[] }) {
  return (
    <div>
      <p className="mb-1 font-medium text-text">{title}</p>
      <ul className="flex list-disc flex-col gap-0.5 pl-5 text-text-2">
        {items.map((item, i) => (
          <li key={i}>{item}</li>
        ))}
      </ul>
    </div>
  );
}
