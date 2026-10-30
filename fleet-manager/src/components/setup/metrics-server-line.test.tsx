// @vitest-environment jsdom
import { cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { MetricsServerLine } from "./metrics-server-line";

afterEach(cleanup);

describe("MetricsServerLine", () => {
  it("says that metrics-server is found and pages show live use", () => {
    const { container } = render(<MetricsServerLine state={{ found: true }} />);
    expect(container.textContent).toContain("metrics-server found");
    expect(container.textContent).toContain("Pages show live CPU and memory.");
  });

  it("says that metrics-server is not found and pages hide live use", () => {
    const { container } = render(
      <MetricsServerLine state={{ found: false }} />,
    );
    expect(container.textContent).toContain("metrics-server not found");
    expect(container.textContent).toContain(
      "Pages hide the live CPU and memory.",
    );
  });

  it("shows the error of a check that failed", () => {
    const { container } = render(
      <MetricsServerLine state={{ error: "connect ECONNREFUSED" }} />,
    );
    expect(container.textContent).toContain(
      "Could not check for metrics-server: connect ECONNREFUSED",
    );
  });
});
