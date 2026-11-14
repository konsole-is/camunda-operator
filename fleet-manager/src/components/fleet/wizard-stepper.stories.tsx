import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { WizardStepper } from "./wizard-stepper";

const steps = (errors?: string[]) => [
  { id: "namespace", label: "Namespace" },
  { id: "storage", label: "Storage and access", errors },
  { id: "settings", label: "Settings" },
  { id: "review", label: "Review" },
];

const meta = {
  title: "fleet/WizardStepper",
  component: WizardStepper,
  args: {
    steps: steps(),
    current: "storage",
    children: (
      <p className="text-sm text-text-2">
        The section cards of the step go here.
      </p>
    ),
    onNext: fn(),
  },
} satisfies Meta<typeof WizardStepper>;
export default meta;
type Story = StoryObj<typeof meta>;

export const StepWithoutErrors: Story = { name: "Step without errors" };

export const StepWithErrors: Story = {
  name: "Step with errors",
  args: { steps: steps(["Choose a secondary storage option"]) },
};

export const LastStep: Story = {
  name: "Last step",
  args: {
    current: "review",
    children: <p className="text-sm text-text-2">The review goes here.</p>,
  },
};
