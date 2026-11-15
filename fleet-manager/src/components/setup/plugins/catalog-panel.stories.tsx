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

export const SevenCharts: Story = { name: "Seven charts" };

export const OneChartWithTwoSections: Story = {
  name: "One chart with two sections",
  args: {
    charts: [
      {
        name: "aws-identity",
        description:
          "Creates one AWS IAM role for each cluster and one for its Connectors.",
        sections: ["cloud-identity", "connectors-permissions"],
        chart:
          "oci://ghcr.io/konsole-is/camunda-operator-plugins/charts/aws-identity",
        version: "0.1.0",
        repository: "https://github.com/konsole-is/camunda-operator-plugins",
        installCommand:
          "helm install aws-identity oci://ghcr.io/konsole-is/camunda-operator-plugins/charts/aws-identity --version 0.1.0 --namespace camunda-operator-plugins --create-namespace",
      },
    ],
  },
};
