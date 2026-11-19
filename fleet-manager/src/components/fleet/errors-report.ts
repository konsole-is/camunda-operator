"use client";

import { useEffect, useRef } from "react";
import type { ValidationError } from "@/lib/schema";

/**
 * useErrorsReport gives onErrors the errors that an editor shows: after the
 * first render, and again each time they change. It gives no value, so a
 * report at mount changes nothing that the editor edits.
 */
export function useErrorsReport(
  errors: readonly ValidationError[],
  onErrors: ((errors: ValidationError[]) => void) | undefined,
) {
  const report = JSON.stringify(errors);
  const reported = useRef<string | undefined>(undefined);
  // A remount in strict mode keeps the ref, and its parent can forget the
  // errors on the unmount, so a remount reports again.
  useEffect(
    () => () => {
      reported.current = undefined;
    },
    [],
  );
  useEffect(() => {
    if (reported.current === report) return;
    reported.current = report;
    onErrors?.([...errors]);
  });
}
