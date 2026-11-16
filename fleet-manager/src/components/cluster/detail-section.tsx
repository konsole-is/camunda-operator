import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * DetailSection is one titled box of a component detail. note is a short
 * fact after the title, such as a count. With region, the box is a region
 * named by its title.
 */
export function DetailSection({
  title,
  note,
  region = false,
  className,
  children,
}: {
  title: string;
  note?: string;
  region?: boolean;
  className?: string;
  children: ReactNode;
}) {
  const Box = region ? "section" : "div";
  return (
    <Box
      aria-label={region ? title : undefined}
      className={cn(
        "flex min-w-0 flex-col gap-2 rounded-md border border-border p-3",
        className,
      )}
    >
      <div className="flex flex-wrap items-baseline gap-x-2">
        <h3 className="font-heading text-sm font-semibold text-text">
          {title}
        </h3>
        {note && <span className="text-xs text-muted">{note}</span>}
      </div>
      {children}
    </Box>
  );
}
