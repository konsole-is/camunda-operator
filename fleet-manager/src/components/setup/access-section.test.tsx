// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { AccessSection } from "./access-section";

afterEach(cleanup);

describe("AccessSection", () => {
  it("shows the port-forward command for the Service of the app", () => {
    const { container } = render(
      <AccessSection
        namespace="fleet"
        serviceName="camunda-operator-fleet-manager"
      />,
    );
    expect(container.querySelector("code")?.textContent).toBe(
      "kubectl -n fleet port-forward service/camunda-operator-fleet-manager 3000:3000",
    );
    expect(screen.getByText("Then open http://localhost:3000.")).toBeTruthy();
  });

  it("names a placeholder when the Service name is not known", () => {
    const { container } = render(<AccessSection namespace="fleet" />);
    expect(container.querySelector("code")?.textContent).toBe(
      "kubectl -n fleet port-forward service/<fleet manager Service> 3000:3000",
    );
  });
});
