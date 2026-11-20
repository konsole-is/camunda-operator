"use client";

import { ChevronRight } from "lucide-react";
import {
  Fragment,
  type ReactNode,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
} from "react";
import { cn } from "@/lib/utils";

/**
 * DisclosureCount is one pill on the row of a DisclosureGroup, with its
 * visible text. label, when given, is what a screen reader reads in place
 * of text, so a pill whose text is a bare number gets a label that says
 * what it counts. A bad pill has the color of an error, so its text must
 * say the meaning too, as "1 error" does.
 */
export type DisclosureCount = { text: string; label?: string; bad?: boolean };

/**
 * DisclosureGroup is a bordered group with a row that opens and closes its
 * body: a chevron, label, and a pill for each of counts. The body stays in
 * the page while it is hidden. It holds no state: a click on the row calls
 * onToggle with the other state, and a match of the find function of the
 * browser in the closed body calls it with true. The caller must open the
 * group on true, or the body shows under a closed row. role and name set
 * the role and the accessible name of the group, a group named label by
 * default. With heading, the row is in a heading of level 3 that label
 * alone names.
 */
export function DisclosureGroup({
  label,
  counts,
  open,
  onToggle,
  heading = false,
  role = "group",
  name = label,
  children,
}: {
  label: string;
  counts: readonly DisclosureCount[];
  open: boolean;
  onToggle: (open: boolean) => void;
  heading?: boolean;
  role?: "group" | "region";
  name?: string;
  children: ReactNode;
}) {
  const id = useId();
  const body = useRef<HTMLDivElement>(null);
  const toggle = useRef(onToggle);
  useLayoutEffect(() => {
    toggle.current = onToggle;
  });
  // React sets hidden only as a boolean. With "until-found", the find
  // function of the browser matches text in the closed body and fires
  // beforematch on it.
  useLayoutEffect(() => {
    if (!open) body.current?.setAttribute("hidden", "until-found");
  }, [open]);
  useEffect(() => {
    const element = body.current;
    // beforematch bubbles from a closed element inside an open body.
    const found = (event: Event) => {
      if (event.target === element) toggle.current(true);
    };
    element?.addEventListener("beforematch", found);
    return () => element?.removeEventListener("beforematch", found);
  }, []);
  const row = (
    <button
      type="button"
      aria-expanded={open}
      aria-controls={`${id}-body`}
      className="flex min-h-10 w-full items-center gap-2 px-4 py-2.5 text-left text-sm font-medium"
      onClick={() => onToggle(!open)}
    >
      <ChevronRight
        aria-hidden
        className={cn(
          "size-4 shrink-0 text-muted transition-transform",
          open && "rotate-90",
        )}
      />
      <span id={`${id}-label`}>{label}</span>
      {counts.map((count, i) => (
        <Fragment key={i}>
          {" "}
          <span
            data-count
            className={cn(
              "rounded-full px-2 py-0.5 text-xs",
              count.bad ? "bg-bad/10 text-bad" : "bg-panel-2 text-muted",
            )}
          >
            {count.label === undefined ? (
              count.text
            ) : (
              <>
                <span aria-hidden>{count.text}</span>
                <span className="sr-only">{count.label}</span>
              </>
            )}
          </span>
        </Fragment>
      ))}
    </button>
  );
  return (
    <div
      role={role}
      aria-label={name}
      className="rounded-md border border-border"
    >
      {heading ? <h3 aria-labelledby={`${id}-label`}>{row}</h3> : row}
      {/* A closed body that the find function can open keeps its own box,
          so the border and the padding are on the inner element. */}
      <div ref={body} id={`${id}-body`} hidden={!open}>
        <div className="flex flex-col gap-4 border-t border-divider px-4 py-4">
          {children}
        </div>
      </div>
    </div>
  );
}
