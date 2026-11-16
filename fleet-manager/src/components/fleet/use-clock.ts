"use client";

import { useEffect, useState } from "react";

/**
 * useClock returns initial, then the time each second while ticking, so that
 * the first render in the browser matches the server.
 */
export function useClock(initial: string, ticking: boolean): string {
  const [clock, setClock] = useState(initial);
  useEffect(() => {
    if (!ticking) return;
    const timer = setInterval(() => setClock(new Date().toISOString()), 1000);
    return () => clearInterval(timer);
  }, [ticking]);
  return clock;
}
