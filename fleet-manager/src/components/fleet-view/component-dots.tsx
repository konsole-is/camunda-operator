"use client";

import { useId, useRef, useState } from "react";
import { describeReady } from "@/components/fleet/status";
import { StatusDot } from "@/components/fleet/status-dot";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import type { ComponentDot } from "./model";

/**
 * ComponentDots shows one dot for each component of a cluster. On hover or
 * keyboard focus it lists each component with its state and the message of
 * its condition. Escape, or moving the pointer or the focus away, closes the
 * list. Without dots it shows "-".
 */
export function ComponentDots({
  dots,
  defaultOpen = false,
}: {
  dots: ComponentDot[];
  defaultOpen?: boolean;
}) {
  const id = useId();
  const [open, setOpen] = useState(defaultOpen);
  const anchor = useRef<HTMLSpanElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const note = useRef<HTMLDivElement>(null);
  if (dots.length === 0) return <span className="text-muted">-</span>;
  const holds = (node: Node | null) =>
    Boolean(
      node && (anchor.current?.contains(node) || note.current?.contains(node)),
    );
  return (
    <span
      ref={anchor}
      className="inline-flex align-middle"
      onMouseEnter={() => setOpen(true)}
      onMouseLeave={() => {
        if (!holds(document.activeElement)) setOpen(false);
      }}
      onFocus={() => setOpen(true)}
      onBlur={(event) => {
        if (!holds(event.relatedTarget as Node | null)) setOpen(false);
      }}
      onKeyDown={(event) => {
        if (event.key !== "ArrowDown" || event.target !== trigger.current)
          return;
        event.preventDefault();
        note.current?.focus();
      }}
    >
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <button
            ref={trigger}
            type="button"
            aria-label={`Components: ${dots.map(stateOf).join(", ")}`}
            aria-controls={id}
            // PopoverTrigger sets aria-haspopup="dialog", and the list is a
            // note.
            aria-haspopup={undefined}
            onClick={(event) => {
              event.preventDefault();
              setOpen(true);
            }}
            className="flex max-w-56 flex-wrap items-center gap-2 rounded-md p-1 outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            {dots.map((dot) => (
              <StatusDot
                key={dot.id}
                label={dot.label}
                condition={
                  dot.condition && {
                    status: dot.condition.status,
                    reason: dot.condition.reason,
                  }
                }
                off={!dot.used}
              />
            ))}
          </button>
        </PopoverTrigger>
        <PopoverContent
          ref={note}
          id={id}
          role="note"
          tabIndex={0}
          align="start"
          collisionPadding={8}
          onOpenAutoFocus={(event) => event.preventDefault()}
          onCloseAutoFocus={(event) => event.preventDefault()}
          // The list is in a portal at the end of the page. Tab from the
          // list goes on from the dots, not from the end of the page.
          onKeyDown={(event) => {
            if (event.key === "Tab") trigger.current?.focus();
          }}
          onEscapeKeyDown={(event) => {
            event.preventDefault();
            if (note.current?.contains(document.activeElement))
              trigger.current?.focus();
            setOpen(false);
          }}
          // The list sits 4 px from the dots. These strips fill the gap, so
          // the pointer stays on the dots or the list while it crosses.
          className="max-h-(--radix-popover-content-available-height) w-80 max-w-[calc(100vw-2rem)] gap-1.5 overflow-y-auto rounded-md border border-border bg-panel-2 p-3 text-xs break-words text-text-2 shadow-lg ring-0 focus-visible:ring-3 focus-visible:ring-ring/50 before:absolute before:inset-x-0 before:-top-1 before:h-1 after:absolute after:inset-x-0 after:-bottom-1 after:h-1"
        >
          {dots.map((dot) => (
            <span key={dot.id} className="flex items-start gap-2">
              <span aria-hidden className="pt-1">
                <StatusDot
                  label={dot.label}
                  off={!dot.used}
                  condition={dot.condition}
                />
              </span>
              <span>
                {dot.condition?.message && dot.used
                  ? `${stateOf(dot)}. ${dot.condition.message}`
                  : stateOf(dot)}
              </span>
            </span>
          ))}
        </PopoverContent>
      </Popover>
    </span>
  );
}

function stateOf(dot: ComponentDot): string {
  if (!dot.used) return `${dot.label}: Not used`;
  const ready = describeReady(dot.condition);
  return ready.reason
    ? `${dot.label}: ${ready.label} (${ready.reason})`
    : `${dot.label}: ${ready.label}`;
}
