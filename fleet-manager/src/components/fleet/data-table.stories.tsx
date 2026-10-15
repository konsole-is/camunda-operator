import type { Story } from "@/gallery/registry";
import { StatusChip } from "./status-chip";
import { type DataTableRow, DataTable } from "./data-table";

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

export const dataTableStories: Story[] = [
  {
    component: "DataTable",
    name: "Nothing selected",
    render: () => (
      <DataTable
        label="Clusters"
        columns={columns}
        rows={rows}
        filters={filters}
        selection={{ applicableLabel: "clusters that can upgrade" }}
      />
    ),
  },
  {
    component: "DataTable",
    name: "Rows that are not applicable",
    render: () => (
      <DataTable
        label="Clusters"
        columns={columns}
        rows={rows.map((r) => ({ ...r, applicable: r.id === "orders" }))}
        selection={{ applicableLabel: "clusters that can upgrade" }}
      />
    ),
  },
  {
    component: "DataTable",
    name: "Pages",
    render: () => (
      <DataTable label="Clusters" columns={columns} rows={rows} pageSize={2} />
    ),
  },
  {
    component: "DataTable",
    name: "Empty",
    render: () => (
      <DataTable
        label="Clusters"
        columns={columns}
        rows={[]}
        emptyText="No clusters yet. Create one to see it here."
      />
    ),
  },
];
