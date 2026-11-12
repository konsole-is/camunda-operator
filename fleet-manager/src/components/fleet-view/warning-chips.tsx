"use client";

import { CircleAlert, TriangleAlert } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";
import { age } from "@/components/fleet/age";
import { CHIP_TONE } from "@/components/fleet/status";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { Unavailable } from "@/lib/observe/types";
import type { PodWarnings, Warning } from "@/lib/warnings";
import { cn } from "@/lib/utils";

const SETTINGS = "/setup/settings";
const LINK = "text-info underline underline-offset-2";

/**
 * WarningChips shows one chip for each warning, Error first, with the
 * detail and the age of since as a tooltip. Without warnings it shows "-".
 */
export function WarningChips({
  warnings,
  now,
}: {
  warnings: Warning[];
  now: string;
}): ReactNode {
  if (warnings.length === 0) return <span className="text-muted">-</span>;
  const ordered = [
    ...warnings.filter((w) => w.severity === "Error"),
    ...warnings.filter((w) => w.severity !== "Error"),
  ];
  return (
    <TooltipProvider>
      <span className="flex flex-wrap gap-1">
        {ordered.map((warning, i) => {
          const Icon =
            warning.severity === "Error" ? CircleAlert : TriangleAlert;
          return (
            <Tooltip key={`${warning.kind}-${i}`}>
              <TooltipTrigger asChild>
                <button
                  type="button"
                  className={cn(
                    "inline-flex max-w-full items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                    CHIP_TONE[warning.severity === "Error" ? "bad" : "warn"],
                  )}
                >
                  <Icon aria-hidden className="size-3 shrink-0" />
                  <span className="sr-only">{warning.severity}:</span>{" "}
                  <span className="truncate">{warning.label}</span>
                </button>
              </TooltipTrigger>
              <TooltipContent>
                {warning.since
                  ? `${sentence(warning.detail)}, ${age(warning.since, now)} ago.`
                  : `${sentence(warning.detail)}.`}
              </TooltipContent>
            </Tooltip>
          );
        })}
      </span>
    </TooltipProvider>
  );
}

// A detail can be a condition message that ends with its own period.
function sentence(detail: string): string {
  return detail.endsWith(".") ? detail.slice(0, -1) : detail;
}

/**
 * MissingSources says once which source of the pod and metric warnings is
 * missing, and which warnings do not show because of it. It shows nothing
 * before the first answer and while no source is missing.
 */
export function MissingSources({
  pods,
}: {
  pods: PodWarnings | Unavailable | undefined;
}): ReactNode {
  if (pods === undefined) return null;
  if (pods === "unavailable") {
    return (
      <p className="text-sm text-muted">
        The warnings of the pods do not show, because the fleet manager could
        not read them.
      </p>
    );
  }
  if (pods.missing.length === 0) return null;
  return (
    <div className="flex flex-col gap-1 text-sm text-muted">
      {pods.missing.includes("metrics-server") && (
        <p>
          metrics-server is not found, so the near memory and CPU limit warnings
          do not show.{" "}
          <Link href={SETTINGS} className={LINK}>
            Open Settings
          </Link>
        </p>
      )}
      {pods.missing.includes("prometheus") && (
        <p>
          Prometheus is not set or does not answer, so the backpressure warning
          does not show.{" "}
          <Link href={SETTINGS} className={LINK}>
            Set the Prometheus URL in Settings
          </Link>
        </p>
      )}
    </div>
  );
}
