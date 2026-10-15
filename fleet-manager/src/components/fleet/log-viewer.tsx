"use client";

import { useEffect, useId, useRef, useState } from "react";
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
 * level and text filters apply to the lines that show. A line with extra
 * fields opens to show them.
 */
export function LogViewer({
  lines,
  label = "Logs",
}: {
  lines: LogLine[];
  label?: string;
}) {
  const id = useId();
  const [paused, setPaused] = useState<LogLine[] | undefined>();
  const [level, setLevel] = useState("");
  const [text, setText] = useState("");
  const box = useRef<HTMLDivElement>(null);

  const kept = paused ?? lines;
  const newLines = paused ? countAfter(paused, lines) : 0;
  const shown = kept.filter(
    (l) =>
      (!level || l.level?.toLowerCase() === level) &&
      (!text || l.raw.toLowerCase().includes(text.toLowerCase())),
  );
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
        className="h-80 overflow-auto rounded-md border border-border bg-panel py-1 font-mono text-xs leading-5"
      >
        {shown.length === 0 && (
          <p className="px-3 py-2 font-sans text-muted">No lines</p>
        )}
        {shown.map((line, i) => {
          const row = (
            <>
              {line.time && (
                <span className="shrink-0 text-muted">
                  {line.time.slice(11, 19)}
                </span>
              )}
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
              <span className="min-w-0 break-all whitespace-pre-wrap text-text">
                {line.message}
              </span>
            </>
          );
          return Object.keys(line.fields).length > 0 ? (
            <details key={i} className="px-3">
              <summary className="flex cursor-pointer gap-3">{row}</summary>
              <pre className="ml-6 text-muted">
                {JSON.stringify(line.fields, null, 2)}
              </pre>
            </details>
          ) : (
            <div key={i} className="flex gap-3 px-3">
              {row}
            </div>
          );
        })}
      </div>
    </div>
  );
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
