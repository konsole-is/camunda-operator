"use client";

import { CircleHelp } from "lucide-react";
import { useId, useRef, useState } from "react";
import type { FieldDoc } from "@/generated/fielddocs";
import { apiReferenceUrl } from "@/lib/fielddocs/link";

/**
 * FieldHelp is the (?) next to a form label. On hover or keyboard focus it
 * shows the description of the field and a link to the API reference of the
 * running operator version. Escape, or moving the pointer or the focus away,
 * closes it. The label stays the words of the page: the description adds
 * detail. Without a doc it renders nothing.
 */
export function FieldHelp({
  label,
  doc,
  operatorVersion,
  defaultOpen = false,
}: {
  label: string;
  doc: FieldDoc | undefined;
  operatorVersion: string;
  defaultOpen?: boolean;
}) {
  const id = useId();
  const [open, setOpen] = useState(defaultOpen);
  const trigger = useRef<HTMLButtonElement>(null);
  if (!doc?.description) return null;
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
        aria-label={`About ${label}`}
        aria-expanded={open}
        aria-controls={id}
        className="inline-flex size-6 items-center justify-center rounded-full text-muted outline-none hover:text-text focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        <CircleHelp aria-hidden className="size-4" />
      </button>
      {open && (
        // Padding, not a margin: a gap outside the box ends the hover
        // before the pointer reaches the link.
        <span className="absolute top-full left-0 z-20 pt-1">
          <span
            id={id}
            role="note"
            className="flex w-72 max-w-[calc(100vw-2rem)] flex-col gap-2 rounded-md border border-border bg-panel-2 p-3 text-sm font-normal text-text-2 shadow-lg"
          >
            {paragraphs(doc.description).map((text, i) => (
              <span key={i}>{text}</span>
            ))}
            <a
              href={apiReferenceUrl(operatorVersion, doc.anchor)}
              target="_blank"
              rel="noreferrer"
              className="text-info underline-offset-4 hover:underline"
            >
              API reference
            </a>
          </span>
        </span>
      )}
    </span>
  );
}

/** The CRD descriptions wrap their lines. A blank line starts a paragraph. */
function paragraphs(description: string): string[] {
  return description
    .split(/\n{2,}/)
    .map((p) => p.replace(/\s*\n\s*/g, " ").trim());
}
