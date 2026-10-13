import { Lock } from "lucide-react";
import { type ReactNode, useId } from "react";

/**
 * LockedField shows a value that the user cannot change here, with the
 * reason. It is not an input: it has no form control and takes no focus.
 */
export function LockedField({
  label,
  value,
  reason,
}: {
  label: string;
  value: ReactNode;
  reason: string;
}) {
  const id = useId();
  return (
    <div
      role="group"
      aria-labelledby={`${id}-label`}
      aria-describedby={`${id}-reason`}
      className="flex min-w-0 flex-col gap-1"
    >
      <span id={`${id}-label`} className="text-sm font-medium text-muted">
        {label}
      </span>
      <div className="flex min-h-10 cursor-default items-center text-sm break-all text-text-2">
        {value}
      </div>
      <p
        id={`${id}-reason`}
        className="flex items-center gap-1.5 text-xs text-muted"
      >
        <Lock aria-hidden className="size-3.5 shrink-0" />
        {reason}
      </p>
    </div>
  );
}
