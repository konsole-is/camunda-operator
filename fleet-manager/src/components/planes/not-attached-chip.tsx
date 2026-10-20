"use client";

import { TriangleAlert } from "lucide-react";
import {
  type CSSProperties,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";
import { createPortal } from "react-dom";
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
  const anchor = useRef<HTMLSpanElement>(null);
  const panel = useRef<HTMLSpanElement>(null);
  const [place, setPlace] = useState<CSSProperties>({ visibility: "hidden" });
  const inBrowser = useSyncExternalStore(
    noChange,
    () => true,
    () => false,
  );
  useLayoutEffect(() => {
    if (!open || !inBrowser) return;
    const update = () => {
      if (anchor.current && panel.current)
        setPlace(placeNear(anchor.current, panel.current));
    };
    const close = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      if (panel.current?.contains(document.activeElement))
        trigger.current?.focus();
      setOpen(false);
    };
    update();
    window.addEventListener("resize", update);
    window.addEventListener("scroll", update, true);
    window.addEventListener("keydown", close);
    return () => {
      window.removeEventListener("resize", update);
      window.removeEventListener("scroll", update, true);
      window.removeEventListener("keydown", close);
    };
  }, [open, inBrowser]);
  const holds = (node: Node | null) =>
    Boolean(
      node && (anchor.current?.contains(node) || panel.current?.contains(node)),
    );
  if (clusters.length === 0) return null;
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
        if (event.key !== "ArrowDown" || !panel.current) return;
        event.preventDefault();
        panel.current.querySelector<HTMLElement>("[role=note]")?.focus();
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
      {open &&
        inBrowser &&
        // The table box scrolls, so a panel inside it is clipped. Padding,
        // not a margin: a gap outside the box ends the hover before the
        // pointer reaches it.
        createPortal(
          <span
            ref={panel}
            style={place}
            className="fixed z-50 flex flex-col py-1"
          >
            <span
              id={id}
              role="note"
              tabIndex={0}
              className="flex min-h-0 w-80 max-w-[calc(100vw-2rem)] flex-col gap-2 overflow-y-auto rounded-md border border-border bg-panel-2 p-3 text-sm font-normal break-words text-text-2 shadow-lg outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
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
          </span>,
          document.body,
        )}
    </span>
  );
}

function noChange() {
  return () => {};
}

function placeNear(anchor: HTMLElement, panel: HTMLElement): CSSProperties {
  const chip = anchor.getBoundingClientRect();
  const { width, height } = panel.getBoundingClientRect();
  const margin = 8;
  const roomBelow = window.innerHeight - chip.bottom - margin;
  const roomAbove = chip.top - margin;
  const below = height <= roomBelow || roomBelow >= roomAbove;
  const room = below ? roomBelow : roomAbove;
  return {
    top: below ? chip.bottom : chip.top - Math.min(height, room),
    left: Math.max(
      margin,
      Math.min(chip.right - width, window.innerWidth - width - margin),
    ),
    maxHeight: room,
  };
}

function why({ reason, message }: NotAttached): string | undefined {
  if (reason && message) return `${reason}: ${message}`;
  return reason || message || undefined;
}
