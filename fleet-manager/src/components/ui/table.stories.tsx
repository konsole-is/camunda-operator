import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "./table";

const meta = {
  title: "ui/Table",
  component: Table,
} satisfies Meta<typeof Table>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ThreeRows: Story = {
  name: "Three rows",
  render: (args) => (
    <Table {...args}>
      <TableHeader>
        <TableRow>
          <TableHead>Cluster</TableHead>
          <TableHead>Namespace</TableHead>
          <TableHead>Version</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow>
          <TableCell>orders</TableCell>
          <TableCell>team-orders</TableCell>
          <TableCell>8.9.23</TableCell>
        </TableRow>
        <TableRow>
          <TableCell>payments</TableCell>
          <TableCell>team-payments</TableCell>
          <TableCell>8.9.23</TableCell>
        </TableRow>
        <TableRow>
          <TableCell>shipping</TableCell>
          <TableCell>team-orders</TableCell>
          <TableCell>8.9.22</TableCell>
        </TableRow>
      </TableBody>
    </Table>
  ),
};

export const Wide: Story = {
  name: "Wide",
  args: { label: "Clusters" },
  render: (args) => (
    <Table {...args}>
      <TableHeader>
        <TableRow>
          <TableHead>Cluster</TableHead>
          <TableHead>Namespace</TableHead>
          <TableHead>Environment</TableHead>
          <TableHead>Release</TableHead>
          <TableHead>Image</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow>
          <TableCell>orders</TableCell>
          <TableCell>team-orders</TableCell>
          <TableCell>production-eu-north-1</TableCell>
          <TableCell>8.9 long-term support</TableCell>
          <TableCell>registry.example.com/camunda/zeebe:8.9.23</TableCell>
        </TableRow>
      </TableBody>
    </Table>
  ),
};
