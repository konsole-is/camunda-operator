"use client";

import { type ReactNode, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";

/**
 * InlineConfirm asks for a confirm in place, not in a dialog. Closed, it is
 * one button with triggerLabel. Open, it lists what goes and what stays,
 * with the confirm and Cancel. Cancel closes it and puts the focus back on
 * the trigger.
 */
export function InlineConfirm({
  triggerLabel,
  confirmLabel,
  goes,
  stays = [],
  defaultOpen = false,
  onConfirm,
}: {
  triggerLabel: string;
  confirmLabel: string;
  goes: ReactNode[];
  stays?: ReactNode[];
  defaultOpen?: boolean;
  onConfirm?: () => void;
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
      <Button ref={trigger} variant="outline" onClick={() => setOpen(true)}>
        {triggerLabel}
      </Button>
    );
  }
  return (
    <div
      role="group"
      aria-label={triggerLabel}
      className="flex max-w-xl flex-col gap-3 rounded-md border border-bad/40 bg-bad/5 p-3 text-sm"
    >
      <List title="What goes" items={goes} />
      {stays.length > 0 && <List title="What stays" items={stays} />}
      <div className="flex flex-wrap gap-2">
        <Button variant="destructive" onClick={() => onConfirm?.()}>
          {confirmLabel}
        </Button>
        <Button
          ref={cancel}
          variant="ghost"
          onClick={() => {
            setReturnFocus(true);
            setOpen(false);
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
