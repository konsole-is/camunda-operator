"use client";

import Link from "next/link";
import { useId, useState } from "react";
import { MetricChart } from "@/components/fleet/metric-chart";
import { type PodLike, USAGE_UNAVAILABLE } from "@/components/fleet/pod-table";
import { UsageBars } from "@/components/fleet/usage-bars";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type {
  MetricsAnswer,
  MetricsRange,
  MetricsRef,
  PodUsage,
  Unavailable,
} from "@/lib/observe/types";
import { useMetrics } from "@/lib/observe/use-observe";

/** METRICS_RANGE_LABELS names each range of the Range select, in order. */
export const METRICS_RANGE_LABELS: Record<MetricsRange, string> = {
  "1h": "Last hour",
  "6h": "Last 6 hours",
  "24h": "Last 24 hours",
};

/**
 * DetailMetrics is the Metrics tab of a component detail: the live usage of
 * pods, then the charts of source over the range the user picks, the last
 * hour first. Without source it asks no charts and shows no history. It
 * needs a QueryClientProvider above it.
 */
export function DetailMetrics({
  namespace,
  source,
  pods,
  usage,
}: {
  namespace: string;
  source?: MetricsRef;
  pods: PodLike[];
  usage: PodUsage[] | Unavailable | undefined;
}) {
  const [range, setRange] = useState<MetricsRange>("1h");
  const answer = useMetrics(namespace, source, range);
  return (
    <DetailMetricsView
      pods={pods}
      usage={usage}
      charts={source ? (answer ?? "loading") : undefined}
      range={range}
      onRange={setRange}
    />
  );
}

/**
 * DetailMetricsView is DetailMetrics with its answer given. An undefined
 * usage is not loaded yet. Without charts, the view has no history
 * section.
 */
export function DetailMetricsView({
  pods,
  usage,
  charts,
  range,
  onRange,
}: {
  pods: PodLike[];
  usage: PodUsage[] | Unavailable | undefined;
  charts?: MetricsAnswer | "failed" | "loading";
  range: MetricsRange;
  onRange?: (range: MetricsRange) => void;
}) {
  return (
    <div className="flex flex-col gap-6 pt-2">
      <LiveUsage pods={pods} usage={usage} />
      {charts !== undefined && (
        <History charts={charts} range={range} onRange={onRange} />
      )}
    </div>
  );
}

function LiveUsage({
  pods,
  usage,
}: {
  pods: PodLike[];
  usage: PodUsage[] | Unavailable | undefined;
}) {
  return (
    <section className="flex min-w-0 flex-col gap-2">
      <h3 className="font-heading text-sm font-semibold text-text">
        Live usage
      </h3>
      {usage === "unavailable" ? (
        <p className="text-sm text-muted">{USAGE_UNAVAILABLE}</p>
      ) : (
        <Table label="Live usage">
          <TableHeader>
            <TableRow>
              <TableHead>Pod</TableHead>
              <TableHead>CPU and memory</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {pods.map((pod) => {
              const name = pod.metadata?.name ?? "";
              const own = usage?.find((u) => u.pod === name);
              return (
                <TableRow key={name}>
                  <th
                    scope="row"
                    className="px-2 py-2 text-left align-top font-medium text-text"
                  >
                    {name}
                  </th>
                  <TableCell>{own ? <UsageBars usage={own} /> : "-"}</TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      )}
    </section>
  );
}

function History({
  charts,
  range,
  onRange,
}: {
  charts: MetricsAnswer | "failed" | "loading";
  range: MetricsRange;
  onRange?: (range: MetricsRange) => void;
}) {
  const id = useId();
  return (
    <section className="flex min-w-0 flex-col gap-3">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <h3 className="font-heading text-sm font-semibold text-text">
          History
        </h3>
        <div className="flex items-center gap-2">
          <label htmlFor={`${id}-range`} className="text-sm text-muted">
            Range
          </label>
          <NativeSelect
            id={`${id}-range`}
            className="w-40"
            value={range}
            onChange={(event) => onRange?.(event.target.value as MetricsRange)}
          >
            {Object.entries(METRICS_RANGE_LABELS).map(([value, label]) => (
              <NativeSelectOption key={value} value={value}>
                {label}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>
      </div>
      <HistoryBody charts={charts} />
    </section>
  );
}

function HistoryBody({
  charts,
}: {
  charts: MetricsAnswer | "failed" | "loading";
}) {
  if (charts === "loading") {
    return <p className="text-sm text-muted">Loading the charts</p>;
  }
  if (charts === "failed") {
    return <p className="text-sm text-muted">Could not load the charts.</p>;
  }
  if (charts.prometheus === "notConfigured") {
    return (
      <p className="text-sm text-muted">
        Prometheus is not configured, so the charts do not show.{" "}
        <Link
          href="/setup/settings"
          className="text-info underline underline-offset-2"
        >
          Set the Prometheus URL in Settings
        </Link>
      </p>
    );
  }
  if (charts.prometheus === "unavailable") {
    return (
      <p className="text-sm text-muted">
        Prometheus did not answer. The charts show again when it answers.
      </p>
    );
  }
  return (
    <div className="flex flex-col gap-4">
      {charts.charts.length === 0 ? (
        <p className="text-sm text-muted">No metric has data for this range.</p>
      ) : (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          {charts.charts.map((chart) => (
            <MetricChart
              key={`${chart.declaredBy ?? ""}/${chart.label}`}
              title={chart.label}
              unit={chart.unit}
              note={chart.declaredBy && `Declared by ${chart.declaredBy}`}
              series={chart.series}
            />
          ))}
        </div>
      )}
      {charts.errors.length > 0 && (
        <div className="flex flex-col gap-1">
          <h4 className="text-sm font-medium text-text">
            Queries that do not render
          </h4>
          <ul className="flex flex-col gap-1 text-sm text-warn">
            {charts.errors.map((e) => (
              <li key={`${e.declaredBy}/${e.label}`}>
                {`${e.label}, declared by ${e.declaredBy}: ${e.error}`}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
