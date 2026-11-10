"use client";

import { useId } from "react";
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { Series, Unavailable } from "@/lib/observe/types";
import { formatAxis, formatClock, formatDateClock } from "./units";

const COLORS = ["var(--info)", "var(--accent)", "var(--ok)", "var(--warn)"];

/**
 * MetricChart draws Prometheus range series as lines. It renders nothing
 * for "unavailable", so a page without Prometheus hides the chart. A range
 * without points says "No data for this range". note shows under the title.
 */
export function MetricChart({
  title,
  unit,
  note,
  series,
}: {
  title: string;
  unit?: string;
  note?: string;
  series: Series[] | Unavailable;
}) {
  const captionId = useId();
  if (series === "unavailable") return null;
  const empty = series.every((s) => s.points.length === 0);
  const rows = toRows(series);
  return (
    <figure
      aria-labelledby={captionId}
      className="flex min-w-0 flex-col gap-2 rounded-md border border-border bg-panel p-3"
    >
      <figcaption id={captionId} className="text-sm font-medium text-text">
        {title}
        {unit && (
          <span className="ml-1 font-normal text-muted">{`(${unit})`}</span>
        )}
      </figcaption>
      {note && <p className="-mt-1 text-xs text-muted">{note}</p>}
      {empty ? (
        <p className="py-8 text-center text-sm text-muted">
          No data for this range
        </p>
      ) : (
        <div
          role="img"
          aria-label={`${title}, ${series.map((s) => s.name).join(", ")}`}
          className="h-48 w-full"
        >
          <ResponsiveContainer width="100%" height="100%">
            <LineChart
              data={rows}
              margin={{ top: 4, right: 8, bottom: 0, left: 0 }}
            >
              <CartesianGrid stroke="var(--divider)" vertical={false} />
              <XAxis
                dataKey="time"
                type="number"
                scale="time"
                domain={["dataMin", "dataMax"]}
                tickFormatter={(t: number) => formatClock(t)}
                stroke="var(--muted)"
                fontSize={11}
              />
              <YAxis
                stroke="var(--muted)"
                fontSize={11}
                width={56}
                tickFormatter={(v: number) => formatAxis(v, unit)}
              />
              <Tooltip
                contentStyle={{
                  background: "var(--panel-2)",
                  border: "1px solid var(--border)",
                  color: "var(--text)",
                }}
                labelFormatter={(t) => formatDateClock(Number(t))}
                formatter={(value) => formatAxis(Number(value), unit)}
              />
              {series.map((s, i) => (
                <Line
                  key={s.name}
                  dataKey={seriesKey(i)}
                  name={s.name}
                  stroke={COLORS[i % COLORS.length]}
                  dot={false}
                  strokeWidth={1.5}
                  isAnimationActive={false}
                />
              ))}
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}
    </figure>
  );
}

function toRows(series: Series[]): Record<string, number>[] {
  const byTime = new Map<number, Record<string, number>>();
  series.forEach((s, i) => {
    for (const [time, value] of s.points) {
      byTime.set(time, {
        ...(byTime.get(time) ?? { time }),
        [seriesKey(i)]: value,
      });
    }
  });
  return [...byTime.values()].sort((a, b) => a.time - b.time);
}

// A row key, not the series name: a name such as "time" would replace the time.
function seriesKey(index: number): string {
  return `series-${index}`;
}
