import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { CatalogChart } from "@/startup/bundled-data";
import catalog from "../../../../catalog/official-plugins.json";
import { CatalogPanel } from "./catalog-panel";
import { registrationSources } from "./fixtures";

const sections = registrationSources().sections;

const meta = {
  title: "setup/plugins/CatalogPanel",
  component: CatalogPanel,
  args: { charts: catalog.charts as CatalogChart[], sections },
} satisfies Meta<typeof CatalogPanel>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ThreeCharts: Story = { name: "Three charts" };

export const CatalogNotRead: Story = {
  name: "The catalog cannot be read",
  args: {
    error:
      "reading catalog /app/catalog/official-plugins.json: ENOENT: no such file or directory",
  },
};

export const InstalledNotKnown: Story = {
  name: "Which charts are installed is not known",
  args: { installedUnknown: true },
};
