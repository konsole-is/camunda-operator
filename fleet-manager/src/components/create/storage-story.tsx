"use client";

import { useState } from "react";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { WizardStepper } from "@/components/fleet/wizard-stepper";
import { CLUSTER_STEPS, type ClusterDraft } from "@/lib/clusters/draft";
import {
  type StorageData,
  storageErrors,
  storageViews,
} from "@/lib/clusters/storage";
import type { BuiltinObjects } from "@/lib/policy";
import type { ValidationError } from "@/lib/schema";
import { ClusterStorageStep } from "./cluster-storage-step";
import { useCreateFlow } from "./use-create-flow";

/**
 * StorageStory shows step 2 of the cluster create flow in the gallery. A
 * story passes no event handler, so this wrapper owns the draft and takes
 * only data. initial starts on step 2.
 */
export function StorageStory({
  sections,
  registrations,
  environment,
  objects = {},
  initial,
}: {
  sections: PluginSection[];
  registrations: PluginRegistration[];
  environment?: Environment;
  objects?: BuiltinObjects;
  initial: ClusterDraft;
}) {
  const data: StorageData = { environment, sections, registrations, objects };
  const [cardErrors, setCardErrors] = useState<
    Record<string, ValidationError[]>
  >({});
  const flow = useCreateFlow<ClusterDraft>({
    steps: CLUSTER_STEPS.filter((s) => s.id === "storage").map((s) => ({
      ...s,
      check: (draft: ClusterDraft) =>
        storageErrors(draft, storageViews(draft, data), cardErrors, []),
    })),
    initial,
  });
  return (
    <WizardStepper
      steps={CLUSTER_STEPS.map((s) => ({
        ...s,
        errors: s.id === "storage" ? flow.errors.storage : undefined,
      }))}
      current="storage"
    >
      <ClusterStorageStep
        draft={flow.draft}
        views={storageViews(flow.draft, data)}
        registrations={registrations}
        sections={sections}
        hosts={[]}
        conflicts={[]}
        onSection={(name, selections, errors) => {
          flow.set(["storage", name], selections);
          setCardErrors((current) => ({ ...current, [name]: errors }));
        }}
      />
    </WizardStepper>
  );
}
