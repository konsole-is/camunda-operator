// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { type DataTableRow, DataTable } from "./data-table";

afterEach(cleanup);

const columns = [
  { key: "name", label: "Name", sortable: true },
  { key: "version", label: "Version", sortable: true },
];
const row = (
  id: string,
  version: string,
  applicable: boolean,
  env = "prod",
): DataTableRow => ({
  id,
  label: id,
  href: `/fleet/${id}`,
  cells: { name: id, version },
  search: id,
  sort: { name: id, version },
  filters: { environment: env },
  applicable,
});
const rows = [
  row("orders", "8.9.20", true),
  row("billing", "8.9.23", false),
  row("payments", "8.9.21", true, "dev"),
];
const filters = [
  {
    key: "environment",
    label: "Environment",
    options: [
      { value: "prod", label: "Production" },
      { value: "dev", label: "Development" },
    ],
  },
];

function table(onChange = vi.fn(), pageSize?: number) {
  render(
    <DataTable
      label="Clusters"
      columns={columns}
      rows={rows}
      filters={filters}
      pageSize={pageSize}
      selection={{ applicableLabel: "clusters that can upgrade", onChange }}
    />,
  );
  return onChange;
}

const checked = () =>
  screen
    .queryAllByRole("checkbox")
    .filter((box) => box.getAttribute("aria-checked") === "true")
    .map((box) => box.getAttribute("aria-label"));

describe("DataTable", () => {
  it("selects nothing at first", () => {
    const onChange = table();
    expect(checked()).toEqual([]);
    expect(onChange).not.toHaveBeenCalled();
  });

  it("selects only the applicable rows with Select all", async () => {
    const onChange = table();
    await userEvent.click(
      screen.getByRole("button", {
        name: "Select all clusters that can upgrade",
      }),
    );
    expect(checked()).toEqual(["Select orders", "Select payments"]);
    expect(onChange).toHaveBeenLastCalledWith(["orders", "payments"]);
  });

  it("does not let a row that is not applicable be selected", () => {
    table();
    expect(
      (
        screen.getByRole("checkbox", {
          name: "Select billing",
        }) as HTMLButtonElement
      ).disabled,
    ).toBe(true);
  });

  it("selects only the rows that the search shows", async () => {
    const onChange = table();
    await userEvent.type(
      screen.getByRole("searchbox", { name: "Search" }),
      "pay",
    );
    await userEvent.click(
      screen.getByRole("button", {
        name: "Select all clusters that can upgrade",
      }),
    );
    expect(onChange).toHaveBeenLastCalledWith(["payments"]);
  });

  it("drops a selected row that a filter hides", async () => {
    const onChange = table();
    await userEvent.click(
      screen.getByRole("button", {
        name: "Select all clusters that can upgrade",
      }),
    );
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Environment" }),
      "prod",
    );
    expect(onChange).toHaveBeenLastCalledWith(["orders"]);
    expect(checked()).toEqual(["Select orders"]);
  });

  it("links each row from its first cell", () => {
    table();
    expect(
      screen.getByRole("link", { name: "orders" }).getAttribute("href"),
    ).toBe("/fleet/orders");
  });

  it("sorts by a column, then reverses the order", async () => {
    table();
    const names = () => screen.getAllByRole("link").map((a) => a.textContent);
    await userEvent.click(screen.getByRole("button", { name: "Name" }));
    expect(names()).toEqual(["billing", "orders", "payments"]);
    await userEvent.click(screen.getByRole("button", { name: "Name" }));
    expect(names()).toEqual(["payments", "orders", "billing"]);
    expect(
      screen
        .getByRole("columnheader", { name: /Name/ })
        .getAttribute("aria-sort"),
    ).toBe("descending");
  });

  it("shows one page at a time", async () => {
    table(vi.fn(), 2);
    expect(screen.getAllByRole("link")).toHaveLength(2);
    expect(screen.getByText("Page 1 of 2")).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Next page" }));
    expect(screen.getAllByRole("link").map((a) => a.textContent)).toEqual([
      "payments",
    ]);
  });

  it("shows the last page that is left when rows go away", async () => {
    const { rerender } = render(
      <DataTable label="Clusters" columns={columns} rows={rows} pageSize={2} />,
    );
    await userEvent.click(screen.getByRole("button", { name: "Next page" }));
    rerender(
      <DataTable
        label="Clusters"
        columns={columns}
        rows={rows.slice(0, 2)}
        pageSize={2}
      />,
    );
    expect(screen.getAllByRole("link").map((a) => a.textContent)).toEqual([
      "orders",
      "billing",
    ]);
  });

  it("drops a selected row that goes away or can no longer be selected", async () => {
    const onChange = vi.fn();
    const props = {
      label: "Clusters",
      columns,
      selection: { applicableLabel: "clusters that can upgrade", onChange },
    };
    const { rerender } = render(<DataTable {...props} rows={rows} />);
    await userEvent.click(
      screen.getByRole("button", {
        name: "Select all clusters that can upgrade",
      }),
    );
    expect(onChange).toHaveBeenLastCalledWith(["orders", "payments"]);

    rerender(
      <DataTable
        {...props}
        rows={[{ ...rows[0], applicable: false }, rows[1], rows[2]]}
      />,
    );
    expect(onChange).toHaveBeenLastCalledWith(["payments"]);
    expect(checked()).toEqual(["Select payments"]);

    rerender(<DataTable {...props} rows={rows.slice(0, 2)} />);
    expect(onChange).toHaveBeenLastCalledWith([]);
    expect(checked()).toEqual([]);
  });

  it.each([
    ["the filter", []],
    [
      "its picked option",
      [{ ...filters[0], options: [{ value: "prod", label: "Production" }] }],
    ],
  ])("stops a filter value when new filters remove %s", async (_, after) => {
    const { rerender } = render(
      <DataTable
        label="Clusters"
        columns={columns}
        rows={rows}
        filters={filters}
      />,
    );
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Environment" }),
      "dev",
    );
    expect(screen.getAllByRole("link").map((a) => a.textContent)).toEqual([
      "payments",
    ]);
    rerender(
      <DataTable
        label="Clusters"
        columns={columns}
        rows={rows}
        filters={after}
      />,
    );
    expect(screen.getAllByRole("link")).toHaveLength(3);
  });

  it("says so when no row matches", async () => {
    table();
    await userEvent.type(
      screen.getByRole("searchbox", { name: "Search" }),
      "zzz",
    );
    const body = screen.getAllByRole("rowgroup")[1];
    expect(
      within(body).getByText("No rows match the search and the filters."),
    ).toBeTruthy();
  });
});
