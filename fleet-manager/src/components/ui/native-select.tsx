import * as React from "react";
import { ChevronDownIcon } from "lucide-react";
import { cn } from "@/lib/utils";

/**
 * NativeSelect is a real select with the look of the other controls.
 * className sizes the box around the select and its arrow.
 */
function NativeSelect({ className, ...props }: React.ComponentProps<"select">) {
  return (
    <div
      data-slot="native-select-wrapper"
      className={cn(
        "relative w-full min-w-0 has-[select:disabled]:opacity-50",
        className,
      )}
    >
      <select
        data-slot="native-select"
        className="h-8 w-full min-w-0 appearance-none rounded-lg border border-control-edge bg-transparent py-1 pr-8 pl-2.5 text-base transition-colors outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 md:text-sm dark:bg-input/30"
        {...props}
      />
      <ChevronDownIcon
        aria-hidden
        className="pointer-events-none absolute top-1/2 right-2.5 size-4 -translate-y-1/2 text-muted"
      />
    </div>
  );
}

function NativeSelectOption({
  className,
  ...props
}: React.ComponentProps<"option">) {
  return (
    <option
      data-slot="native-select-option"
      className={cn("bg-panel-2 text-text", className)}
      {...props}
    />
  );
}

export { NativeSelect, NativeSelectOption };
