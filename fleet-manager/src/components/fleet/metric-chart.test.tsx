// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { MetricChart } from "./metric-chart";

afterEach(cleanup);

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
});
