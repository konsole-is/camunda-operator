import type { Story } from "@/gallery/registry";
import { NamespacePicker } from "./namespace-picker";
import { WizardStepper } from "./wizard-stepper";

const steps = (errors?: string[]) => [
  { id: "namespace", label: "Namespace" },
  { id: "storage", label: "Storage and access", errors },
  { id: "settings", label: "Settings" },
  { id: "review", label: "Review" },
];
const environment = {
  name: "prod",
  displayName: "Production",
  namespaceCreation: {
    allowed: true,
    labels: { "cost-center": "4711" },
    requiredLabelKeys: ["team"],
  },
};
const namespaces = [
  { name: "team-payments", environment: "prod" },
  { name: "team-orders", environment: "prod" },
];

export const createFlowStories: Story[] = [
  {
    component: "WizardStepper",
    name: "Step without errors",
    render: () => (
      <WizardStepper steps={steps()} current="storage">
        <p className="text-sm text-text-2">
          The section cards of the step go here.
        </p>
      </WizardStepper>
    ),
  },
  {
    component: "WizardStepper",
    name: "Step with errors",
    render: () => (
      <WizardStepper
        steps={steps(["Choose a secondary storage option"])}
        current="storage"
      >
        <p className="text-sm text-text-2">
          The section cards of the step go here.
        </p>
      </WizardStepper>
    ),
  },
  {
    component: "WizardStepper",
    name: "Last step",
    render: () => (
      <WizardStepper steps={steps()} current="review">
        <p className="text-sm text-text-2">The review goes here.</p>
      </WizardStepper>
    ),
  },
  {
    component: "NamespacePicker",
    name: "Existing namespace",
    render: () => (
      <NamespacePicker
        namespaces={namespaces}
        environment={environment}
        defaultValue={{ kind: "existing", name: "team-orders" }}
      />
    ),
  },
  {
    component: "NamespacePicker",
    name: "New namespace",
    render: () => (
      <NamespacePicker
        namespaces={namespaces}
        environment={environment}
        defaultValue={{
          kind: "new",
          name: "team-search",
          labels: { team: "search" },
        }}
      />
    ),
  },
  {
    component: "NamespacePicker",
    name: "Creation not allowed",
    render: () => (
      <NamespacePicker
        namespaces={namespaces}
        environment={{ name: "prod", displayName: "Production" }}
      />
    ),
  },
];
