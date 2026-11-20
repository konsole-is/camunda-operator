"use client";

import { useEffect, useState } from "react";

/**
 * useClock returns initial, then the time every intervalMs while ticking,
 * so that the first render in the browser matches the server.
 */
export function useClock(
  initial: string,
  ticking: boolean,
  intervalMs = 1000,
): string {
  const [clock, setClock] = useState(initial);
  useEffect(() => {
    if (!ticking) return;
    const timer = setInterval(
      () => setClock(new Date().toISOString()),
      intervalMs,
    );
    return () => clearInterval(timer);
  }, [ticking, intervalMs]);
  return clock;
}
