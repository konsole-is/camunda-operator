// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { Table, TableBody, TableCell, TableRow } from "./table";

afterEach(cleanup);

const rows = (
  <TableBody>
    <TableRow>
      <TableCell>orders</TableCell>
    </TableRow>
  </TableBody>
);

describe("Table", () => {
  it("is a named region that takes focus when the caller gives a label", () => {
    render(<Table label="Clusters">{rows}</Table>);
    const region = screen.getByRole("region", { name: "Clusters" });
    expect(region.getAttribute("tabindex")).toBe("0");
    expect(region.querySelector("table")).toBeTruthy();
  });

  it("adds no region and no tab stop without a label", () => {
    const { container } = render(<Table>{rows}</Table>);
    expect(screen.queryByRole("region")).toBeNull();
    expect(container.querySelectorAll("[tabindex]")).toHaveLength(0);
  });
});
