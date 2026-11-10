"use client";

import { useState } from "react";
import { useCreateFlow } from "@/components/create/use-create-flow";
import { WizardStepper } from "@/components/fleet/wizard-stepper";
import { planesOf } from "@/components/planes/plane";
import type { PlaneDraft } from "@/lib/planes/draft";
import type { ValidationError } from "@/lib/schema";
import type { WriteMode } from "@/lib/write/types";
import {
  PLANE_STEPS,
  type PlaneCreateData,
  type PlaneModels,
  PlaneStepBody,
  planeSteps,
} from "./plane-create-flow";

/**
 * PlaneCreateStory shows the plane create flow at step in the gallery. A
 * story passes no event handler, so this wrapper owns the draft, starts
 * from initial, and takes only data. It reads no live list, and its review
 * in mode previews clean and writes nothing.
 */
export function PlaneCreateStory({
  data,
  initial,
  step = "basics",
  mode = "direct",
}: {
  data: PlaneCreateData;
  initial: PlaneDraft;
  step?: (typeof PLANE_STEPS)[number]["id"];
  mode?: WriteMode;
}) {
  const models: PlaneModels = {
    environments: data.environments as PlaneModels["environments"],
    namespaces: data.namespaces as PlaneModels["namespaces"],
    clusters: data.clusters as PlaneModels["clusters"],
    platformConfigs: data.platformConfigs as PlaneModels["platformConfigs"],
    releases: data.releases as PlaneModels["releases"],
    sections: data.sections as PlaneModels["sections"],
    registrations: data.registrations as PlaneModels["registrations"],
    accessRequests: data.accessRequests as PlaneModels["accessRequests"],
    authConfigs: data.authConfigs.map((c) => c.metadata?.name ?? ""),
    objects: {
      DatabaseServerPreset:
        data.databaseServerPresets as PlaneModels["objects"]["DatabaseServerPreset"],
      DatabaseConfig:
        data.databaseConfigs as PlaneModels["objects"]["DatabaseConfig"],
    },
    planes: planesOf(data.planes),
    sources: data.planes,
    images: data.images,
  };
  const [selectorErrors, setSelectorErrors] = useState<ValidationError[]>([]);
  const [sectionErrors, setSectionErrors] = useState<
    Record<string, ValidationError[]>
  >({});
  const steps = planeSteps(
    models,
    selectorErrors,
    [],
    Object.values(sectionErrors).flat(),
  );
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
        onSectionErrors={(section, errors) =>
          setSectionErrors((current) => ({ ...current, [section]: errors }))
        }
        review={{
          mode,
          preview: async () => ({ objects: [], errors: [] }),
          create: async () => ({ ok: true, mode: "direct" }),
          onWritten: () => {},
        }}
      />
    </WizardStepper>
  );
}
