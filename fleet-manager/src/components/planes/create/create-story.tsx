"use client";

import { useState } from "react";
import { useCreateFlow } from "@/components/create/use-create-flow";
import { WizardStepper } from "@/components/fleet/wizard-stepper";
import { planesOf } from "@/components/planes/plane";
import type { PlaneDraft } from "@/lib/planes/draft";
import type { ValidationError } from "@/lib/schema";
import {
  type PlaneCreateData,
  type PlaneModels,
  PlaneStepBody,
  planeSteps,
} from "./plane-create-flow";

/**
 * PlaneCreateStory shows the plane create flow at step in the gallery. A
 * story passes no event handler, so this wrapper owns the draft, starts
 * from initial, and takes only data. It reads no live list.
 */
export function PlaneCreateStory({
  data,
  initial,
  step = "basics",
}: {
  data: PlaneCreateData;
  initial: PlaneDraft;
  step?: "basics" | "served";
}) {
  const models: PlaneModels = {
    environments: data.environments as PlaneModels["environments"],
    namespaces: data.namespaces as PlaneModels["namespaces"],
    clusters: data.clusters as PlaneModels["clusters"],
    platformConfigs: data.platformConfigs as PlaneModels["platformConfigs"],
    planes: planesOf(data.planes),
    sources: data.planes,
  };
  const [selectorErrors, setSelectorErrors] = useState<ValidationError[]>([]);
  const steps = planeSteps(models, selectorErrors);
  const flow = useCreateFlow({ steps, initial });
  const reached = steps.slice(0, steps.findIndex((s) => s.id === step) + 1);
  return (
    <WizardStepper
      steps={steps.map((s) => ({
        id: s.id,
        label: s.label,
        errors: reached.includes(s) ? s.check?.(flow.draft) : undefined,
      }))}
      current={step}
    >
      <PlaneStepBody
        current={step}
        draft={flow.draft}
        set={flow.set}
        models={models}
        secrets={[{ name: "keycloak-admin", keys: ["password", "username"] }]}
        onSelectorErrors={setSelectorErrors}
      />
    </WizardStepper>
  );
}
