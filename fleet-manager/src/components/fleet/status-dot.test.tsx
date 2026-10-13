// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { StatusDot } from "./status-dot";

afterEach(cleanup);

describe("StatusDot", () => {
  it.each([
    [{ status: "True" as const, reason: "Healthy" }, false, "Zeebe: Ready"],
    [
      { status: "False" as const, reason: "PodsNotReady" },
      false,
      "Zeebe: Not ready (PodsNotReady)",
    ],
    [{ status: "Unknown" as const }, false, "Zeebe: Unknown"],
    [undefined, false, "Zeebe: No status"],
    [undefined, true, "Zeebe: Not used"],
  ])(
    "names the component and its state for %j (off: %s)",
    (condition, off, name) => {
      render(<StatusDot label="Zeebe" condition={condition} off={off} />);
      expect(screen.getByRole("img", { name })).toBeTruthy();
    },
  );
});
