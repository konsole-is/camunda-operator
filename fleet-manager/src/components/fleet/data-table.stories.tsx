import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { type DataTableRow, DataTable } from "./data-table";
import { StatusChip } from "./status-chip";

const columns = [
  { key: "name", label: "Name", sortable: true },
  { key: "environment", label: "Environment" },
  { key: "version", label: "Version", sortable: true },
  { key: "status", label: "Status" },
];
const clusters: [string, string, string, boolean][] = [
  ["orders", "Production", "8.9.20", true],
  ["billing", "Production", "8.9.23", true],
  ["payments", "Development", "8.9.21", false],
  ["search", "Development", "8.9.19", true],
];
const rows: DataTableRow[] = clusters.map(
  ([name, environment, version, ready]) => ({
    id: name,
    href: `/fleet/clusters/${name}`,
    cells: {
      name,
      environment,
      version,
      status: (
        <StatusChip
          condition={
            ready
              ? { status: "True" }
              : { status: "False", reason: "Converging" }
          }
        />
      ),
    },
    search: name,
    sort: { name, version },
    filters: { environment },
    applicable: version !== "8.9.23",
  }),
);
const filters = [
  {
    key: "environment",
    label: "Environment",
    options: [
      { value: "Production", label: "Production" },
      { value: "Development", label: "Development" },
    ],
  },
];
const selection = {
  applicableLabel: "clusters that can upgrade",
  onChange: fn(),
};

const meta = {
  title: "fleet/DataTable",
  component: DataTable,
  args: { label: "Clusters", columns, rows },
} satisfies Meta<typeof DataTable>;
export default meta;
type Story = StoryObj<typeof meta>;

export const NothingSelected: Story = {
  name: "Nothing selected",
  args: { filters, selection },
};

export const RowsThatAreNotApplicable: Story = {
  name: "Rows that are not applicable",
  args: {
    rows: rows.map((r) => ({ ...r, applicable: r.id === "orders" })),
    selection,
  },
};

export const Pages: Story = { name: "Pages", args: { pageSize: 2 } };

export const Empty: Story = {
  name: "Empty",
  args: { rows: [], emptyText: "No clusters yet. Create one to see it here." },
};
