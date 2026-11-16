// @vitest-environment jsdom
import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useClock } from "./use-clock";

const SERVER = "2026-11-13T09:00:00.000Z";

describe("useClock", () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["setInterval", "clearInterval", "Date"] });
    vi.setSystemTime(new Date("2026-11-13T09:00:05.000Z"));
  });
  afterEach(() => vi.useRealTimers());

  it("returns the time of the server until the first tick", () => {
    const { result } = renderHook(() => useClock(SERVER, true));
    expect(result.current).toBe(SERVER);
  });

  it("gives the time each second by default", () => {
    const { result } = renderHook(() => useClock(SERVER, true));
    act(() => vi.advanceTimersByTime(1000));
    expect(result.current).toBe("2026-11-13T09:00:06.000Z");
  });

  it("gives the time only after the interval it is given", () => {
    const { result } = renderHook(() => useClock(SERVER, true, 30_000));
    act(() => vi.advanceTimersByTime(29_999));
    expect(result.current).toBe(SERVER);
    act(() => vi.advanceTimersByTime(1));
    expect(result.current).toBe("2026-11-13T09:00:35.000Z");
  });

  it("keeps the time of the server while not ticking", () => {
    const { result } = renderHook(() => useClock(SERVER, false));
    act(() => vi.advanceTimersByTime(60_000));
    expect(result.current).toBe(SERVER);
  });
});
