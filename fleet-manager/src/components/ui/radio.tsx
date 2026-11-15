import type * as React from "react";
import { cn } from "@/lib/utils";

/**
 * Radio is a radio button with the look of the Checkbox. It takes the props
 * of an input of type radio. Its pointer target is at least 24 px.
 */
function Radio({
  className,
  ...props
}: Omit<React.ComponentProps<"input">, "type">) {
  return (
    <input
      type="radio"
      data-slot="radio"
      className={cn(
        "peer relative size-4 shrink-0 cursor-pointer appearance-none rounded-full border border-control-edge-strong bg-input/30 transition-colors outline-none hit-area focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 checked:border-primary checked:bg-[radial-gradient(circle,var(--color-primary-foreground)_0_3px,var(--color-primary)_3.5px)]",
        className,
      )}
      {...props}
    />
  );
}

export { Radio };
