"use client";

import { TriangleAlert } from "lucide-react";
import { useId, useRef, useState } from "react";
import { CHIP_TONE } from "@/components/fleet/status";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";
import type { NotAttached } from "./plane";

/**
 * NotAttachedChip is the warning "<n> not attached" of a plane. On hover or
 * keyboard focus it lists each cluster with its reason and message. Escape,
 * or moving the pointer or the focus away, closes it. Without clusters it
 * renders nothing.
 */
export function NotAttachedChip({
  clusters,
  defaultOpen = false,
}: {
  clusters: NotAttached[];
  defaultOpen?: boolean;
}) {
  const id = useId();
  const [open, setOpen] = useState(defaultOpen);
  const anchor = useRef<HTMLSpanElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const note = useRef<HTMLDivElement>(null);
  if (clusters.length === 0) return null;
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
            aria-controls={id}
            onClick={(event) => {
              event.preventDefault();
              setOpen(true);
            }}
            className={cn(
              "inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium whitespace-nowrap outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
              CHIP_TONE.warn,
            )}
          >
            <TriangleAlert aria-hidden className="size-3.5" />
            {`${clusters.length} not attached`}
          </button>
        </PopoverTrigger>
        <PopoverContent
          ref={note}
          id={id}
          role="note"
          tabIndex={0}
          align="end"
          collisionPadding={8}
          onOpenAutoFocus={(event) => event.preventDefault()}
          onCloseAutoFocus={(event) => event.preventDefault()}
          onEscapeKeyDown={(event) => {
            event.preventDefault();
            if (note.current?.contains(document.activeElement))
              trigger.current?.focus();
            setOpen(false);
          }}
          // The popover sits 4 px from the chip. These strips fill the gap,
          // so the pointer stays on the chip or the list while it crosses.
          className="max-h-(--radix-popover-content-available-height) w-80 max-w-[calc(100vw-2rem)] gap-2 overflow-y-auto rounded-md border border-border bg-panel-2 p-3 font-normal break-words text-text-2 shadow-lg ring-0 before:absolute before:inset-x-0 before:-top-1 before:h-1 after:absolute after:inset-x-0 after:-bottom-1 after:h-1 focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          {clusters.map((cluster) => (
            <span
              key={`${cluster.namespace}/${cluster.name}`}
              className="flex flex-col"
            >
              <span className="font-medium text-text">{`${cluster.namespace}/${cluster.name}`}</span>
              {why(cluster) && <span>{why(cluster)}</span>}
            </span>
          ))}
        </PopoverContent>
      </Popover>
    </span>
  );
}

function why({ reason, message }: NotAttached): string | undefined {
  if (reason && message) return `${reason}: ${message}`;
  return reason || message || undefined;
}
