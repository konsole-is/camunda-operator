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
});
