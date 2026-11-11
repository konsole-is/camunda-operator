"use client";

import { type ReactNode, useEffect, useId, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import type { LogLine } from "@/lib/observe/types";
import { cn } from "@/lib/utils";

const LEVEL_TONE: Record<string, string> = {
  error: "text-bad",
  warn: "text-warn",
  warning: "text-warn",
  debug: "text-muted",
};

/**
 * LogViewer shows log lines and follows new ones. Pause keeps the lines
 * that show and counts the new ones, and Follow shows them all again. The
 * level and text filters apply to the lines that show. controls go first
 * in the toolbar. Each line shows its time in UTC, its pod when the lines
 * come from more than one pod, its level, its logger, and its message. A
 * line with other fields opens to show them.
 */
export function LogViewer({
  lines,
  label = "Logs",
  controls,
}: {
  lines: LogLine[];
  label?: string;
  controls?: ReactNode;
}) {
  const id = useId();
  const [paused, setPaused] = useState<LogLine[] | undefined>();
  const [level, setLevel] = useState("");
  const [text, setText] = useState("");
  const box = useRef<HTMLDivElement>(null);
  // A buffer drops and adds lines, so a position does not name a line. Each
  // line object keeps its key, also when its text repeats another line.
  const [keyOf] = useState(() => {
    const keys = new WeakMap<LogLine, number>();
    let next = 0;
    return (line: LogLine) => {
      if (!keys.has(line)) keys.set(line, next++);
      return keys.get(line)!;
    };
  });

  const kept = paused ?? lines;
  const newLines = paused ? countAfter(paused, lines) : 0;
  const shown = kept.filter(
    (l) =>
      (!level || l.level?.toLowerCase() === level) &&
      (!text || l.raw.toLowerCase().includes(text.toLowerCase())),
  );
  const columns = {
    time: kept.some((l) => l.time),
    pod: new Set(kept.map((l) => l.pod)).size > 1,
    logger: kept.some((l) => l.logger),
  };
  const levels = [
    ...new Set(
      [...kept.map((l) => l.level?.toLowerCase()), level].filter(
        (l): l is string => !!l,
      ),
    ),
  ];

  useEffect(() => {
    if (paused === undefined && box.current)
      box.current.scrollTop = box.current.scrollHeight;
  }, [lines, paused]);

  return (
    <div className="flex min-w-0 flex-col gap-2">
      <div className="flex flex-wrap items-end gap-2">
        {controls}
        {paused === undefined ? (
          <Button variant="outline" size="sm" onClick={() => setPaused(lines)}>
            Pause
          </Button>
        ) : (
          <Button
            variant="outline"
            size="sm"
            onClick={() => setPaused(undefined)}
          >
            Follow
          </Button>
        )}
        <div className="flex flex-col gap-1">
          <label htmlFor={`${id}-level`} className="text-xs text-muted">
            Level
          </label>
          <NativeSelect
            id={`${id}-level`}
            value={level}
            onChange={(e) => setLevel(e.target.value)}
            className="w-32"
          >
            <NativeSelectOption value="">All levels</NativeSelectOption>
            {levels.map((l) => (
              <NativeSelectOption key={l} value={l}>
                {l}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>
        <div className="flex min-w-40 flex-1 flex-col gap-1">
          <label htmlFor={`${id}-filter`} className="text-xs text-muted">
            Filter
          </label>
          <Input
            id={`${id}-filter`}
            type="search"
            value={text}
            onChange={(e) => setText(e.target.value)}
          />
        </div>
        {newLines > 0 && (
          <span className="text-xs text-info">
            {newLines === 1 ? "1 new line" : `${newLines} new lines`}
          </span>
        )}
      </div>
      <div
        ref={box}
        role="log"
        aria-label={label}
        aria-live="off"
        tabIndex={0}
        className="h-80 overflow-auto rounded-md border border-border bg-panel py-1 font-mono text-xs leading-5 outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        {shown.length === 0 && (
          <p className="px-3 py-2 font-sans text-muted">No lines</p>
        )}
        {shown.map((line) => {
          const row = (
            <>
              {columns.time && (
                <span className="min-w-16 shrink-0 text-muted">
                  {line.time && utcClock(line.time)}
                </span>
              )}
              {columns.pod && <Clipped text={line.pod} width="w-28 sm:w-56" />}
              {line.level && (
                <span
                  className={cn(
                    "w-12 shrink-0",
                    LEVEL_TONE[line.level.toLowerCase()] ?? "text-text-2",
                  )}
                >
                  {line.level}
                </span>
              )}
              {columns.logger && (
                <Clipped text={line.logger} width="hidden w-48 sm:block" />
              )}
              <span className="min-w-0 basis-full break-all whitespace-pre-wrap text-text sm:basis-auto">
                {line.message}
              </span>
            </>
          );
          return Object.keys(line.fields).length > 0 ? (
            <details key={keyOf(line)} className="px-3">
              <summary className="flex cursor-pointer flex-wrap gap-x-3 sm:flex-nowrap">
                {row}
              </summary>
              <pre className="ml-6 text-muted">
                {JSON.stringify(line.fields, null, 2)}
              </pre>
            </details>
          ) : (
            <div
              key={keyOf(line)}
              className="flex flex-wrap gap-x-3 px-3 sm:flex-nowrap"
            >
              {row}
            </div>
          );
        })}
      </div>
      {columns.time && (
        <p className="-mt-1 text-right text-xs text-muted">Time (UTC)</p>
      )}
    </div>
  );
}

// A pod or a logger name differs at its end, so a long one loses its start.
function Clipped({ text, width }: { text?: string; width: string }) {
  return (
    <span
      title={text}
      className={cn(
        "shrink-0 truncate text-left text-muted [direction:rtl]",
        width,
      )}
    >
      <bdi>{text}</bdi>
    </span>
  );
}

// A time that Date cannot read shows as the line wrote it.
function utcClock(time: string): string {
  const ms = Date.parse(time);
  return Number.isNaN(ms) ? time : new Date(ms).toISOString().slice(11, 19);
}

// A buffer can drop its oldest lines, so a count by length is wrong.
function countAfter(shown: LogLine[], lines: LogLine[]): number {
  const last = shown.at(-1);
  if (!last) return lines.length;
  // Identity first, so a repeated line counts. Content for a caller that
  // builds new line objects for each render.
  const key = (l: LogLine) => `${l.time}\n${l.raw}\n${l.message}`;
  const same = lines.lastIndexOf(last);
  const index =
    same !== -1 ? same : lines.findLastIndex((l) => key(l) === key(last));
  return index === -1 ? lines.length : lines.length - 1 - index;
}
