"use client";

import { TriangleAlert } from "lucide-react";
import { useId, useRef, useState } from "react";
import { CHIP_TONE } from "@/components/fleet/status";
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
  const trigger = useRef<HTMLButtonElement>(null);
  if (clusters.length === 0) return null;
  return (
    <span
      className="relative inline-flex align-middle"
      onMouseEnter={() => setOpen(true)}
      onMouseLeave={(event) => {
        if (!event.currentTarget.contains(document.activeElement))
          setOpen(false);
      }}
      onFocus={() => setOpen(true)}
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null))
          setOpen(false);
      }}
      onKeyDown={(event) => {
        if (event.key !== "Escape") return;
        trigger.current?.focus();
        setOpen(false);
      }}
    >
      <button
        ref={trigger}
        type="button"
        aria-expanded={open}
        aria-controls={id}
        className={cn(
          "inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium whitespace-nowrap outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
          CHIP_TONE.warn,
        )}
      >
        <TriangleAlert aria-hidden className="size-3.5" />
        {`${clusters.length} not attached`}
      </button>
      {open && (
        // Padding, not a margin: a gap outside the box ends the hover
        // before the pointer reaches it.
        <span
          className="absolute top-full right-0 z-20 pt-1"
          ref={(box) => {
            if (!box) return;
            const { left } = box.getBoundingClientRect();
            if (left < 0) box.style.right = `${left}px`;
          }}
        >
          <span
            id={id}
            role="note"
            className="flex w-80 max-w-[calc(100vw-2rem)] flex-col gap-2 rounded-md border border-border bg-panel-2 p-3 text-sm font-normal text-text-2 shadow-lg"
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
          </span>
        </span>
      )}
    </span>
  );
}

function why({ reason, message }: NotAttached): string | undefined {
  if (reason && message) return `${reason}: ${message}`;
  return reason || message || undefined;
}
