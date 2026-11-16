// @vitest-environment jsdom
import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { FONT_WAIT_MS, MetricChart } from "./metric-chart";

const series = [
  {
    name: "zeebe-0",
    points: [
      [0, 1],
      [60_000, 2],
    ] as [number, number][],
  },
];
const drawn = (container: HTMLElement) =>
  container.querySelector(".recharts-responsive-container") !== null;

function stubFonts(fonts: {
  check(font: string): boolean;
  load(font: string): Promise<unknown>;
}) {
  Object.defineProperty(document, "fonts", {
    configurable: true,
    value: fonts,
  });
}

afterEach(() => {
  cleanup();
  Reflect.deleteProperty(document, "fonts");
  vi.useRealTimers();
});

describe("MetricChart", () => {
  it('renders nothing for "unavailable"', () => {
    const { container } = render(
      <MetricChart title="Exported records" series="unavailable" />,
    );
    expect(container.innerHTML).toBe("");
  });

  it("says so when the range has no data", () => {
    render(<MetricChart title="Exported records" series={[]} />);
    expect(screen.getByText("No data for this range")).toBeTruthy();
  });

  it("shows the note under the title", () => {
    render(
      <MetricChart
        title="Requests"
        note="Declared by Bucket on RustFS"
        series={[]}
      />,
    );
    const figure = screen.getByRole("figure", { name: /Requests/ });
    expect(figure.textContent).toContain("Declared by Bucket on RustFS");
  });

  it("says once that the times are in UTC", () => {
    render(
      <MetricChart
        title="CPU"
        series={[{ name: "zeebe-0", points: [[0, 1]] }]}
      />,
    );
    const figure = screen.getByRole("figure", { name: /CPU/ });
    expect(figure.textContent?.match(/UTC/g)).toHaveLength(1);
  });
  it("draws at once without document.fonts", () => {
    Reflect.deleteProperty(document, "fonts");
    const { container } = render(<MetricChart title="CPU" series={series} />);
    expect(drawn(container)).toBe(true);
  });

  it("waits for the font before it draws, and keeps the caption and the box", () => {
    stubFonts({ check: () => false, load: () => new Promise(() => {}) });
    const { container } = render(<MetricChart title="CPU" series={series} />);
    expect(drawn(container)).toBe(false);
    expect(screen.getByRole("figure", { name: /CPU/ })).toBeTruthy();
    expect(screen.getByRole("img", { name: "CPU, zeebe-0" })).toBeTruthy();
  });

  it("draws once the font has loaded", async () => {
    let resolve: (fonts: unknown) => void = () => {};
    const load = vi.fn(
      () =>
        new Promise<unknown>((r) => {
          resolve = r;
        }),
    );
    stubFonts({ check: () => false, load });
    const { container } = render(<MetricChart title="CPU" series={series} />);
    expect(drawn(container)).toBe(false);
    await act(async () => resolve([]));
    expect(drawn(container)).toBe(true);
    expect(load).toHaveBeenCalledWith('400 11px "IBM Plex Sans"');
  });

  it("draws at once when the font is already loaded", () => {
    stubFonts({ check: () => true, load: () => new Promise(() => {}) });
    const { container } = render(<MetricChart title="CPU" series={series} />);
    expect(drawn(container)).toBe(true);
  });

  it("draws after FONT_WAIT_MS when the font does not load", () => {
    vi.useFakeTimers();
    stubFonts({ check: () => false, load: () => new Promise(() => {}) });
    const { container } = render(<MetricChart title="CPU" series={series} />);
    expect(drawn(container)).toBe(false);
    act(() => vi.advanceTimersByTime(FONT_WAIT_MS - 1));
    expect(drawn(container)).toBe(false);
    act(() => vi.advanceTimersByTime(1));
    expect(drawn(container)).toBe(true);
  });

  it("draws when the font load fails", async () => {
    stubFonts({
      check: () => false,
      load: () => Promise.reject(new Error("no font")),
    });
    const { container } = render(<MetricChart title="CPU" series={series} />);
    await act(async () => {});
    expect(drawn(container)).toBe(true);
  });
});
