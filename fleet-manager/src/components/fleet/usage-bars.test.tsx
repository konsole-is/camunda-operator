// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { UsageBars } from "./usage-bars";

afterEach(cleanup);

describe("UsageBars", () => {
  it.each([
    ["a request of 0", { usedCores: 0.5, requestCores: 0 }, "0.5", "0.5"],
    ["no use and no limit", { usedCores: 0 }, "0", "1"],
  ])("keeps a valid meter range with %s", (_, cpu, now, max) => {
    render(<UsageBars usage={{ pod: "x", cpu, memory: { usedBytes: 0 } }} />);
    const meter = screen.getByRole("meter", { name: "CPU" });
    expect(meter.getAttribute("aria-valuenow")).toBe(now);
    expect(meter.getAttribute("aria-valuemax")).toBe(max);
  });

  it("keeps the meter value in range when the use passes the request", () => {
    render(
      <UsageBars
        usage={{
          pod: "x",
          cpu: { usedCores: 0.5, requestCores: 0.25 },
          memory: { usedBytes: 2 ** 30 },
        }}
      />,
    );
    const cpu = screen.getByRole("meter", { name: "CPU" });
    expect(cpu.getAttribute("aria-valuenow")).toBe("0.25");
    expect(cpu.getAttribute("aria-valuemax")).toBe("0.25");
    expect(cpu.getAttribute("aria-valuetext")).toBe("500m of 250m cores");
  });

  it.each([
    ["at 90 % of a limit", true, { usedCores: 0.9, limitCores: 1 }],
    ["below 90 % of a limit", false, { usedCores: 0.85, limitCores: 1 }],
    [
      "above 90 % of a request with no limit",
      false,
      { usedCores: 0.95, requestCores: 1 },
    ],
  ])("marks use %s as a warning: %s", (_, warns, cpu) => {
    render(<UsageBars usage={{ pod: "x", cpu, memory: { usedBytes: 0 } }} />);
    const fill = screen.getByRole("meter", { name: "CPU" }).firstElementChild;
    expect(fill).not.toBeNull();
    expect(fill?.classList.contains("bg-warn")).toBe(warns);
  });

  it("shows only the use without a request or a limit", () => {
    render(
      <UsageBars
        usage={{ pod: "x", cpu: { usedCores: 0.95 }, memory: { usedBytes: 0 } }}
      />,
    );
    const cpu = screen.getByRole("meter", { name: "CPU" });
    expect(cpu.getAttribute("aria-valuetext")).toBe("950m cores");
    expect(cpu.firstElementChild).toBeNull();
  });

  it("names the request beside the limit when a pod has both", () => {
    render(
      <UsageBars
        usage={{
          pod: "x",
          cpu: { usedCores: 0.8, requestCores: 0.5, limitCores: 1 },
          memory: {
            usedBytes: 1.2 * 2 ** 30,
            requestBytes: 2 ** 30,
            limitBytes: 2 * 2 ** 30,
          },
        }}
      />,
    );
    expect(
      screen
        .getByRole("meter", { name: "Memory" })
        .getAttribute("aria-valuetext"),
    ).toBe("1.2 GiB of 2 GiB, request 1 GiB");
    expect(
      screen.getByRole("meter", { name: "CPU" }).getAttribute("aria-valuetext"),
    ).toBe("800m of 1 cores, request 500m cores");
    expect(screen.getByText("request 1 GiB")).toBeTruthy();
  });

  it("marks the request on the meter at its share of the limit", () => {
    const { container } = render(
      <UsageBars
        usage={{
          pod: "x",
          cpu: { usedCores: 0.8, requestCores: 0.25, limitCores: 1 },
          memory: { usedBytes: 0 },
        }}
      />,
    );
    const marks = container.querySelectorAll('[data-slot="request-mark"]');
    expect(marks).toHaveLength(1);
    expect((marks[0] as HTMLElement).style.left).toBe("25%");
  });

  it("names no request when the pod has only a request", () =>
    expect(
      render(
        <UsageBars
          usage={{
            pod: "x",
            cpu: { usedCores: 0.5, requestCores: 1 },
            memory: { usedBytes: 0 },
          }}
        />,
      ).container.querySelector('[data-slot="request-mark"]'),
    ).toBeNull());
});
