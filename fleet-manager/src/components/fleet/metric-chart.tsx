"use client";

import { useId, useState, useSyncExternalStore } from "react";
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
import { dateTime, formatAxis, formatClock } from "./units";

const COLORS = ["var(--info)", "var(--accent)", "var(--ok)", "var(--warn)"];

// Recharts keeps the width of each axis label for the life of the page, and
// the label style names no font, so a label measured in the fallback font
// keeps that width.
const CHART_FONT = '400 11px "IBM Plex Sans"';

/**
 * FONT_WAIT_MS is how long a chart waits for its font before it draws in the
 * fallback font.
 */
export const FONT_WAIT_MS = 3000;

/**
 * MetricChart draws Prometheus range series as lines. It renders nothing
 * for "unavailable", so a page without Prometheus hides the chart. A range
 * without points says "No data for this range". note shows under the title.
 * It draws the lines once the font of the app has loaded, or after
 * FONT_WAIT_MS, so the axis labels fit that font.
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
  const fontReady = useChartFont();
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
          {fontReady && (
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
                  labelFormatter={(t) =>
                    dateTime(new Date(Number(t)).toISOString())
                  }
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
          )}
        </div>
      )}
      {!empty && (
        <p className="-mt-1 text-right text-xs text-muted">Time (UTC)</p>
      )}
    </figure>
  );
}

function useChartFont(): boolean {
  const [wait] = useState(fontWait);
  return useSyncExternalStore(wait.subscribe, wait.ready, () => false);
}

function fontWait() {
  let over = false;
  return {
    ready: () => over || !document.fonts || document.fonts.check(CHART_FONT),
    subscribe: (changed: () => void) => {
      if (!document.fonts) return () => {};
      let live = true;
      const done = () => {
        if (!live) return;
        over = true;
        changed();
      };
      const timer = setTimeout(done, FONT_WAIT_MS);
      // load, not ready: ready resolves at once when no font load is pending.
      document.fonts.load(CHART_FONT).then(done, done);
      return () => {
        live = false;
        clearTimeout(timer);
      };
    },
  };
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
