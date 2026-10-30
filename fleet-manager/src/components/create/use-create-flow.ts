"use client";

import { useLayoutEffect, useRef, useState } from "react";

/** DraftPath names a value in a draft, one key or index for each level, so a key can hold a dot. */
export type DraftPath = readonly (string | number)[];

/** CreateStep is one step of a create flow. check returns the errors of the step for a draft. */
export type CreateStep<T> = {
  id: string;
  label: string;
  check?: (draft: T) => string[];
};

/**
 * useCreateFlow holds the draft and the current step of a create flow: the
 * cluster flow, the plane flow (#655), and the later create steps. The
 * draft is not the edit session of #633. It starts at initial and the first
 * step.
 *
 * set returns a new draft with value at path, and leaves the parts off the
 * path shared. undefined deletes the key. errors holds the errors of the
 * current step and of each step before it, and nothing for a later step.
 * next moves on only while the current step has no error. back moves one
 * step back. go moves only to a step before the current one, and ignores
 * any other id. steps can be a new array on each render: the newest check
 * of each step gives the errors.
 */
export function useCreateFlow<T extends object>(options: {
  steps: readonly CreateStep<T>[];
  initial: T;
}): {
  draft: T;
  set: (path: DraftPath, value: unknown) => void;
  current: string;
  go: (id: string) => void;
  next: () => void;
  back: () => void;
  errors: Record<string, string[]>;
} {
  const { steps } = options;
  const [state, setState] = useState(() => ({
    draft: options.initial,
    current: steps[0]?.id ?? "",
  }));
  // next, back, and go run in updaters, after the render that made them.
  const latest = useRef(steps);
  useLayoutEffect(() => {
    latest.current = steps;
  });

  const indexOf = (s: readonly CreateStep<T>[], id: string) =>
    Math.max(
      0,
      s.findIndex((step) => step.id === id),
    );
  const index = indexOf(steps, state.current);
  const errors: Record<string, string[]> = {};
  for (const step of steps.slice(0, index + 1)) {
    errors[step.id] = step.check?.(state.draft) ?? [];
  }

  return {
    draft: state.draft,
    current: steps[index]?.id ?? state.current,
    errors,
    set: (path, value) =>
      setState((s) => ({ ...s, draft: setIn(s.draft, path, value) as T })),
    next: () =>
      setState((s) => {
        const all = latest.current;
        const i = indexOf(all, s.current);
        if (i >= all.length - 1 || (all[i].check?.(s.draft) ?? []).length) {
          return s;
        }
        return { ...s, current: all[i + 1].id };
      }),
    back: () =>
      setState((s) => {
        const i = indexOf(latest.current, s.current);
        return i === 0 ? s : { ...s, current: latest.current[i - 1].id };
      }),
    go: (id) =>
      setState((s) => {
        const all = latest.current;
        const target = all.findIndex((step) => step.id === id);
        return target >= 0 && target < indexOf(all, s.current)
          ? { ...s, current: id }
          : s;
      }),
  };
}

function setIn(target: unknown, path: DraftPath, value: unknown): unknown {
  if (path.length === 0) return value;
  const [key, ...rest] = path;
  const copy = (
    Array.isArray(target)
      ? [...(target as unknown[])]
      : typeof target === "object" && target !== null
        ? { ...target }
        : typeof key === "number"
          ? []
          : {}
  ) as Record<string | number, unknown>;
  if (rest.length === 0 && value === undefined) {
    if (Array.isArray(copy)) copy.splice(Number(key), 1);
    else delete copy[key];
    return copy;
  }
  copy[key] = setIn(copy[key], rest, value);
  return copy;
}
