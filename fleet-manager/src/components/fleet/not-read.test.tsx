// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { NotRead, notReadText } from "./not-read";

afterEach(cleanup);

describe("notReadText", () => {
  it("says that a value is not known while the fleet manager has no current list of each kind", () => {
    expect(notReadText(["Namespace", "Environment"])).toBe(
      "Not known: the fleet manager has no current list of Namespace, Environment.",
    );
  });

  it("says for a tab that it can be incomplete or out of date", () => {
    expect(notReadText(["CamundaOptimize"], true)).toBe(
      "The fleet manager has no current list of CamundaOptimize. This tab can be incomplete or out of date.",
    );
  });
});

describe("NotRead", () => {
  it("shows the text of its kinds", () => {
    render(<NotRead kinds={["PluginRequest"]} tab />);
    expect(screen.getByText(notReadText(["PluginRequest"], true))).toBeTruthy();
  });

  it("renders nothing without kinds", () => {
    const { container } = render(<NotRead kinds={[]} />);
    expect(container.innerHTML).toBe("");
  });
});
