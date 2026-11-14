import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { renderJSON } from "@/lib/gotemplate";
import { sectionFixtures } from "./fixtures";
import { emptyPreviewText, previewClaims, templateData } from "./preview";
import { TemplatePreview } from "./template-preview";

const sources = sectionFixtures();
const production = sources.environments[0];
const role = sources.registrations.find(
  (r) => r.metadata.name === "aws-connectors-role",
)!;
const permissions = sources.sections.find(
  (s) => s.metadata.name === "connectors-permissions",
)!;

const meta = {
  title: "setup/environments/TemplatePreview",
  component: TemplatePreview,
} satisfies Meta<typeof TemplatePreview>;
export default meta;
type Story = StoryObj<typeof meta>;

export const InlinePolicy: Story = {
  name: "Inline policy",
  args: {
    claims: previewClaims(sources, "production", "aws-connectors-role"),
    empty: emptyPreviewText(sources, "production"),
    render: (claim) =>
      renderJSON(
        production.spec.parameters?.["aws-connectors-role"]?.inlinePolicy
          ?.value,
        templateData({
          claim,
          section: permissions,
          registration: role,
          environment: production,
          registrations: sources.registrations,
          requests: sources.requests,
        }),
        "parameter",
      ),
  },
};
