import { CircleAlert, CircleCheck, CircleSlash } from "lucide-react";

/**
 * MetricsServerLine says whether metrics-server serves the live CPU and
 * memory that the pages show, or why the check failed.
 */
export function MetricsServerLine({
  state,
}: {
  state: { found: boolean } | { error: string };
}) {
  if ("error" in state) {
    return (
      <p role="alert" className="flex items-start gap-2 text-sm text-bad">
        <CircleAlert aria-hidden className="mt-0.5 size-4 shrink-0" />
        Could not check for metrics-server: {state.error}
      </p>
    );
  }
  const Icon = state.found ? CircleCheck : CircleSlash;
  return (
    <p className="flex items-start gap-2 text-sm">
      <Icon
        aria-hidden
        className={`mt-0.5 size-4 shrink-0 ${state.found ? "text-ok" : "text-muted"}`}
      />
      <span>
        <span className="text-text">
          {state.found ? "metrics-server found" : "metrics-server not found"}
        </span>
        {". "}
        <span className="text-muted">
          {state.found
            ? "Pages show live CPU and memory."
            : "Pages hide the live CPU and memory."}
        </span>
      </span>
    </p>
  );
}
